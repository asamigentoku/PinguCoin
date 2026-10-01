package health

import (
	"context"
	"io"
	"log/slog"
	"net"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/test/bufconn"

	"github.com/asamigentoku/PinguCoin/server/payment-api/internal/interceptor"
	"github.com/asamigentoku/PinguCoin/server/payment-api/internal/testutil"
)

// startServer はサービス間認証つきのgRPCサーバー(本番と同じ構成)にヘルスチェックを登録し、
// メモリ上の接続(bufconn)で繋いだクライアントを返す。
func startServer(t *testing.T, ctx context.Context, fake *testutil.FakeDB) healthpb.HealthClient {
	t.Helper()
	previous := checkInterval
	checkInterval = 10 * time.Millisecond
	t.Cleanup(func() { checkInterval = previous })

	listener := bufconn.Listen(1 << 20)
	server := grpc.NewServer(grpc.ChainUnaryInterceptor(interceptor.Auth("shared-secret")))
	Register(ctx, server, fake.DB, slog.New(slog.NewTextHandler(io.Discard, nil)))
	go func() { _ = server.Serve(listener) }()
	t.Cleanup(server.Stop)

	conn, err := grpc.NewClient("passthrough:///bufnet",
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return listener.Dial() }),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	return healthpb.NewHealthClient(conn)
}

func statusOf(t *testing.T, client healthpb.HealthClient, service string) healthpb.HealthCheckResponse_ServingStatus {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	// トークンは付けない(kubeletと同じ)。認証の対象外であることも、ここで確かめている。
	response, err := client.Check(ctx, &healthpb.HealthCheckRequest{Service: service})
	if err != nil {
		t.Fatalf("Check(%q): %v", service, err)
	}
	return response.GetStatus()
}

func waitForStatus(t *testing.T, client healthpb.HealthClient, service string, want healthpb.HealthCheckResponse_ServingStatus) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if statusOf(t, client, service) == want {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("service %q did not become %v", service, want)
}

func TestServingWhenDatabaseIsReachable(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	client := startServer(t, ctx, testutil.NewFakeDB(t))

	if got := statusOf(t, client, ""); got != healthpb.HealthCheckResponse_SERVING {
		t.Errorf("readiness = %v, want SERVING", got)
	}
	if got := statusOf(t, client, LivenessService); got != healthpb.HealthCheckResponse_SERVING {
		t.Errorf("liveness = %v, want SERVING", got)
	}
}

// DBが落ちたときに落ちるのは readiness だけ。liveness まで落ちると、DBの障害でPodが再起動され続けてしまう。
func TestDatabaseOutageOnlyAffectsReadiness(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	fake := testutil.NewFakeDB(t)
	client := startServer(t, ctx, fake)

	fake.SetFailing(true)
	waitForStatus(t, client, "", healthpb.HealthCheckResponse_NOT_SERVING)
	if got := statusOf(t, client, LivenessService); got != healthpb.HealthCheckResponse_SERVING {
		t.Errorf("liveness = %v during a database outage, want SERVING", got)
	}

	fake.SetFailing(false)
	waitForStatus(t, client, "", healthpb.HealthCheckResponse_SERVING)
}

func TestShutdownMarksEverythingNotServing(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	client := startServer(t, ctx, testutil.NewFakeDB(t))

	cancel()

	waitForStatus(t, client, "", healthpb.HealthCheckResponse_NOT_SERVING)
	waitForStatus(t, client, LivenessService, healthpb.HealthCheckResponse_NOT_SERVING)
}

func TestUnknownServiceIsNotFound(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	client := startServer(t, ctx, testutil.NewFakeDB(t))

	callCtx, callCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer callCancel()
	if _, err := client.Check(callCtx, &healthpb.HealthCheckRequest{Service: "does.not.Exist"}); err == nil {
		t.Error("expected an error for an unknown service name")
	}
}

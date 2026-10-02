package metrics

import (
	"context"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/stats"
	"google.golang.org/grpc/status"
)

// healthMethodPrefix は、gRPC 標準のヘルスチェック。kubelet が数秒おきに呼ぶので、ログと同じく、数に入れない
// (入れると、本物のリクエストが、ヘルスチェックに埋もれる)。
const healthMethodPrefix = "/grpc.health.v1.Health/"

var (
	grpcServerHandled = factory().NewCounterVec(prometheus.CounterOpts{
		Name: "pingucoin_grpc_server_handled_total",
		Help: "gRPC サーバーが処理を終えたリクエストの数(結果のコード別)。",
	}, []string{"grpc_service", "grpc_method", "grpc_code"})

	grpcServerDuration = factory().NewHistogramVec(prometheus.HistogramOpts{
		Name:    "pingucoin_grpc_server_handling_seconds",
		Help:    "gRPC サーバーの処理時間(秒)。",
		Buckets: LatencyBuckets,
	}, []string{"grpc_service", "grpc_method"})

	grpcServerInFlight = factory().NewGauge(prometheus.GaugeOpts{
		Name: "pingucoin_grpc_server_in_flight_requests",
		Help: "gRPC サーバーが、いま処理中のリクエストの数。",
	})

	grpcClientHandled = factory().NewCounterVec(prometheus.CounterOpts{
		Name: "pingucoin_grpc_client_handled_total",
		Help: "gRPC クライアントの呼び出しが終わった数(再試行を含めた最終の結果のコード別)。",
	}, []string{"target", "grpc_service", "grpc_method", "grpc_code"})

	grpcClientDuration = factory().NewHistogramVec(prometheus.HistogramOpts{
		Name:    "pingucoin_grpc_client_handling_seconds",
		Help:    "gRPC クライアントの呼び出しにかかった時間(秒)。再試行の待ち時間を含む。",
		Buckets: LatencyBuckets,
	}, []string{"target", "grpc_service", "grpc_method"})

	grpcClientAttempts = factory().NewCounterVec(prometheus.CounterOpts{
		Name: "pingucoin_grpc_client_attempts_total",
		Help: "gRPC クライアントが、実際に送った回数(再試行を含む)。handled_total との差が、再試行の数。",
	}, []string{"target", "grpc_service", "grpc_method"})
)

// splitMethod は、"/orcan.v1.ProductService/GetProduct" を、サービス名とメソッド名に分ける。
// 想定外の形(スラッシュが無いなど)のときは、"unknown" にして、ラベルが増えるのを防ぐ。
func splitMethod(fullMethod string) (service, method string) {
	name := strings.TrimPrefix(fullMethod, "/")
	service, method, ok := strings.Cut(name, "/")
	if !ok || service == "" || method == "" {
		return "unknown", "unknown"
	}
	return service, method
}

// UnaryServerInterceptor は、gRPC サーバーに付けて、リクエストの数・結果のコード・処理時間・処理中の数を測る。
// logging / auth と同じく、全 RPC を、1か所で測る(各ハンドラーで、個別に書かなくてよい)。
//
// 認証に失敗したリクエスト(Unauthenticated)も、数に入る(インターセプターの並びで、Auth より外側に置くこと)。
func UnaryServerInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, request any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		if strings.HasPrefix(info.FullMethod, healthMethodPrefix) {
			return handler(ctx, request)
		}
		service, method := splitMethod(info.FullMethod)
		grpcServerInFlight.Inc()
		start := time.Now()

		response, err := handler(ctx, request)

		grpcServerInFlight.Dec()
		grpcServerDuration.WithLabelValues(service, method).Observe(time.Since(start).Seconds())
		grpcServerHandled.WithLabelValues(service, method, status.Code(err).String()).Inc()
		return response, err
	}
}

// UnaryClientInterceptor は、gRPC クライアントに付けて、呼び出しの数・結果のコード・かかった時間を測る。
// target は、呼び先の名前(例: "orcan-api")。アドレスそのものではなく、固定の名前にすること。
func UnaryClientInterceptor(target string) grpc.UnaryClientInterceptor {
	return func(ctx context.Context, fullMethod string, request, reply any, conn *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		service, method := splitMethod(fullMethod)
		start := time.Now()

		err := invoker(ctx, fullMethod, request, reply, conn, opts...)

		grpcClientDuration.WithLabelValues(target, service, method).Observe(time.Since(start).Seconds())
		grpcClientHandled.WithLabelValues(target, service, method, status.Code(err).String()).Inc()
		return err
	}
}

// ClientStatsHandler は、gRPC クライアントの「実際に送った回数」を数える(grpc.WithStatsHandler で付ける)。
// gRPC のリトライは、クライアントの中で、見えないように行われるので、インターセプターでは、再試行の回数が分からない。
// この回数と handled_total の差が、再試行の数になる。再試行が増えているのは、相手が不安定なサイン。
func ClientStatsHandler(target string) stats.Handler {
	return &clientStats{target: target}
}

type clientStats struct{ target string }

type methodKey struct{}

func (c *clientStats) TagRPC(ctx context.Context, info *stats.RPCTagInfo) context.Context {
	return context.WithValue(ctx, methodKey{}, info.FullMethodName)
}

func (c *clientStats) HandleRPC(ctx context.Context, event stats.RPCStats) {
	if _, ok := event.(*stats.Begin); !ok {
		return
	}
	fullMethod, _ := ctx.Value(methodKey{}).(string)
	service, method := splitMethod(fullMethod)
	grpcClientAttempts.WithLabelValues(c.target, service, method).Inc()
}

func (c *clientStats) TagConn(ctx context.Context, _ *stats.ConnTagInfo) context.Context { return ctx }

func (c *clientStats) HandleConn(context.Context, stats.ConnStats) {}

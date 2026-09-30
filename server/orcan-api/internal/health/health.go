// Package health は orcan-api のヘルスチェック(gRPC標準の grpc.health.v1)を提供する。
//
// Kubernetesのgrpc probeは、ここで登録したサービス名でステータスを問い合わせる。
//
//	""         ... readiness(トラフィックを受けてよいか)。DBに接続できる間だけ SERVING
//	"liveness" ... liveness(プロセスが生きているか)。プロセスが動いている限り常に SERVING
//
// livenessにDBの状態を含めないのは、DBが一時的に落ちただけでPodを再起動し続けて、
// 復旧を遅らせたり障害を広げたりしないため。DBが落ちたときは readiness だけが NOT_SERVING になり、
// Serviceの宛先から外れる(再起動はされない)。
package health

import (
	"context"
	"log/slog"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"gorm.io/gorm"
)

const (
	// LivenessService はlivenessプローブが指定するサービス名。
	LivenessService = "liveness"

	checkInterval = 5 * time.Second
	checkTimeout  = 2 * time.Second
)

// Register はgRPCサーバーにヘルスチェックのサービスを登録し、DBの疎通を定期的に確認して
// readiness(サービス名 "")の状態を更新する。ctxがキャンセルされると確認を止める。
func Register(ctx context.Context, server *grpc.Server, db *gorm.DB, logger *slog.Logger) {
	healthServer := health.NewServer()
	healthpb.RegisterHealthServer(server, healthServer)

	healthServer.SetServingStatus(LivenessService, healthpb.HealthCheckResponse_SERVING)
	// 起動時点でDBへの接続とマイグレーションは済んでいる。
	healthServer.SetServingStatus("", healthpb.HealthCheckResponse_SERVING)

	sqlDB, err := db.DB()
	if err != nil {
		logger.Error("health: failed to get sql.DB; readiness will stay NOT_SERVING", slog.Any("error", err))
		healthServer.SetServingStatus("", healthpb.HealthCheckResponse_NOT_SERVING)
		return
	}

	go func() {
		ticker := time.NewTicker(checkInterval)
		defer ticker.Stop()
		serving := true
		for {
			select {
			case <-ctx.Done():
				healthServer.Shutdown() // 終了時は全サービスを NOT_SERVING にする
				return
			case <-ticker.C:
			}

			pingCtx, cancel := context.WithTimeout(ctx, checkTimeout)
			err := sqlDB.PingContext(pingCtx)
			cancel()

			// 状態が変わったときだけログに残す(定期実行のたびには出さない)。
			if err != nil && serving {
				logger.Warn("health: database is unreachable; marking NOT_SERVING", slog.Any("error", err))
			} else if err == nil && !serving {
				logger.Info("health: database is reachable again; marking SERVING")
			}
			serving = err == nil
			if serving {
				healthServer.SetServingStatus("", healthpb.HealthCheckResponse_SERVING)
			} else {
				healthServer.SetServingStatus("", healthpb.HealthCheckResponse_NOT_SERVING)
			}
		}
	}()
}

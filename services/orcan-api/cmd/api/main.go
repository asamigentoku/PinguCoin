package main

import (
	"context"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"syscall"

	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
	"gorm.io/gorm"

	"github.com/asamigentoku/PinguCoin/pkg/cache"
	"github.com/asamigentoku/PinguCoin/pkg/dbretry"
	"github.com/asamigentoku/PinguCoin/pkg/health"
	"github.com/asamigentoku/PinguCoin/pkg/interceptor"
	"github.com/asamigentoku/PinguCoin/pkg/logging"
	"github.com/asamigentoku/PinguCoin/pkg/metrics"
	"github.com/asamigentoku/PinguCoin/pkg/version"
	"github.com/asamigentoku/PinguCoin/services/orcan-api/internal/config"
	"github.com/asamigentoku/PinguCoin/services/orcan-api/internal/database"
	"github.com/asamigentoku/PinguCoin/services/orcan-api/internal/grpcserver"
	"github.com/asamigentoku/PinguCoin/services/orcan-api/internal/repository"
	"github.com/asamigentoku/PinguCoin/services/orcan-api/internal/storage"
	pb "github.com/asamigentoku/PinguCoin/services/orcan-api/proto/orcan/v1"
)

func main() {
	// JSON形式の構造化ログ。ログ収集基盤に食わせやすいようにstdoutへ出す。
	// Datadog の標準属性にそろえた JSON のログ(service / env / version が、すべてのログに付く)。
	// 環境は APP_ENV、ログのレベルは LOG_LEVEL(debug / info / warn / error)で決める。
	logger := logging.NewFromEnv("orcan-api")
	slog.SetDefault(logger)
	build := version.Get()
	metrics.Init("orcan-api")
	logger.Info("orcan-api starting",
		slog.Group("build", slog.String("commit", build.Commit), slog.String("time", build.BuildTime), slog.String("go", build.GoVersion)),
	)

	cfg := config.Load()
	if cfg.InternalAPIToken == "" {
		logger.Error("INTERNAL_API_TOKEN is required")
		os.Exit(1)
	}

	// DB が起動中・再起動中などで、一時的につながらないときは、バックオフつきで再試行する。
	// 設定の間違い(接続文字列・パスワード・DB名)は、再試行せず、すぐに終了する。
	db, err := dbretry.Connect(context.Background(), logger, func() (*gorm.DB, error) { return database.Connect(cfg, logger) })
	if err != nil {
		logger.Error("failed to connect database", logging.Err(err))
		os.Exit(1)
	}

	// 起動時に、まだ適用していない DB のマイグレーション(services/orcan-api/migrations の SQL)を、番号順に適用する。
	// 複数の Pod が同時に起動しても、DB のロックで1つずつ実行する(pkg/dbmigrate)。
	if err := database.Migrate(db); err != nil {
		logger.Error("failed to migrate database", logging.Err(err))
		os.Exit(1)
	}

	blobStorage, err := storage.New(cfg.AzureStorageConnectionString, cfg.AzureStoragePublicContainer, cfg.AzureStoragePrivateContainer, cfg.AppEnv)
	if err != nil {
		logger.Error("failed to init blob storage client", logging.Err(err))
		os.Exit(1)
	}

	// gRPCはHTTPサーバーと同じくTCPソケットで待ち受ける。
	listener, err := net.Listen("tcp", ":"+cfg.Port)
	if err != nil {
		logger.Error("failed to listen", logging.Err(err))
		os.Exit(1)
	}

	// interceptor.Logging を挟むことで、以降登録する全RPCの
	// 呼び出しログ(メソッド名・処理時間・結果コード)が自動的に出るようになる。
	// 各ハンドラー(internal/grpcserver)側で個別にログを書く必要はない。
	// interceptor.Auth はpingu-api以外からの直接のgRPC呼び出しを拒否する(サービス間認証)。
	server := grpc.NewServer(
		grpc.ChainUnaryInterceptor(
			metrics.UnaryServerInterceptor(), // リクエストの数・結果・処理時間(認証に失敗したものも数える。いちばん外側)
			interceptor.RequestID(),          // 呼び出し元(pingu-api)のリクエスト ID を受け取る(Logging より前)
			interceptor.Logging(logger),
			interceptor.Auth(cfg.InternalAPIToken),
		),
	)

	// proto定義した各サービス(ProductService等)の実装を登録する。
	// RegisterXxxServiceServer は internal/pb (生成コード) 側の関数で、
	// 「このgRPCサーバーに、このRPCが来たらこの実装(grpcserver.NewXxxServer)を呼ぶ」
	// というルーティングをserverの内部に登録する処理。
	// 各実装(grpcserver.NewXxxServer)はDBアクセス用のrepositoryを注入されて動く。
	// 商品の読み取りキャッシュ。REDIS_ENABLED=true のときだけ Redis を使う(false なら nil で、常に DB から読む)。
	var productCache *cache.Cache
	if cfg.RedisEnabled {
		productCache = cache.New(cache.Config{Addr: cfg.RedisAddr, Password: cfg.RedisPassword, DB: cfg.RedisDB, TTL: cfg.ProductCacheTTL}, logger)
		defer productCache.Close()
		logger.Info("product cache enabled (redis)", slog.String("addr", cfg.RedisAddr), slog.Duration("ttl", cfg.ProductCacheTTL))
	} else {
		logger.Info("product cache disabled (REDIS_ENABLED=false)")
	}
	productRepo := repository.NewProductRepository(db).WithCache(productCache)
	detailRepo := repository.NewProductDetailRepository(db)
	pb.RegisterProductServiceServer(server, grpcserver.NewProductServer(productRepo, detailRepo, blobStorage))
	pb.RegisterProductAssetServiceServer(server, grpcserver.NewProductAssetServer(repository.NewProductAssetRepository(db), productRepo, blobStorage))
	pb.RegisterProductCategoryServiceServer(server, grpcserver.NewProductCategoryServer(repository.NewProductCategoryRepository(db)))
	pb.RegisterProductDetailServiceServer(server, grpcserver.NewProductDetailServer(detailRepo))
	pb.RegisterProductInventoryServiceServer(server, grpcserver.NewProductInventoryServer(repository.NewProductInventoryRepository(db)))
	pb.RegisterProductListingServiceServer(server, grpcserver.NewProductListingServer(repository.NewProductListingRepository(db)))

	pb.RegisterUserServiceServer(server, grpcserver.NewUserServer(repository.NewUserRepository(db)))

	// reflectionを有効にすると、.protoファイルを配らなくても
	// grpcurl等のツールがサーバーに直接問い合わせてスキーマ(サービス一覧・メッセージ構造)を取得できる。
	reflection.Register(server)

	// ヘルスチェック(Kubernetesのprobe用)。SIGTERMを受けたら readiness を落として止める。
	shutdownCtx, stopSignals := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stopSignals()
	// メトリクス(/metrics)。別のポートで待ち受け、クラスター内の Prometheus だけが取る(pkg/metrics)。
	go metrics.Serve(shutdownCtx, metrics.Addr(), logger)
	health.Register(shutdownCtx, server, db, logger)
	go func() {
		<-shutdownCtx.Done()
		logger.Info("shutting down: draining in-flight requests")
		// 処理中のRPCが終わるのを待ってから止める(新しいRPCは受け付けない)。
		server.GracefulStop()
	}()

	logger.Info("orcan-api (gRPC) listening", slog.String("port", cfg.Port))
	// listenしているTCPソケット(listener)に対してリクエストの受付・処理ループを開始する。
	// ここでブロックし、プロセスが終了するまで返ってこない。
	if err := server.Serve(listener); err != nil {
		logger.Error("failed to serve", logging.Err(err))
		os.Exit(1)
	}
}

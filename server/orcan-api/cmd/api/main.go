package main

import (
	"log/slog"
	"net"
	"os"

	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	"github.com/asamigentoku/PinguCoin/server/orcan-api/internal/config"
	"github.com/asamigentoku/PinguCoin/server/orcan-api/internal/database"
	"github.com/asamigentoku/PinguCoin/server/orcan-api/internal/grpcserver"
	"github.com/asamigentoku/PinguCoin/server/orcan-api/internal/interceptor"
	pb "github.com/asamigentoku/PinguCoin/server/orcan-api/internal/pb/orcan/v1"
	"github.com/asamigentoku/PinguCoin/server/orcan-api/internal/repository"
)

func main() {
	// JSON形式の構造化ログ。ログ収集基盤に食わせやすいようにstdoutへ出す。
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	cfg := config.Load()

	db, err := database.Connect(cfg, logger)
	if err != nil {
		logger.Error("failed to connect database", slog.Any("error", err))
		os.Exit(1)
	}

	// 起動時にモデル(internal/model)の定義に合わせてテーブルを作成・更新する。
	if err := database.AutoMigrate(db); err != nil {
		logger.Error("failed to migrate database", slog.Any("error", err))
		os.Exit(1)
	}

	// gRPCはHTTPサーバーと同じくTCPソケットで待ち受ける。
	listener, err := net.Listen("tcp", ":"+cfg.Port)
	if err != nil {
		logger.Error("failed to listen", slog.Any("error", err))
		os.Exit(1)
	}

	// interceptor.Logging を挟むことで、以降登録する全RPCの
	// 呼び出しログ(メソッド名・処理時間・結果コード)が自動的に出るようになる。
	// 各ハンドラー(internal/grpcserver)側で個別にログを書く必要はない。
	server := grpc.NewServer(
		grpc.ChainUnaryInterceptor(interceptor.Logging(logger)),
	)

	// proto定義した各サービス(ProductService等)の実装を登録する。
	// RegisterXxxServiceServer は internal/pb (生成コード) 側の関数で、
	// 「このgRPCサーバーに、このRPCが来たらこの実装(grpcserver.NewXxxServer)を呼ぶ」
	// というルーティングをserverの内部に登録する処理。
	// 各実装(grpcserver.NewXxxServer)はDBアクセス用のrepositoryを注入されて動く。
	pb.RegisterProductServiceServer(server, grpcserver.NewProductServer(repository.NewProductRepository(db)))
	pb.RegisterProductCategoryServiceServer(server, grpcserver.NewProductCategoryServer(repository.NewProductCategoryRepository(db)))
	pb.RegisterProductDetailServiceServer(server, grpcserver.NewProductDetailServer(repository.NewProductDetailRepository(db)))
	pb.RegisterProductInventoryServiceServer(server, grpcserver.NewProductInventoryServer(repository.NewProductInventoryRepository(db)))
	pb.RegisterProductListingServiceServer(server, grpcserver.NewProductListingServer(repository.NewProductListingRepository(db)))

	pb.RegisterUserServiceServer(server, grpcserver.NewUserServer(repository.NewUserRepository(db)))

	// reflectionを有効にすると、.protoファイルを配らなくても
	// grpcurl等のツールがサーバーに直接問い合わせてスキーマ(サービス一覧・メッセージ構造)を取得できる。
	reflection.Register(server)

	logger.Info("orcan-api (gRPC) listening", slog.String("port", cfg.Port))
	// listenしているTCPソケット(listener)に対してリクエストの受付・処理ループを開始する。
	// ここでブロックし、プロセスが終了するまで返ってこない。
	if err := server.Serve(listener); err != nil {
		logger.Error("failed to serve", slog.Any("error", err))
		os.Exit(1)
	}
}

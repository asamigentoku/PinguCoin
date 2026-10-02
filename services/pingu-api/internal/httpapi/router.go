package httpapi

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/99designs/gqlgen/graphql"
	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/handler/extension"
	"github.com/99designs/gqlgen/graphql/handler/lru"
	"github.com/99designs/gqlgen/graphql/handler/transport"
	"github.com/99designs/gqlgen/graphql/playground"
	"github.com/vektah/gqlparser/v2/ast"
	"github.com/vektah/gqlparser/v2/gqlerror"

	"github.com/asamigentoku/PinguCoin/pkg/logging"
	"github.com/asamigentoku/PinguCoin/pkg/metrics"
	"github.com/asamigentoku/PinguCoin/services/pingu-api/graph"
	"github.com/asamigentoku/PinguCoin/services/pingu-api/graph/resolvers"
	"github.com/asamigentoku/PinguCoin/services/pingu-api/internal/apperr"
	"github.com/asamigentoku/PinguCoin/services/pingu-api/internal/orcanclient"
	"github.com/asamigentoku/PinguCoin/services/pingu-api/internal/paymentclient"
	"github.com/asamigentoku/PinguCoin/services/pingu-api/internal/repository"
)

// APIVersionPrefix は公開APIエンドポイントに共通で付与するバージョンプレフィックス。
// GraphQL Playground(開発用UI)のみ、API自体ではないためプレフィックス外の "/" に置く。
const APIVersionPrefix = "/api/v1"

// NewRouter は以下のエンドポイントを1つのhttp.Handlerにまとめる。
// ログイン/ログアウトはClerk(フロントエンド)側で完結するため、pingu-apiにRESTのauthエンドポイントは無い。
// 各リクエストはAuthorizationヘッダのClerkセッショントークンをWithOptionalAuthが検証する。
//   - GET  /                    : GraphQL Playground(開発用)
//   - POST /api/v1/graphql      : GraphQL(商品・ユーザーのCRUD)
//   - POST /api/v1/orders       : 商品購入(注文API)
//   - GET  /api/v1/orders       : 自分の注文一覧
//   - GET  /api/v1/orders/{id}  : 注文詳細
//   - GET  /api/v1/points       : 自分のポイント残高と履歴
//   - GET  /healthz, /readyz    : Kubernetesのprobe用(liveness / readiness)
//
// メトリクス(/metrics)は、このルーターには無い。別のポートで、別のサーバーが出す(pkg/metrics)。
func NewRouter(logger *slog.Logger, orcan *orcanclient.Client, payment *paymentclient.Client, orderRepo *repository.OrderRepository, ping func(context.Context) error) http.Handler {
	//muxはapp_router
	mux := http.NewServeMux()

	graphqlServer := handler.New(graph.NewExecutableSchema(graph.Config{Resolvers: &resolvers.Resolver{Orcan: orcan, Orders: orderRepo}}))
	graphqlServer.AddTransport(transport.Options{})
	graphqlServer.AddTransport(transport.GET{})
	graphqlServer.AddTransport(transport.POST{})
	graphqlServer.SetQueryCache(lru.New[*ast.QueryDocument](1000))
	graphqlServer.Use(extension.Introspection{})
	graphqlServer.Use(extension.AutomaticPersistedQuery{Cache: lru.New[string](100)})
	graphqlServer.SetErrorPresenter(newErrorPresenter(logger))
	graphqlServer.AroundResponses(operationMetrics) // 操作の数・結果(エラー)・処理時間

	mux.Handle("/", playground.Handler("PinguCoin GraphQL playground", APIVersionPrefix+"/graphql"))
	mux.Handle(APIVersionPrefix+"/graphql", graphqlServer)

	healthHandler := NewHealthHandler(ping)
	mux.HandleFunc("GET /version", healthHandler.Version)
	mux.HandleFunc("GET /healthz", healthHandler.Live)
	mux.HandleFunc("GET /readyz", healthHandler.Ready)

	orderHandler := NewOrderHandler(logger, orcan, payment, orderRepo)
	mux.HandleFunc("POST "+APIVersionPrefix+"/orders", orderHandler.CreateOrder)
	mux.HandleFunc("GET "+APIVersionPrefix+"/orders", orderHandler.ListOrders)
	mux.HandleFunc("GET "+APIVersionPrefix+"/orders/{id}", orderHandler.GetOrder)
	mux.HandleFunc("GET "+APIVersionPrefix+"/points", NewPointHandler(payment).GetPoints)

	// 内側から: ルートの記録(メトリクス)→ 認証 → ログ → API のバージョンのヘッダー → リクエスト ID → メトリクス(いちばん外側)。
	// リクエスト ID は、ログにも、レスポンスにも、同じ ID を出すため、外側。メトリクスは、認証などを含めた処理時間を測るため、さらに外側。
	// metrics.Route は、ServeMux を直接包む(合ったルートのパターンを、外側の metrics.HTTP に伝える)。
	return metrics.HTTP(WithRequestID(WithAPIVersion(WithLogging(logger)(WithOptionalAuth(orcan)(metrics.Route(mux))))))
}

// newErrorPresenter はresolverが返したエラーをGraphQLのエラーレスポンスに変換する。
// *apperr.AppError以外(想定外のエラー)やInternal相当のエラーは、元の詳細(DBエラー等)を
// クライアントに一切返さず、安全なメッセージ("internal server error")のみを返す。
// 元の詳細はサーバーログにのみ残す(gRPC側のinternal/apperrと同じ方針)。
func newErrorPresenter(logger *slog.Logger) graphql.ErrorPresenterFunc {
	return func(ctx context.Context, err error) *gqlerror.Error {
		appErr, ok := apperr.As(err)
		if !ok {
			appErr = apperr.Internal(err)
		}

		if appErr.Reason == apperr.ReasonInternal && appErr.Err != nil {
			logger.ErrorContext(ctx, "graphql internal error",
				logging.RequestID(ctx),
				slog.Any("graphql_path", graphql.GetPath(ctx)),
				logging.Err(appErr.Err),
			)
		}

		gqlErr := graphql.DefaultErrorPresenter(ctx, appErr)
		gqlErr.Message = appErr.Message
		if gqlErr.Extensions == nil {
			gqlErr.Extensions = map[string]interface{}{}
		}
		gqlErr.Extensions["reason"] = string(appErr.Reason)
		return gqlErr
	}
}

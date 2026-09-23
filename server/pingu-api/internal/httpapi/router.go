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

	"github.com/asamigentoku/PinguCoin/server/pingu-api/graph"
	"github.com/asamigentoku/PinguCoin/server/pingu-api/internal/apperr"
	"github.com/asamigentoku/PinguCoin/server/pingu-api/internal/orcanclient"
	"github.com/asamigentoku/PinguCoin/server/pingu-api/internal/paymentclient"
	"github.com/asamigentoku/PinguCoin/server/pingu-api/internal/repository"
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
func NewRouter(logger *slog.Logger, orcan *orcanclient.Client, payment *paymentclient.Client, orderRepo *repository.OrderRepository) http.Handler {
	//muxはapp_router
	mux := http.NewServeMux()

	graphqlServer := handler.New(graph.NewExecutableSchema(graph.Config{Resolvers: &graph.Resolver{Orcan: orcan, Orders: orderRepo}}))
	graphqlServer.AddTransport(transport.Options{})
	graphqlServer.AddTransport(transport.GET{})
	graphqlServer.AddTransport(transport.POST{})
	graphqlServer.SetQueryCache(lru.New[*ast.QueryDocument](1000))
	graphqlServer.Use(extension.Introspection{})
	graphqlServer.Use(extension.AutomaticPersistedQuery{Cache: lru.New[string](100)})
	graphqlServer.SetErrorPresenter(newErrorPresenter(logger))

	mux.Handle("/", playground.Handler("PinguCoin GraphQL playground", APIVersionPrefix+"/graphql"))
	mux.Handle(APIVersionPrefix+"/graphql", graphqlServer)

	orderHandler := NewOrderHandler(orcan, payment, orderRepo)
	mux.HandleFunc("POST "+APIVersionPrefix+"/orders", orderHandler.CreateOrder)
	mux.HandleFunc("GET "+APIVersionPrefix+"/orders", orderHandler.ListOrders)
	mux.HandleFunc("GET "+APIVersionPrefix+"/orders/{id}", orderHandler.GetOrder)

	return WithLogging(logger)(WithOptionalAuth(orcan)(mux))
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
			logger.Error("graphql internal error",
				slog.Any("path", graphql.GetPath(ctx)),
				slog.String("error", appErr.Err.Error()),
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

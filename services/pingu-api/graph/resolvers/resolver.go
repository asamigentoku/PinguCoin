// Package resolvers は GraphQL のリゾルバー(クエリ・ミューテーションの処理)をまとめる。
// このファイルは、その入口と依存先をつなぐ配線だけを持つ。実際の処理は product.go / asset.go / user.go にある。
package resolvers

import (
	"github.com/asamigentoku/PinguCoin/services/pingu-api/graph"
	"github.com/asamigentoku/PinguCoin/services/pingu-api/internal/orcanclient"
	"github.com/asamigentoku/PinguCoin/services/pingu-api/internal/repository"
)

// Resolver は全リゾルバーが共有する依存先をまとめた入れ物。
//   - Orcan  : orcan-api へのgRPCクライアント(商品・ユーザー・アセット・在庫)
//   - Orders : 注文のリポジトリ(購入済みかどうかの確認に使う)
type Resolver struct {
	Orcan  *orcanclient.Client
	Orders *repository.OrderRepository
}

// Resolver が gqlgen の求める ResolverRoot を満たしているか、コンパイル時に確認する(実行時には何もしない)。
var _ graph.ResolverRoot = (*Resolver)(nil)

// Mutation / Query は gqlgen が最初に呼ぶ入口。書き込み系と読み取り系で、担当の型を分けて返す。
func (r *Resolver) Mutation() graph.MutationResolver { return &mutationResolver{Resolver: r} }
func (r *Resolver) Query() graph.QueryResolver       { return &queryResolver{Resolver: r} }

// mutationResolver / queryResolver は、各ファイルがメソッドを足す型。
// *Resolver を埋め込んでいるので、r.Orcan や r.Orders をそのまま使える。
type mutationResolver struct{ *Resolver }
type queryResolver struct{ *Resolver }

package graph

// This file will not be regenerated automatically.
//
// It serves as dependency injection for your app, add any dependencies you require
// here.

import "github.com/asamigentoku/PinguCoin/server/pingu-api/internal/orcanclient"

// Resolver は各resolverが使う依存(orcan-apiクライアント)を持つ。
// 商品・ユーザーはどちらもorcan-apiが真実の記録を持つため、GraphQL層はorcanclient経由でのみアクセスする。
type Resolver struct {
	Orcan *orcanclient.Client
}

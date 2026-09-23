package graph

// This file will not be regenerated automatically.
//
// It serves as dependency injection for your app, add any dependencies you require
// here.

import (
	"github.com/asamigentoku/PinguCoin/server/pingu-api/internal/orcanclient"
	"github.com/asamigentoku/PinguCoin/server/pingu-api/internal/repository"
)

// Resolver は各resolverが使う依存(orcan-apiクライアント等)を持つ。
// 商品・ユーザーはどちらもorcan-apiが真実の記録を持つため、GraphQL層はorcanclient経由でのみアクセスする。
// Orders は商品ファイルのダウンロード許可判定(購入済みか)にのみ使う。
type Resolver struct {
	Orcan  *orcanclient.Client
	Orders *repository.OrderRepository
}

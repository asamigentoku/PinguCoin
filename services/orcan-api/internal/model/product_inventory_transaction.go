package model

import "time"

// ProductInventoryTransaction は在庫数(quantity)の増減履歴。1件が1回の増減に対応する
// (point_transactionsと同じ考え方)。IdempotencyKeyで同一操作の重複適用を防ぐ。
type ProductInventoryTransaction struct {
	ID        uint   `gorm:"primaryKey" json:"id"`
	ProductID uint   `gorm:"not null;index" json:"product_id"`
	Amount    int    `gorm:"not null" json:"amount"` // 負=消費(注文等), 正=戻し(注文失敗時の補償等)
	Reason    string `gorm:"type:text" json:"reason"`
	// IdempotencyKeyは呼び出し元(pingu-api)が発行する冪等性キー。同じキーでの再呼び出しは
	// 新たに在庫を増減させず、最初に作られたこのレコードをそのまま返す。
	IdempotencyKey string    `gorm:"size:255;not null;uniqueIndex" json:"idempotency_key"`
	QuantityAfter  int       `gorm:"not null" json:"quantity_after"`
	CreatedAt      time.Time `json:"created_at"`
}

func (ProductInventoryTransaction) TableName() string {
	return "product_inventory_transactions"
}

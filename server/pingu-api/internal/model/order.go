package model

import "time"

// Order は購入(注文)を表す。決済の実処理・履歴はpayment-apiのPaymentが真実の記録として持ち、
// Orderはpingu-api側で「誰が・何を・いくつ買ったか」という注文としての記録を、
// 決済(Payment)とは別エンティティとして保持する。PaymentIDでpayment-apiのPaymentを参照する。
type Order struct {
	ID          uint  `gorm:"primaryKey" json:"id"`
	UserID      uint  `gorm:"not null;index" json:"user_id"`
	ProductID   uint  `gorm:"not null;index" json:"product_id"`
	Quantity    int64 `gorm:"not null" json:"quantity"`
	UnitPrice   int64 `gorm:"not null" json:"unit_price"`
	TotalAmount int64 `gorm:"not null" json:"total_amount"`
	// PaymentID はpayment-apiの`payments.id`への参照。サービスをまたぐため外部キーは張らずID参照のみ。
	PaymentID uint `gorm:"not null" json:"payment_id"`
	// "pending" / "paid" / "failed" / "canceled"
	Status string `gorm:"size:20;not null;default:'pending'" json:"status"`
	// IdempotencyKeyはクライアント(フロントエンド)が発行する冪等性キー(Idempotency-Keyヘッダ)。
	// 同じキーでの再送は新たに注文を作らず、最初に作られたこのOrderをそのまま返す
	// (二重注文防止)。同じキーはorcan-apiの在庫減算・payment-apiの決済作成にもそのまま渡し、
	// それぞれの側でも二重処理を防ぐ。
	IdempotencyKey string    `gorm:"size:255;not null;uniqueIndex" json:"idempotency_key"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

func (Order) TableName() string {
	return "orders"
}

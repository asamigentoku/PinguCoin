package orderflow

const (
	TopicOrderCreated          = "orders.created.v1"
	TopicInventoryReserved     = "inventory.reserved.v1"
	TopicInventoryRejected     = "inventory.rejected.v1"
	TopicPaymentSucceeded      = "payments.succeeded.v1"
	TopicPaymentFailed         = "payments.failed.v1"
	TopicPaymentActionRequired = "payments.action-required.v1"
)

type OrderCreated struct {
	OrderID        uint   `json:"order_id"`
	UserID         uint   `json:"user_id"`
	ProductID      uint   `json:"product_id"`
	Quantity       int64  `json:"quantity"`
	UnitPrice      int64  `json:"unit_price"`
	TotalAmount    int64  `json:"total_amount"`
	PaymentMethod  string `json:"payment_method"`
	Currency       string `json:"currency"`
	IdempotencyKey string `json:"idempotency_key"`
}

type InventoryResult struct {
	OrderCreated
	Reason string `json:"reason,omitempty"`
}

type PaymentResult struct {
	OrderCreated
	PaymentID uint   `json:"payment_id,omitempty"`
	Reason    string `json:"reason,omitempty"`
	ActionURL string `json:"action_url,omitempty"`
}

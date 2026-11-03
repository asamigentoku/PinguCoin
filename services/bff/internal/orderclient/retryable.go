package orderclient

import pb "github.com/asamigentoku/PinguCoin/services/order-api/proto/order/v1"

// RetryableMethods contains only operations that are safe to repeat. CreateOrder is protected by
// the required idempotency key, so a retry returns the existing order rather than creating another.
var RetryableMethods = []string{
	pb.OrderService_CreateOrder_FullMethodName,
	pb.OrderService_GetOrder_FullMethodName,
	pb.OrderService_ListOrders_FullMethodName,
	pb.OrderService_HasPaidOrder_FullMethodName,
}

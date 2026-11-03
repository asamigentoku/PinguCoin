package eventconsumer

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/twmb/franz-go/pkg/kgo"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/asamigentoku/PinguCoin/pkg/consumerretry"
	"github.com/asamigentoku/PinguCoin/pkg/orderflow"
	pb "github.com/asamigentoku/PinguCoin/services/payment-api/proto/payment/v1"
)

type paymentCreator interface {
	CreatePayment(context.Context, *pb.CreatePaymentRequest) (*pb.CreatePaymentResponse, error)
}
type Consumer struct {
	client   *kgo.Client
	payments paymentCreator
	logger   *slog.Logger
}

func New(brokers []string, payments paymentCreator, logger *slog.Logger) (*Consumer, error) {
	if len(brokers) == 0 {
		return &Consumer{payments: payments, logger: logger}, nil
	}
	client, err := kgo.NewClient(kgo.SeedBrokers(brokers...), kgo.ConsumerGroup("payment-api-v1"), kgo.ConsumeTopics(orderflow.TopicInventoryReserved), kgo.DisableAutoCommit(), kgo.RequiredAcks(kgo.AllISRAcks()))
	if err != nil {
		return nil, err
	}
	return &Consumer{client: client, payments: payments, logger: logger}, nil
}
func (c *Consumer) Close() {
	if c.client != nil {
		c.client.Close()
	}
}
func (c *Consumer) Run(ctx context.Context) error {
	if c.client == nil {
		return nil
	}
	for ctx.Err() == nil {
		fetches := c.client.PollFetches(ctx)
		for _, fetchErr := range fetches.Errors() {
			c.logger.Error("payment event fetch failed", "topic", fetchErr.Topic, "partition", fetchErr.Partition, "error", fetchErr.Err)
		}
		for _, record := range fetches.Records() {
			if err := consumerretry.Do(ctx, c.logger, "payment.handle", func(attemptCtx context.Context) error {
				return c.handle(attemptCtx, record)
			}); err != nil {
				return fmt.Errorf("payment event handling exhausted: %w", err)
			}
			if err := consumerretry.Do(ctx, c.logger, "payment.commit", func(attemptCtx context.Context) error {
				return c.client.CommitRecords(attemptCtx, record)
			}); err != nil {
				return fmt.Errorf("payment offset commit exhausted: %w", err)
			}
		}
	}
	return nil
}
func (c *Consumer) handle(ctx context.Context, record *kgo.Record) error {
	var inventory orderflow.InventoryResult
	if err := json.Unmarshal(record.Value, &inventory); err != nil {
		return err
	}
	event := orderflow.PaymentResult{OrderCreated: inventory.OrderCreated}
	response, paymentErr := c.payments.CreatePayment(ctx, &pb.CreatePaymentRequest{UserId: uint32(event.UserID), ProductId: uint32(event.ProductID), Amount: event.TotalAmount, Currency: "POINT", PaymentMethod: event.PaymentMethod, IdempotencyKey: event.IdempotencyKey})
	topic := orderflow.TopicPaymentSucceeded
	if paymentErr != nil {
		if !isBusinessFailure(paymentErr) {
			return paymentErr
		}
		topic = orderflow.TopicPaymentFailed
		event.Reason = status.Convert(paymentErr).Message()
	} else {
		event.PaymentID = uint(response.GetPayment().GetId())
	}
	payload, err := json.Marshal(event)
	if err != nil {
		return err
	}
	return c.client.ProduceSync(ctx, &kgo.Record{Topic: topic, Key: []byte(event.IdempotencyKey), Value: payload}).FirstErr()
}

func isBusinessFailure(err error) bool {
	switch status.Code(err) {
	case codes.InvalidArgument, codes.FailedPrecondition, codes.NotFound, codes.PermissionDenied, codes.Unauthenticated:
		return true
	default:
		return false
	}
}

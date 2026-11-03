package eventconsumer

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/twmb/franz-go/pkg/kgo"

	"github.com/asamigentoku/PinguCoin/pkg/consumerretry"
	"github.com/asamigentoku/PinguCoin/pkg/orderflow"
	"github.com/asamigentoku/PinguCoin/services/payment-api/internal/application"
)

type paymentCreator interface {
	Create(context.Context, application.CreatePaymentCommand) (*application.CreatePaymentResult, error)
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
	client, err := kgo.NewClient(kgo.SeedBrokers(brokers...), kgo.ConsumerGroup("payment-api-v1"), kgo.ConsumeTopics(orderflow.TopicInventoryReserved), kgo.DisableAutoCommit())
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
	order := inventory.OrderCreated
	_, err := c.payments.Create(ctx, application.CreatePaymentCommand{UserID: order.UserID, ProductID: order.ProductID, Amount: order.TotalAmount, Currency: order.Currency, PaymentMethod: order.PaymentMethod, IdempotencyKey: order.IdempotencyKey, Order: &order})
	return err
}

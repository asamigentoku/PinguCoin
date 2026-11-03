package eventconsumer

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"

	"github.com/twmb/franz-go/pkg/kgo"

	"github.com/asamigentoku/PinguCoin/pkg/consumerretry"
	"github.com/asamigentoku/PinguCoin/pkg/orderflow"
	"github.com/asamigentoku/PinguCoin/services/inventory-api/internal/repository"
)

type Consumer struct {
	client *kgo.Client
	repo   *repository.InventoryRepository
	logger *slog.Logger
}

func New(brokers []string, repo *repository.InventoryRepository, logger *slog.Logger) (*Consumer, error) {
	if len(brokers) == 0 {
		return &Consumer{repo: repo, logger: logger}, nil
	}
	client, err := kgo.NewClient(kgo.SeedBrokers(brokers...), kgo.ConsumerGroup("inventory-api-v1"), kgo.ConsumeTopics(orderflow.TopicOrderCreated, orderflow.TopicPaymentFailed), kgo.DisableAutoCommit(), kgo.RequiredAcks(kgo.AllISRAcks()))
	if err != nil {
		return nil, err
	}
	return &Consumer{client: client, repo: repo, logger: logger}, nil
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
			c.logger.Error("inventory event fetch failed", "topic", fetchErr.Topic, "partition", fetchErr.Partition, "error", fetchErr.Err)
		}
		for _, record := range fetches.Records() {
			if err := consumerretry.Do(ctx, c.logger, "inventory.handle", func(attemptCtx context.Context) error {
				return c.handle(attemptCtx, record)
			}); err != nil {
				return fmt.Errorf("inventory event handling exhausted: %w", err)
			}
			if err := consumerretry.Do(ctx, c.logger, "inventory.commit", func(attemptCtx context.Context) error {
				return c.client.CommitRecords(attemptCtx, record)
			}); err != nil {
				return fmt.Errorf("inventory offset commit exhausted: %w", err)
			}
		}
	}
	return nil
}
func (c *Consumer) handle(ctx context.Context, record *kgo.Record) error {
	if record.Topic == orderflow.TopicPaymentFailed {
		var event orderflow.PaymentResult
		if err := json.Unmarshal(record.Value, &event); err != nil {
			return err
		}
		_, _, err := c.repo.AdjustContext(ctx, event.ProductID, int(event.Quantity), "payment compensation", event.IdempotencyKey+":inventory-release")
		return err
	}
	var event orderflow.OrderCreated
	if err := json.Unmarshal(record.Value, &event); err != nil {
		return err
	}
	_, _, adjustErr := c.repo.AdjustContext(ctx, event.ProductID, -int(event.Quantity), "order stock decrement", event.IdempotencyKey+":inventory")
	result := orderflow.InventoryResult{OrderCreated: event}
	topic := orderflow.TopicInventoryReserved
	if adjustErr != nil {
		if !errors.Is(adjustErr, repository.ErrInsufficientStock) {
			return adjustErr
		}
		topic = orderflow.TopicInventoryRejected
		result.Reason = "insufficient stock"
	}
	payload, err := json.Marshal(result)
	if err != nil {
		return err
	}
	if err = c.client.ProduceSync(ctx, &kgo.Record{Topic: topic, Key: []byte(event.IdempotencyKey), Value: payload}).FirstErr(); err != nil {
		return err
	}
	return nil
}

package eventconsumer

import (
	"context"
	"encoding/json"
	"log/slog"

	"github.com/twmb/franz-go/pkg/kgo"

	"github.com/asamigentoku/PinguCoin/pkg/orderflow"
	"github.com/asamigentoku/PinguCoin/services/order-api/internal/repository"
)

type Consumer struct {
	client *kgo.Client
	repo   *repository.Repository
	logger *slog.Logger
}

func New(brokers []string, repo *repository.Repository, logger *slog.Logger) (*Consumer, error) {
	if len(brokers) == 0 {
		return &Consumer{repo: repo, logger: logger}, nil
	}
	client, err := kgo.NewClient(kgo.SeedBrokers(brokers...), kgo.ConsumerGroup("order-api-v1"), kgo.ConsumeTopics(orderflow.TopicInventoryReserved, orderflow.TopicInventoryRejected, orderflow.TopicPaymentSucceeded, orderflow.TopicPaymentFailed), kgo.DisableAutoCommit())
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
func (c *Consumer) Run(ctx context.Context) {
	if c.client == nil {
		return
	}
	for ctx.Err() == nil {
		fetches := c.client.PollFetches(ctx)
		fetches.EachRecord(func(record *kgo.Record) {
			if err := c.handle(record); err != nil {
				c.logger.Error("order event failed", "topic", record.Topic, "error", err)
				return
			}
			if err := c.client.CommitRecords(ctx, record); err != nil {
				c.logger.Error("order event commit failed", "error", err)
			}
		})
	}
}
func (c *Consumer) handle(record *kgo.Record) error {
	switch record.Topic {
	case orderflow.TopicInventoryReserved, orderflow.TopicInventoryRejected:
		var event orderflow.InventoryResult
		if err := json.Unmarshal(record.Value, &event); err != nil {
			return err
		}
		return c.repo.ApplyInventoryResult(event.OrderID, record.Topic == orderflow.TopicInventoryReserved, event.Reason)
	case orderflow.TopicPaymentSucceeded, orderflow.TopicPaymentFailed:
		var event orderflow.PaymentResult
		if err := json.Unmarshal(record.Value, &event); err != nil {
			return err
		}
		return c.repo.ApplyPaymentResult(event.OrderID, event.PaymentID, record.Topic == orderflow.TopicPaymentSucceeded, event.Reason)
	}
	return nil
}

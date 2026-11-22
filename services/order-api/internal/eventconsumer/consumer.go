package eventconsumer

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/twmb/franz-go/pkg/kgo"

	"github.com/asamigentoku/PinguCoin/pkg/consumerretry"
	"github.com/asamigentoku/PinguCoin/pkg/orderflow"
	"github.com/asamigentoku/PinguCoin/services/order-api/internal/repository"
)

type Consumer struct {
	client *kgo.Client
	repo   *repository.Repository
	logger *slog.Logger
}

// Consumer はKafkaから在庫・決済の結果を受け取り、注文状態を更新します。
func New(brokers []string, repo *repository.Repository, logger *slog.Logger) (*Consumer, error) {
	if len(brokers) == 0 {
		return &Consumer{repo: repo, logger: logger}, nil
	}
	client, err := kgo.NewClient(kgo.SeedBrokers(brokers...), kgo.ConsumerGroup("order-api-v1"), kgo.ConsumeTopics(orderflow.TopicInventoryReserved, orderflow.TopicInventoryRejected, orderflow.TopicPaymentSucceeded, orderflow.TopicPaymentFailed, orderflow.TopicPaymentActionRequired), kgo.ConsumeResetOffset(kgo.NewOffset().AtStart()), kgo.DisableAutoCommit())
	if err != nil {
		return nil, err
	}
	return &Consumer{client: client, repo: repo, logger: logger}, nil
}

// Close はKafkaコンシューマーを終了します。
func (c *Consumer) Close() {
	if c.client != nil {
		c.client.Close()
	}
}

// Run はイベント処理が成功した後にKafkaのoffsetをcommitします。
// DB更新に失敗した場合はcommitしないため、イベントを再処理できます。
func (c *Consumer) Run(ctx context.Context) error {
	if c.client == nil {
		return nil
	}
	for ctx.Err() == nil {
		fetches := c.client.PollFetches(ctx)
		for _, fetchErr := range fetches.Errors() {
			c.logger.Error("order event fetch failed", "topic", fetchErr.Topic, "partition", fetchErr.Partition, "error", fetchErr.Err)
		}
		for _, record := range fetches.Records() {
			if err := consumerretry.Do(ctx, c.logger, "order.handle", func(attemptCtx context.Context) error {
				return c.handle(attemptCtx, record)
			}); err != nil {
				return fmt.Errorf("order event handling exhausted: %w", err)
			}
			if err := consumerretry.Do(ctx, c.logger, "order.commit", func(attemptCtx context.Context) error {
				return c.client.CommitRecords(attemptCtx, record)
			}); err != nil {
				return fmt.Errorf("order offset commit exhausted: %w", err)
			}
		}
	}
	return nil
}

// handle はイベントの種類に応じて注文ステータスを更新します。
func (c *Consumer) handle(ctx context.Context, record *kgo.Record) error {
	switch record.Topic {
	case orderflow.TopicInventoryReserved, orderflow.TopicInventoryRejected:
		var event orderflow.InventoryResult
		if err := json.Unmarshal(record.Value, &event); err != nil {
			return err
		}
		return c.repo.ApplyInventoryResult(ctx, event.OrderID, record.Topic == orderflow.TopicInventoryReserved, event.Reason)
	case orderflow.TopicPaymentSucceeded, orderflow.TopicPaymentFailed, orderflow.TopicPaymentActionRequired:
		var event orderflow.PaymentResult
		if err := json.Unmarshal(record.Value, &event); err != nil {
			return err
		}
		if record.Topic == orderflow.TopicPaymentActionRequired {
			return c.repo.ApplyPaymentActionRequired(ctx, event.OrderID, event.PaymentID, event.ActionURL)
		}
		return c.repo.ApplyPaymentResult(ctx, event.OrderID, event.PaymentID, record.Topic == orderflow.TopicPaymentSucceeded, event.Reason)
	}
	return nil
}

package outbox

import (
	"context"
	"log/slog"
	"time"

	"github.com/twmb/franz-go/pkg/kgo"

	"github.com/asamigentoku/PinguCoin/services/payment-api/internal/repository"
)

type Publisher struct {
	client *kgo.Client
	repo   *repository.OutboxRepository
	logger *slog.Logger
}

func New(brokers []string, repo *repository.OutboxRepository, logger *slog.Logger) (*Publisher, error) {
	if len(brokers) == 0 {
		return &Publisher{repo: repo, logger: logger}, nil
	}
	client, err := kgo.NewClient(kgo.SeedBrokers(brokers...), kgo.RequiredAcks(kgo.AllISRAcks()))
	if err != nil {
		return nil, err
	}
	return &Publisher{client: client, repo: repo, logger: logger}, nil
}

func (p *Publisher) Close() {
	if p.client != nil {
		p.client.Close()
	}
}

func (p *Publisher) Run(ctx context.Context) error {
	if p.client == nil {
		return nil
	}
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	for {
		if err := p.publishBatch(ctx); err != nil {
			p.logger.Error("outbox publish failed", "error", err)
		}
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}
	}
}

func (p *Publisher) publishBatch(ctx context.Context) error {
	events, err := p.repo.Pending(100)
	if err != nil {
		return err
	}
	for _, event := range events {
		err := p.client.ProduceSync(ctx, &kgo.Record{Topic: event.Topic, Key: []byte(event.Key), Value: []byte(event.Payload)}).FirstErr()
		if err != nil {
			_ = p.repo.MarkAttempt(event.ID)
			return err
		}
		if err := p.repo.MarkPublished(event.ID); err != nil {
			return err
		}
	}
	return nil
}

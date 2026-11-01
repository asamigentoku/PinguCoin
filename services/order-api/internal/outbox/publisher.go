package outbox

import (
	"context"
	"github.com/asamigentoku/PinguCoin/services/order-api/internal/model"
	"github.com/twmb/franz-go/pkg/kgo"
	"gorm.io/gorm"
	"time"
)

type Publisher struct {
	db     *gorm.DB
	client *kgo.Client
}

func New(db *gorm.DB, brokers []string) (*Publisher, error) {
	if len(brokers) == 0 {
		return &Publisher{db: db}, nil
	}
	c, e := kgo.NewClient(kgo.SeedBrokers(brokers...), kgo.RequiredAcks(kgo.AllISRAcks()))
	if e != nil {
		return nil, e
	}
	return &Publisher{db: db, client: c}, nil
}
func (p *Publisher) Close() {
	if p.client != nil {
		p.client.Close()
	}
}
func (p *Publisher) Run(ctx context.Context) {
	if p.client == nil {
		return
	}
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			p.flush(ctx)
		}
	}
}
func (p *Publisher) flush(ctx context.Context) {
	var events []model.OutboxEvent
	if p.db.Where("published_at IS NULL").Order("id").Limit(100).Find(&events).Error != nil {
		return
	}
	for i := range events {
		event := &events[i]
		result := p.client.ProduceSync(ctx, &kgo.Record{Topic: event.Topic, Key: []byte(event.EventKey), Value: event.Payload})
		if e := result.FirstErr(); e != nil {
			p.db.Model(event).Updates(map[string]any{"attempts": gorm.Expr("attempts + 1"), "last_error": e.Error()})
			continue
		}
		now := time.Now()
		p.db.Model(event).Updates(map[string]any{"published_at": now, "last_error": ""})
	}
}

package outbox

import (
	"context"
	"github.com/asamigentoku/PinguCoin/pkg/consumerretry"
	"github.com/asamigentoku/PinguCoin/services/order-api/internal/model"
	"github.com/twmb/franz-go/pkg/kgo"
	"gorm.io/gorm"
	"log/slog"
	"time"
)

type Publisher struct {
	db     *gorm.DB
	client *kgo.Client
	logger *slog.Logger
}

// Publisher はoutboxテーブルに保存された未発行イベントをKafkaへ送信します。
// 注文保存とKafka発行を分離し、Kafka障害時もイベントを再送できるようにします。
func New(db *gorm.DB, brokers []string, logger *slog.Logger) (*Publisher, error) {
	if len(brokers) == 0 {
		return &Publisher{db: db, logger: logger}, nil
	}
	c, e := kgo.NewClient(kgo.SeedBrokers(brokers...), kgo.RequiredAcks(kgo.AllISRAcks()))
	if e != nil {
		return nil, e
	}
	return &Publisher{db: db, client: c, logger: logger}, nil
}

// Close はKafkaプロデューサーを終了します。
func (p *Publisher) Close() {
	if p.client != nil {
		p.client.Close()
	}
}

// Run は一定間隔で未発行イベントを検索し、Kafkaへ発行します。
func (p *Publisher) Run(ctx context.Context) {
	if p.client == nil {
		return
	}
    //定期実行
	ticker := time.NewTicker(time.Second)
	//この関数を抜けるときに必ずタイマーを停止
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
            // アプリ終了時などに ctx がキャンセルされたら return する
    		// → この瞬間、上の defer ticker.Stop() が実行されて安全に終了する！
			return
		case <-ticker.C:
            // 1秒ごとにここが実行される（例：Outboxテーブルから未送信イベントを取得してKafkaへ送信）
			p.flush(ctx)
		}
	}
}

// flush は未発行イベントを最大100件取得してKafkaへ送信します。
// 発行成功時だけpublished_atを更新し、失敗時は再試行情報を保存します。
func (p *Publisher) flush(ctx context.Context) {
	var events []model.OutboxEvent
	//まだ発行されていない100件を取り出して
	if p.db.Where("published_at IS NULL").Order("id").Limit(100).Find(&events).Error != nil {
		return
	}
	for i := range events {
		event := &events[i]
		e := consumerretry.Do(ctx, p.logger, "order.outbox.publish", func(attemptCtx context.Context) error {
			return p.client.ProduceSync(attemptCtx, &kgo.Record{Topic: event.Topic, Key: []byte(event.EventKey), Value: event.Payload}).FirstErr()
		})
		if e != nil {
		    //エラーが起きた場合のエラー名とイベントがpublishされた時間を記録と試行回数を記録
			p.db.Model(event).Updates(map[string]any{"attempts": gorm.Expr("attempts + 1"), "last_error": e.Error()})
			continue
		}
		now := time.Now()
		//エラーが起きた場合のエラー名とイベントがpublishされた時間を記録
		p.db.Model(event).Updates(map[string]any{"published_at": now, "last_error": ""})
	}
}

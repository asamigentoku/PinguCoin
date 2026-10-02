// Package cache は Redis を使った読み取りキャッシュ(pkg/cache)。
//
// キャッシュは「あれば速くなる」だけの仕組みなので、次の2つを守る。
//   - 無効(REDIS_ENABLED=false)のときは nil の *Cache を使う。nil に対するメソッドは、何もしない(常にミス)。
//   - Redis が落ちている・遅いときも、エラーにせず、ミスとして扱う(呼び出し側は DB から読む)。
//
// 値は「ハッシュ(キー)+ フィールド」で持つ。無効化はハッシュごと消す(DEL)ので、
// フィールドが増えても(例: ユーザー別の一覧)、1回の呼び出しで全部消せる。
package cache

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/asamigentoku/PinguCoin/pkg/metrics"
)

// opTimeout は Redis 1回の操作の上限。これを超えたら諦めて DB に回す(Redis の遅延でAPIを遅くしない)。
const opTimeout = 200 * time.Millisecond

// Config は Redis への接続設定。
type Config struct {
	Addr     string
	Password string
	DB       int
	// TTL は、ハッシュを最初に書いてからの有効期間。無効化のすり抜け(読み取りと更新の競合)の
	// 影響を、この時間までに抑える。
	TTL time.Duration
}

// Cache は Redis のキャッシュ。
type Cache struct {
	client *redis.Client
	ttl    time.Duration
	logger *slog.Logger
}

// New は Redis のクライアントを作る(接続は最初の操作で張られる。ここでは接続を確認しない)。
func New(cfg Config, logger *slog.Logger) *Cache {
	return &Cache{
		client: redis.NewClient(&redis.Options{
			Addr:         cfg.Addr,
			Password:     cfg.Password,
			DB:           cfg.DB,
			DialTimeout:  opTimeout,
			ReadTimeout:  opTimeout,
			WriteTimeout: opTimeout,
		}),
		ttl:    cfg.TTL,
		logger: logger,
	}
}

// Get は値を取り出して dst に入れる。あれば true、無い・失敗したときは false。
func (c *Cache) Get(ctx context.Context, name, hash, field string, dst any) bool {
	if c == nil {
		return false
	}
	ctx, cancel := context.WithTimeout(ctx, opTimeout)
	defer cancel()

	raw, err := c.client.HGet(ctx, hash, field).Bytes()
	switch {
	case errors.Is(err, redis.Nil):
		metrics.RecordCache(name, "miss")
		return false
	case err != nil:
		c.warn("cache get failed", hash, err)
		metrics.RecordCache(name, "error")
		return false
	}
	if err := json.Unmarshal(raw, dst); err != nil {
		// 形式が合わない値(古い版が書いたものなど)は、ミスとして扱い、次の書き込みで上書きされる。
		c.warn("cache decode failed", hash, err)
		metrics.RecordCache(name, "error")
		return false
	}
	metrics.RecordCache(name, "hit")
	return true
}

// Set は値を書く。失敗しても、エラーにはしない(ログだけ)。
func (c *Cache) Set(ctx context.Context, hash, field string, value any) {
	if c == nil {
		return
	}
	raw, err := json.Marshal(value)
	if err != nil {
		c.warn("cache encode failed", hash, err)
		return
	}
	ctx, cancel := context.WithTimeout(ctx, opTimeout)
	defer cancel()

	pipe := c.client.Pipeline()
	pipe.HSet(ctx, hash, field, raw)
	pipe.ExpireNX(ctx, hash, c.ttl) // TTL は、ハッシュを最初に書いたときだけ付ける(書くたびに延びない)
	if _, err := pipe.Exec(ctx); err != nil {
		c.warn("cache set failed", hash, err)
	}
}

// Invalidate はハッシュごと消す。データを更新したあとに呼ぶ。失敗しても、エラーにはしない
// (その場合は、TTL が切れるまで古い値が残る)。
func (c *Cache) Invalidate(ctx context.Context, hashes ...string) {
	if c == nil || len(hashes) == 0 {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, opTimeout)
	defer cancel()
	if err := c.client.Del(ctx, hashes...).Err(); err != nil {
		c.warn("cache invalidate failed", hashes[0], err)
	}
}

// Close は接続を閉じる。
func (c *Cache) Close() error {
	if c == nil {
		return nil
	}
	return c.client.Close()
}

func (c *Cache) warn(msg, hash string, err error) {
	if c.logger != nil {
		c.logger.Warn(msg, slog.String("key", hash), slog.String("error", err.Error()))
	}
}

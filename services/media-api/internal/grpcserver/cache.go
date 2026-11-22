package grpcserver

import (
	"context"
	"fmt"
	"time"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	"github.com/asamigentoku/PinguCoin/services/media-api/internal/cache"
)

func productCacheKey(id uint32) string { return fmt.Sprintf("media:product:%d", id) }

func invalidateProduct(ctx context.Context, c cache.Cache, id uint32) {
	_ = c.Delete(ctx, productCacheKey(id))
}

func cacheGet(ctx context.Context, c cache.Cache, key string, message proto.Message) (bool, error) {
	data, found, err := c.Get(ctx, key)
	if err != nil || !found {
		return false, err
	}
	if err := protojson.Unmarshal(data, message); err != nil {
		_ = c.Delete(ctx, key)
		return false, err
	}
	return true, nil
}

func cacheSet(ctx context.Context, c cache.Cache, key string, message proto.Message, ttlSeconds int) error {
	data, err := protojson.Marshal(message)
	if err != nil {
		return err
	}
	return c.Set(ctx, key, data, time.Duration(ttlSeconds)*time.Second)
}

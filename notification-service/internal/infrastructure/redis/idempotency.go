package redis

import (
	"context"
	"fmt"
	"time"

	goredis "github.com/redis/go-redis/v9"
)

const processingTTL = time.Hour

type IdempotencyStore struct {
	client *goredis.Client
}

func NewIdempotencyStore(client *goredis.Client) *IdempotencyStore {
	return &IdempotencyStore{client: client}
}

func (s *IdempotencyStore) TryClaim(ctx context.Context, orderID string) (claimed bool, alreadySent bool, err error) {
	key := sentKey(orderID)
	ok, err := s.client.SetNX(ctx, key, "processing", processingTTL).Result()
	if err != nil {
		return false, false, err
	}
	if ok {
		return true, false, nil
	}

	val, err := s.client.Get(ctx, key).Result()
	if err == goredis.Nil {
		return false, false, nil
	}
	if err != nil {
		return false, false, err
	}

	return false, val == "sent", nil
}

func (s *IdempotencyStore) MarkSent(ctx context.Context, orderID string) error {
	return s.client.Set(ctx, sentKey(orderID), "sent", 0).Err()
}

func (s *IdempotencyStore) Release(ctx context.Context, orderID string) error {
	return s.client.Del(ctx, sentKey(orderID)).Err()
}

func sentKey(orderID string) string {
	return fmt.Sprintf("notification:sent:%s", orderID)
}

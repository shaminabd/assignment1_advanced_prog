package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	goredis "github.com/redis/go-redis/v9"

	"order-service/internal/domain"
)

type OrderCache struct {
	client *goredis.Client
	ttl    time.Duration
}

func NewOrderCache(client *goredis.Client, ttl time.Duration) *OrderCache {
	return &OrderCache{client: client, ttl: ttl}
}

func (c *OrderCache) Get(ctx context.Context, id string) (*domain.Order, bool, error) {
	data, err := c.client.Get(ctx, orderKey(id)).Bytes()
	if err == goredis.Nil {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}

	var order domain.Order
	if err := json.Unmarshal(data, &order); err != nil {
		return nil, false, err
	}

	return &order, true, nil
}

func (c *OrderCache) Set(ctx context.Context, order domain.Order) error {
	data, err := json.Marshal(order)
	if err != nil {
		return err
	}

	return c.client.Set(ctx, orderKey(order.ID), data, c.ttl).Err()
}

func (c *OrderCache) Delete(ctx context.Context, id string) error {
	return c.client.Del(ctx, orderKey(id)).Err()
}

func orderKey(id string) string {
	return fmt.Sprintf("order:%s", id)
}

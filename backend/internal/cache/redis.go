// Package cache contains the optional Redis result-cache adapter.
package cache

import (
	"context"
	"errors"
	"github.com/redis/go-redis/v9"
	"time"
)

type Redis struct{ client *redis.Client }

func NewRedis(addr, password string) *Redis {
	const timeout = 80 * time.Millisecond
	return &Redis{client: redis.NewClient(&redis.Options{
		Addr: addr, Password: password, Protocol: 2, DisableIdentity: true,
		MaxRetries: -1, DialerRetries: 1, DialTimeout: timeout, ReadTimeout: timeout, WriteTimeout: timeout,
		PoolTimeout: timeout, PoolSize: 8, MaxActiveConns: 8, ContextTimeoutEnabled: true,
	})}
}
func (c *Redis) Get(ctx context.Context, key string) ([]byte, error) {
	raw, err := c.client.Get(ctx, key).Bytes()
	if errors.Is(err, redis.Nil) {
		return nil, nil
	}
	return raw, err
}
func (c *Redis) Set(ctx context.Context, key string, raw []byte, ttl time.Duration) error {
	return c.client.Set(ctx, key, raw, ttl).Err()
}
func (c *Redis) Close() error { return c.client.Close() }

// Package redis provides a thin wrapper around go-redis/v9 with connection
// pooling, cache get/set/delete operations, and pattern-based key invalidation.
package redis

import (
	"context"
	"fmt"
	"time"

	"github.com/Adithya-Monish-Kumar-K/Distributed-Search-Analytics-Platform/pkg/config"
	"github.com/redis/go-redis/v9"
)

// Client wraps a go-redis client.
type Client struct {
	rdb *redis.Client
}

// NewClient creates a Redis client and verifies the connection with a PING.
func NewClient(cfg config.RedisConfig) (*Client, error) {
	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.Addr,
		Password: cfg.Password,
		DB:       cfg.DB,
		PoolSize: cfg.PoolSize,
	})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := rdb.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("redis ping failed: %w", err)
	}
	return &Client{rdb: rdb}, nil
}

// Get returns the string value for the given key.
func (c *Client) Get(ctx context.Context, key string) (string, error) {
	return c.rdb.Get(ctx, key).Result()
}

// Set stores a value with the given TTL.
func (c *Client) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	return c.rdb.Set(ctx, key, value, ttl).Err()
}

// Del deletes one or more keys.
func (c *Client) Del(ctx context.Context, keys ...string) error {
	return c.rdb.Del(ctx, keys...).Err()
}

// FlushByPattern scans for keys matching the glob pattern and deletes them,
// returning the number of keys removed.
func (c *Client) FlushByPattern(ctx context.Context, pattern string) (int64, error) {
	var deleted int64
	iter := c.rdb.Scan(ctx, 0, pattern, 100).Iterator()
	for iter.Next(ctx) {
		if err := c.rdb.Del(ctx, iter.Val()).Err(); err != nil {
			return deleted, fmt.Errorf("deleting key %s: %w", iter.Val(), err)
		}
		deleted++
	}
	if err := iter.Err(); err != nil {
		return deleted, fmt.Errorf("scanning pattern %s: %w", pattern, err)
	}
	return deleted, nil
}

// IsNilError reports whether err is a Redis nil (key-not-found) error.
func IsNilError(err error) bool {
	return err == redis.Nil
}

// Close closes the underlying Redis connection.
func (c *Client) Close() error {
	return c.rdb.Close()
}

// Ping sends a PING to Redis and returns any error.
func (c *Client) Ping(ctx context.Context) error {
	return c.rdb.Ping(ctx).Err()
}

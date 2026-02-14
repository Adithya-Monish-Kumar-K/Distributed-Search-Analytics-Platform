// Package cache provides a Redis-backed query cache with singleflight
// deduplication. Queries are normalised and hashed so that semantically
// identical searches share the same cache entry.
package cache

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"sync/atomic"

	"github.com/Adithya-Monish-Kumar-K/Distributed-Search-Analytics-Platform/internal/searcher/executor"
	"github.com/Adithya-Monish-Kumar-K/Distributed-Search-Analytics-Platform/pkg/config"
	pkgredis "github.com/Adithya-Monish-Kumar-K/Distributed-Search-Analytics-Platform/pkg/redis"
	"golang.org/x/sync/singleflight"
)

const keyPrefix = "search:"

// QueryCache wraps a Redis client with singleflight de-duplication and
// hit/miss counters.
type QueryCache struct {
	client *pkgredis.Client
	cfg    config.RedisConfig
	group  singleflight.Group
	logger *slog.Logger
	hits   atomic.Int64
	misses atomic.Int64
}

// New creates a QueryCache backed by the given Redis client.
func New(client *pkgredis.Client, cfg config.RedisConfig) *QueryCache {
	return &QueryCache{
		client: client,
		cfg:    cfg,
		logger: slog.Default().With("component", "query-cache"),
	}
}

// Get reads a cached search result. Returns (nil, false) on miss or error.
func (c *QueryCache) Get(ctx context.Context, query string, limit int) (*executor.SearchResult, bool) {
	key := c.buildKey(query, limit)
	data, err := c.client.Get(ctx, key)
	if err != nil {
		if pkgredis.IsNilError(err) {
			c.misses.Add(1)
			return nil, false
		}
		c.logger.Error("cache get failed", "key", key, "error", err)
		c.misses.Add(1)
		return nil, false
	}
	var result executor.SearchResult
	if err := json.Unmarshal([]byte(data), &result); err != nil {
		c.logger.Error("cache unmarshal failed", "key", key, "err", err)
		c.misses.Add(1)
		return nil, false
	}
	c.hits.Add(1)
	c.logger.Debug("cache hit", "query", query, "key", key)
	return &result, true
}

// Set stores a search result in the cache with the configured TTL.
func (c *QueryCache) Set(ctx context.Context, query string, limit int, result *executor.SearchResult) {
	key := c.buildKey(query, limit)
	data, err := json.Marshal(result)
	if err != nil {
		c.logger.Error("cache marshal failed", "key", key, "error", err)
		return
	}
	if err := c.client.Set(ctx, key, data, c.cfg.CacheTTL); err != nil {
		c.logger.Error("cache set failed", "key", key, "error", err)
	}
}

// GetOrCompute returns a cached result if available; otherwise invokes
// computeFn, caches the outcome, and returns it. A singleflight group
// prevents thundering-herd cache-miss storms.
func (c *QueryCache) GetOrCompute(
	ctx context.Context,
	query string,
	limit int,
	computeFn func() (*executor.SearchResult, error),
) (*executor.SearchResult, bool, error) {
	if result, ok := c.Get(ctx, query, limit); ok {
		return result, true, nil
	}
	key := c.buildKey(query, limit)
	val, err, _ := c.group.Do(key, func() (interface{}, error) {
		if result, ok := c.Get(ctx, query, limit); ok {
			return result, nil
		}
		result, err := computeFn()
		if err != nil {
			return nil, err
		}
		c.Set(ctx, query, limit, result)
		return result, nil
	})
	if err != nil {
		return nil, false, err
	}
	return val.(*executor.SearchResult), false, nil
}

// Invalidate flushes all search-cache keys from Redis.
func (c *QueryCache) Invalidate(ctx context.Context) error {
	pattern := keyPrefix + "*"
	deleted, err := c.client.FlushByPattern(ctx, pattern)
	if err != nil {
		return fmt.Errorf("invalidating cache: %w", err)
	}
	c.logger.Info("cache invalidate", "keys_deleted", deleted)
	return nil
}

// Stats returns the cumulative hit and miss counters.
func (c *QueryCache) Stats() (hits, misses int64) {
	return c.hits.Load(), c.misses.Load()
}

// buildKey produces a deterministic SHA-256 cache key for the normalised
// query and limit.
func (c *QueryCache) buildKey(query string, limit int) string {
	normalized := normalizeQuery(query)
	raw := fmt.Sprintf("%s:limit=%d", normalized, limit)
	hash := sha256.Sum256([]byte(raw))
	return fmt.Sprintf("%s%x", keyPrefix, hash[:16])
}

// normalizeQuery canonicalises a query string by lower-casing, extracting
// terms and excludes, sorting them, and joining with a separator.
func normalizeQuery(query string) string {
	words := strings.Fields(strings.ToLower(query))
	terms := make([]string, 0)
	excludes := make([]string, 0)
	queryType := "AND"
	excludeNext := false
	for _, w := range words {
		upper := strings.ToUpper(w)
		switch upper {
		case "AND":
			queryType = "AND"
		case "OR":
			queryType = "OR"
		case "NOT":
			excludeNext = true
		default:
			if excludeNext {
				excludes = append(excludes, w)
				excludeNext = false
			} else {
				terms = append(terms, w)
			}
		}
	}

	sort.Strings(terms)
	sort.Strings(excludes)
	parts := []string{queryType, strings.Join(terms, ",")}
	if len(excludes) > 0 {
		parts = append(parts, "NOT:"+strings.Join(excludes, ","))
	}
	return strings.Join(parts, "|")
}

// Package shard provides hash-based shard routing for index engines. Each
// shard owns an independent indexer.Engine instance backed by its own data
// directory, and the Router dispatches documents by shard ID.
package shard

import (
	"fmt"
	"log/slog"
	"path/filepath"
	"sync"

	"github.com/Adithya-Monish-Kumar-K/Distributed-Search-Analytics-Platform/internal/indexer"
	"github.com/Adithya-Monish-Kumar-K/Distributed-Search-Analytics-Platform/pkg/config"
)

// Router maps shard IDs to dedicated indexer.Engine instances.
type Router struct {
	engines   map[int]*indexer.Engine
	mu        sync.RWMutex
	baseCfg   config.IndexerConfig
	numShards int
	logger    *slog.Logger
}

// NewRouter creates numShards engines, each in its own sub-directory under
// baseCfg.DataDir.
func NewRouter(baseCfg config.IndexerConfig, numShards int) (*Router, error) {
	r := &Router{
		engines:   make(map[int]*indexer.Engine, numShards),
		baseCfg:   baseCfg,
		numShards: numShards,
		logger:    slog.Default().With("component", "shard-router"),
	}
	for i := 0; i < numShards; i++ {
		shardCfg := baseCfg
		shardCfg.DataDir = filepath.Join(baseCfg.DataDir, fmt.Sprintf("shard-%d", i))
		engine, err := indexer.NewEngine(shardCfg)
		if err != nil {
			r.closeAll()
			return nil, fmt.Errorf("creating engine for shard %d: %w", i, err)
		}
		r.engines[i] = engine
		r.logger.Info("shard engine initialized",
			"shard_id", i,
			"data_dir", shardCfg.DataDir,
		)
	}
	r.logger.Info("shard router ready", "num_shards", numShards)
	return r, nil
}

// Route returns the Engine responsible for the given shard ID.
func (r *Router) Route(shardID int) (*indexer.Engine, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	engine, ok := r.engines[shardID]
	if !ok {
		return nil, fmt.Errorf("unknown shard ID %d (valid range: 0-%d)", shardID, r.numShards-1)
	}
	return engine, nil
}

// GetAllEngines returns a snapshot map of all shard engines.
func (r *Router) GetAllEngines() map[int]*indexer.Engine {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make(map[int]*indexer.Engine, len(r.engines))
	for id, engine := range r.engines {
		result[id] = engine
	}
	return result
}

// NumShards returns the number of shards managed by this router.
func (r *Router) NumShards() int {
	return r.numShards
}

// FlushAll flushes every shard engine to disk.
func (r *Router) FlushAll() error {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var firstErr error
	for id, engine := range r.engines {
		if err := engine.Flush(); err != nil {
			r.logger.Error("flush failed", "shard_id", id, "error", err)
			if firstErr == nil {
				firstErr = err
			}
		}
	}
	return firstErr
}

// ReloadAll tells every shard engine to re-scan for newly flushed segments.
// Returns the total number of new segments loaded across all shards.
func (r *Router) ReloadAll() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	total := 0
	for _, engine := range r.engines {
		total += engine.ReloadSegments()
	}
	return total
}

// Close flushes and closes every shard engine.
func (r *Router) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.closeAll()
}

// closeAll closes every shard engine, collecting the first error encountered.
func (r *Router) closeAll() error {
	var firstErr error
	for id, engine := range r.engines {
		if err := engine.Close(); err != nil {
			r.logger.Error("close failed", "shard_id", id, "error", err)
			if firstErr == nil {
				firstErr = err
			}
		}
	}
	return firstErr
}

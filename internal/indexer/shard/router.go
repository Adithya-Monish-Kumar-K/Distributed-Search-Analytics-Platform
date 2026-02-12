package shard

import (
	"fmt"
	"log/slog"
	"path/filepath"
	"sync"

	"github.com/Adithya-Monish-Kumar-K/Distributed-Search-Analytics-Platform/internal/indexer"
	"github.com/Adithya-Monish-Kumar-K/Distributed-Search-Analytics-Platform/pkg/config"
)

type Router struct {
	engines   map[int]*indexer.Engine
	mu        sync.RWMutex
	baseCfg   config.IndexerConfig
	numShards int
	logger    *slog.Logger
}

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

func (r *Router) Route(shardID int) (*indexer.Engine, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	engine, ok := r.engines[shardID]
	if !ok {
		return nil, fmt.Errorf("unknown shard ID %d (valid range: 0-%d)", shardID, r.numShards-1)
	}
	return engine, nil
}

func (r *Router) GetAllEngines() map[int]*indexer.Engine {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make(map[int]*indexer.Engine, len(r.engines))
	for id, engine := range r.engines {
		result[id] = engine
	}
	return result
}

func (r *Router) NumShards() int {
	return r.numShards
}

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

func (r *Router) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.closeAll()
}

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

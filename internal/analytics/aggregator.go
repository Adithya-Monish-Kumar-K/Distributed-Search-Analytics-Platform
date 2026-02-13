package analytics

import (
	"context"
	"log/slog"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Adithya-Monish-Kumar-K/Distributed-Search-Analytics-Platform/pkg/kafka"
)

type AggregatedStats struct {
	TotalSearches     int64        `json:"total_searches"`
	TotalDocIndexed   int64        `json:"total_docs_indexed"`
	CacheHits         int64        `json:"cache_hits"`
	CacheMisses       int64        `json:"cache_misses"`
	ZeroResultCount   int64        `json:"zero_result_count"`
	AvgLatencyMs      float64      `json:"avg_latency_ms"`
	P50LatencyMs      int64        `json:"p50_latency_ms"`
	P95LatencyMs      int64        `json:"p95_latency_ms"`
	P99LatencyMs      int64        `json:"p99_latency_ms"`
	TopQueries        []QueryCount `json:"top_queries"`
	ZeroResultQueries []QueryCount `json:"zero_result_queries"`
	QueriesPerMinute  float64      `json:"queries_per_minute"`
}
type QueryCount struct {
	Query string `json:"query"`
	Count int64  `json:"count"`
}
type Aggregator struct {
	mu                sync.RWMutex
	totalSearches     atomic.Int64
	totalDocIndexed   atomic.Int64
	cacheHits         atomic.Int64
	cacheMisses       atomic.Int64
	zeroResults       atomic.Int64
	latencies         []int64
	queryCounts       map[string]int64
	zeroResultQueries map[string]int64
	startTime         time.Time

	consumer *kafka.Consumer
	logger   *slog.Logger
}

func NewAggregator(consumer *kafka.Consumer) *Aggregator {
	return &Aggregator{
		latencies:         make([]int64, 0, 10000),
		queryCounts:       make(map[string]int64),
		zeroResultQueries: make(map[string]int64),
		startTime:         time.Now(),
		consumer:          consumer,
		logger:            slog.Default().With("component", "analytics-aggregator"),
	}
}
func (a *Aggregator) Start(ctx context.Context) error {
	a.logger.Info("analytics aggregator starting")
	return a.consumer.Start(ctx)
}
func HandleEvent(agg *Aggregator) kafka.MessageHandler {
	return func(ctx context.Context, key []byte, value []byte) error {
		event, err := kafka.DecodeJSON[SearchEvent](value)
		if err != nil {
			idxEvent, idxErr := kafka.DecodeJSON[IndexEvent](value)
			if idxErr != nil {
				agg.logger.Error("failed to decode analytics event",
					"error", err,
				)
				return nil
			}
			agg.recordIndexEvent(idxEvent)
			return nil
		}
		agg.recordSearchEvent(event)
		return nil
	}
}

func (a *Aggregator) recordSearchEvent(event SearchEvent) {
	a.totalSearches.Add(1)

	if event.CacheHit {
		a.cacheHits.Add(1)
	} else {
		a.cacheMisses.Add(1)
	}

	if event.TotalHits == 0 {
		a.zeroResults.Add(1)
	}

	a.mu.Lock()
	a.latencies = append(a.latencies, event.LatencyMs)
	a.queryCounts[event.Query]++
	if event.TotalHits == 0 {
		a.zeroResultQueries[event.Query]++
	}
	a.mu.Unlock()
}

func (a *Aggregator) recordIndexEvent(event IndexEvent) {
	a.totalDocIndexed.Add(1)
}
func (a *Aggregator) Stats() AggregatedStats {
	a.mu.RLock()
	defer a.mu.RUnlock()

	stats := AggregatedStats{
		TotalSearches:   a.totalSearches.Load(),
		TotalDocIndexed: a.totalDocIndexed.Load(),
		CacheHits:       a.cacheHits.Load(),
		CacheMisses:     a.cacheMisses.Load(),
		ZeroResultCount: a.zeroResults.Load(),
	}
	if len(a.latencies) > 0 {
		sorted := make([]int64, len(a.latencies))
		copy(sorted, a.latencies)
		sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })

		var sum int64
		for _, l := range sorted {
			sum += l
		}
		stats.AvgLatencyMs = float64(sum) / float64(len(sorted))
		stats.P50LatencyMs = percentile(sorted, 50)
		stats.P95LatencyMs = percentile(sorted, 95)
		stats.P99LatencyMs = percentile(sorted, 99)
	}
	stats.TopQueries = topN(a.queryCounts, 10)
	stats.ZeroResultQueries = topN(a.zeroResultQueries, 10)
	elapsed := time.Since(a.startTime).Minutes()
	if elapsed > 0 {
		stats.QueriesPerMinute = float64(stats.TotalSearches) / elapsed
	}

	return stats
}

func percentile(sorted []int64, pct int) int64 {
	if len(sorted) == 0 {
		return 0
	}
	idx := (pct * len(sorted)) / 100
	if idx >= len(sorted) {
		idx = len(sorted) - 1
	}
	return sorted[idx]
}

func topN(counts map[string]int64, n int) []QueryCount {
	result := make([]QueryCount, 0, len(counts))
	for query, count := range counts {
		result = append(result, QueryCount{Query: query, Count: count})
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Count > result[j].Count
	})
	if len(result) > n {
		result = result[:n]
	}
	return result
}

package benchmark

import (
	"context"
	"fmt"
	"testing"

	"github.com/Adithya-Monish-Kumar-K/Distributed-Search-Analytics-Platform/internal/indexer"
	"github.com/Adithya-Monish-Kumar-K/Distributed-Search-Analytics-Platform/internal/indexer/index"
	"github.com/Adithya-Monish-Kumar-K/Distributed-Search-Analytics-Platform/internal/searcher/executor"
	"github.com/Adithya-Monish-Kumar-K/Distributed-Search-Analytics-Platform/internal/searcher/parser"
	"github.com/Adithya-Monish-Kumar-K/Distributed-Search-Analytics-Platform/internal/searcher/ranker"
	"github.com/Adithya-Monish-Kumar-K/Distributed-Search-Analytics-Platform/pkg/config"
)

// BenchmarkQueryParse measures query parsing latency for queries of varying
// complexity.
func BenchmarkQueryParse(b *testing.B) {
	queries := []struct {
		name  string
		query string
	}{
		{"simple", "distributed systems"},
		{"boolean_and", "search AND analytics AND platform"},
		{"boolean_or", "indexing OR caching OR ranking"},
		{"with_not", "distributed NOT monolithic"},
		{"complex", "search AND ranking OR analytics NOT deprecated"},
		{"long", "distributed search analytics platform indexing query processing ranking caching sharding"},
	}

	for _, q := range queries {
		b.Run(q.name, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				plan := parser.Parse(q.query)
				_ = plan
			}
		})
	}
}

// BenchmarkBM25Ranking measures BM25 scoring and sorting for different
// posting-list sizes.
func BenchmarkBM25Ranking(b *testing.B) {
	sizes := []int{100, 1000, 10000}
	for _, numDocs := range sizes {
		b.Run(fmt.Sprintf("docs_%d", numDocs), func(b *testing.B) {
			postings := make(map[string]index.PostingList)
			term := "search"
			pl := make(index.PostingList, numDocs)
			for i := 0; i < numDocs; i++ {
				pl[i] = index.Posting{
					DocID:     fmt.Sprintf("doc-%d", i),
					Frequency: (i % 10) + 1,
					Positions: []int{0, 5, 10},
				}
			}
			postings[term] = pl

			params := ranker.RankParams{
				TotalDocs:    int64(numDocs * 2),
				AvgDocLength: 150.0,
			}
			getDocInfo := func(docID string) ranker.DocInfo {
				return ranker.DocInfo{DocLength: 100 + (len(docID) * 10)}
			}

			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				ranked := ranker.Rank(postings, params, getDocInfo, 10)
				_ = ranked
			}
		})
	}
}

// BenchmarkBM25MultiTerm measures BM25 ranking with an increasing number of
// query terms.
func BenchmarkBM25MultiTerm(b *testing.B) {
	termCount := []int{1, 3, 5, 10}
	for _, tc := range termCount {
		b.Run(fmt.Sprintf("terms_%d", tc), func(b *testing.B) {
			postings := make(map[string]index.PostingList)
			for t := 0; t < tc; t++ {
				term := fmt.Sprintf("term%d", t)
				pl := make(index.PostingList, 500)
				for i := 0; i < 500; i++ {
					pl[i] = index.Posting{
						DocID:     fmt.Sprintf("doc-%d", i),
						Frequency: (i % 5) + 1,
						Positions: []int{t * 10},
					}
				}
				postings[term] = pl
			}

			params := ranker.RankParams{
				TotalDocs:    5000,
				AvgDocLength: 200.0,
			}
			getDocInfo := func(docID string) ranker.DocInfo {
				return ranker.DocInfo{DocLength: 180}
			}

			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				ranked := ranker.Rank(postings, params, getDocInfo, 10)
				_ = ranked
			}
		})
	}
}

// BenchmarkShardedExecutor exercises the sharded query executor with varying
// shard counts.
func BenchmarkShardedExecutor(b *testing.B) {
	shardCounts := []int{1, 4, 8}
	for _, numShards := range shardCounts {
		b.Run(fmt.Sprintf("shards_%d", numShards), func(b *testing.B) {
			engines := make(map[int]*indexer.Engine)
			for s := 0; s < numShards; s++ {
				cfg := config.IndexerConfig{
					DataDir:        b.TempDir(),
					SegmentMaxSize: 100 * 1024 * 1024,
					FlushInterval:  0,
				}
				engine, err := indexer.NewEngine(cfg)
				if err != nil {
					b.Fatal(err)
				}
				defer engine.Close()

				for d := 0; d < 1000; d++ {
					docID := fmt.Sprintf("shard%d-doc%d", s, d)
					engine.IndexDocument(docID, "distributed search",
						"search analytics platform with distributed indexing and query ranking")
				}
				engines[s] = engine
			}

			exec := executor.NewSharded(engines)
			plan := parser.Parse("distributed search")

			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				result, err := exec.Execute(context.Background(), plan, 10)
				if err != nil {
					b.Fatal(err)
				}
				_ = result
			}
		})
	}
}

// BenchmarkShardedExecutorParallel measures concurrent sharded search
// throughput across 8 shards.
func BenchmarkShardedExecutorParallel(b *testing.B) {
	engines := make(map[int]*indexer.Engine)
	for s := 0; s < 8; s++ {
		cfg := config.IndexerConfig{
			DataDir:        b.TempDir(),
			SegmentMaxSize: 100 * 1024 * 1024,
			FlushInterval:  0,
		}
		engine, err := indexer.NewEngine(cfg)
		if err != nil {
			b.Fatal(err)
		}
		defer engine.Close()

		for d := 0; d < 1000; d++ {
			docID := fmt.Sprintf("shard%d-doc%d", s, d)
			engine.IndexDocument(docID, "distributed search analytics",
				"platform with distributed search indexing query processing and ranking engine")
		}
		engines[s] = engine
	}

	exec := executor.NewSharded(engines)
	plan := parser.Parse("distributed search")

	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			result, err := exec.Execute(context.Background(), plan, 10)
			if err != nil {
				b.Fatal(err)
			}
			_ = result
		}
	})
}

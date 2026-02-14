// Package benchmark contains Go benchmarks for the indexer engine, memory
// index, and search pipeline, measuring throughput and allocation behaviour.
package benchmark

import (
	"fmt"
	"testing"

	"github.com/Adithya-Monish-Kumar-K/Distributed-Search-Analytics-Platform/internal/indexer"
	"github.com/Adithya-Monish-Kumar-K/Distributed-Search-Analytics-Platform/internal/indexer/index"
	"github.com/Adithya-Monish-Kumar-K/Distributed-Search-Analytics-Platform/pkg/config"
)

// BenchmarkMemoryIndexAdd measures per-document insert throughput into the
// in-memory inverted index.
func BenchmarkMemoryIndexAdd(b *testing.B) {
	mi := index.NewMemoryIndex()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		docID := fmt.Sprintf("doc-%d", i)
		mi.AddDocument(docID, "benchmark title", "this is a benchmark document with several terms for testing the indexing performance of our memory index")
	}
}

// BenchmarkMemoryIndexSearch measures single-term lookup latency over 10 000
// documents.
func BenchmarkMemoryIndexSearch(b *testing.B) {
	mi := index.NewMemoryIndex()
	for i := 0; i < 10000; i++ {
		docID := fmt.Sprintf("doc-%d", i)
		mi.AddDocument(docID, "distributed search", "search engine with distributed indexing and query processing")
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		results := mi.Search("search")
		_ = results
	}
}

// BenchmarkMemoryIndexSearchParallel measures concurrent read throughput.
func BenchmarkMemoryIndexSearchParallel(b *testing.B) {
	mi := index.NewMemoryIndex()
	for i := 0; i < 10000; i++ {
		docID := fmt.Sprintf("doc-%d", i)
		mi.AddDocument(docID, "distributed search", "search engine with distributed indexing and query processing")
	}

	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			results := mi.Search("search")
			_ = results
		}
	})
}

// BenchmarkMemoryIndexSnapshot measures the cost of snapshotting the index
// before a segment flush.
func BenchmarkMemoryIndexSnapshot(b *testing.B) {
	mi := index.NewMemoryIndex()
	for i := 0; i < 5000; i++ {
		docID := fmt.Sprintf("doc-%d", i)
		mi.AddDocument(docID, "snapshot benchmark", "testing snapshot performance with multiple terms and documents")
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		snapshot := mi.Snapshot()
		_ = snapshot
	}
}

// BenchmarkEngineIndex measures full engine indexing throughput at various
// pre-loaded corpus sizes.
func BenchmarkEngineIndex(b *testing.B) {
	sizes := []int{100, 1000, 5000}
	for _, preload := range sizes {
		b.Run(fmt.Sprintf("preload_%d", preload), func(b *testing.B) {
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

			for i := 0; i < preload; i++ {
				docID := fmt.Sprintf("preload-%d", i)
				engine.IndexDocument(docID, "preload doc", "preloading documents for benchmark warmup phase")
			}

			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				docID := fmt.Sprintf("bench-%d", i)
				err := engine.IndexDocument(docID, "benchmark title", "benchmark document body for measuring indexing throughput")
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// BenchmarkEngineSearch measures end-to-end search latency across 10 000
// documents.
func BenchmarkEngineSearch(b *testing.B) {
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

	terms := []string{"distributed", "search", "analytics", "platform", "indexing", "query", "engine", "ranking"}
	for i := 0; i < 10000; i++ {
		docID := fmt.Sprintf("doc-%d", i)
		title := fmt.Sprintf("document about %s and %s", terms[i%len(terms)], terms[(i+1)%len(terms)])
		body := fmt.Sprintf("this document covers %s %s %s in production systems",
			terms[i%len(terms)], terms[(i+2)%len(terms)], terms[(i+3)%len(terms)])
		engine.IndexDocument(docID, title, body)
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		results, err := engine.Search(terms[i%len(terms)])
		if err != nil {
			b.Fatal(err)
		}
		_ = results
	}
}

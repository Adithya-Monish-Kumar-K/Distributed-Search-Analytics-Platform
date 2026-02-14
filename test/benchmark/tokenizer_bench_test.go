package benchmark

import (
	"fmt"
	"strings"
	"testing"

	"github.com/Adithya-Monish-Kumar-K/Distributed-Search-Analytics-Platform/internal/indexer/tokenizer"
)

var sampleTexts = map[string]string{
	"short": "The quick brown fox jumps over the lazy dog",
	"medium": `Distributed search engines process queries across multiple shards to achieve
        horizontal scalability. Each shard maintains its own inverted index and responds
        to queries independently. Results are merged using a global ranking algorithm
        that accounts for term frequency and inverse document frequency across the
        entire corpus. This architecture enables sub-second query latency even with
        billions of documents spread across hundreds of nodes.`,
	"long": strings.Repeat(`Information retrieval systems form the backbone of modern search
        infrastructure. These systems combine tokenization, stemming, and stop word
        removal to normalize text into searchable terms. The inverted index maps each
        term to the documents containing it, along with positional information for phrase
        queries. BM25 ranking considers term frequency, document length normalization,
        and inverse document frequency to produce relevance scores. Caching layers reduce
        latency for repeated queries while circuit breakers protect against cascade
        failures in distributed deployments. `, 20),
}

func BenchmarkTokenize(b *testing.B) {
	for name, text := range sampleTexts {
		b.Run(name, func(b *testing.B) {
			b.ReportAllocs()
			b.SetBytes(int64(len(text)))
			for i := 0; i < b.N; i++ {
				tokens := tokenizer.Tokenize(text)
				_ = tokens
			}
		})
	}
}

func BenchmarkTokenizeParallel(b *testing.B) {
	text := sampleTexts["medium"]
	b.ReportAllocs()
	b.SetBytes(int64(len(text)))
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			tokens := tokenizer.Tokenize(text)
			_ = tokens
		}
	})
}

func BenchmarkStemming(b *testing.B) {
	words := []string{
		"running", "distributed", "searching", "indexing",
		"tokenization", "normalization", "efficiently",
		"processing", "infrastructure", "scalability",
	}
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		for _, w := range words {
			tokens := tokenizer.Tokenize(w)
			_ = tokens
		}
	}
}

func BenchmarkTokenizeVaryingSize(b *testing.B) {
	sizes := []int{10, 100, 500, 1000, 5000}
	baseWord := "distributed search analytics platform indexing "
	for _, size := range sizes {
		text := strings.Repeat(baseWord, size/len(baseWord)+1)[:size]
		b.Run(fmt.Sprintf("bytes_%d", size), func(b *testing.B) {
			b.ReportAllocs()
			b.SetBytes(int64(len(text)))
			for i := 0; i < b.N; i++ {
				tokens := tokenizer.Tokenize(text)
				_ = tokens
			}
		})
	}
}

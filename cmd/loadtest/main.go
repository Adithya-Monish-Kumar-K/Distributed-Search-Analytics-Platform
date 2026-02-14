// Command loadtest runs an HTTP load test against the search service.
//
// It sends concurrent search queries for a configurable duration and prints a
// detailed report including request counts, error rate, latency percentiles
// (P50/P90/P95/P99), standard deviation, and status code distribution.
//
// Usage:
//
//	go run ./cmd/loadtest [-url http://localhost:8080] [-concurrency 10] [-duration 30s]
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"os"
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

// Config holds load test parameters parsed from command-line flags.
type Config struct {
	BaseURL     string
	Concurrency int
	Duration    time.Duration
	Queries     []string
}

// Stats collects thread-safe request statistics during the load test run.
// Latencies are appended under a mutex; counters use atomic operations.
type Stats struct {
	totalRequests atomic.Int64
	successCount  atomic.Int64
	errorCount    atomic.Int64
	cacheHits     atomic.Int64
	latencies     []time.Duration
	latenciesMu   sync.Mutex
	statusCodes   map[int]*atomic.Int64
	statusCodesMu sync.Mutex
}

// NewStats creates an empty Stats collector.
func NewStats() *Stats {
	return &Stats{
		latencies:   make([]time.Duration, 0, 100000),
		statusCodes: make(map[int]*atomic.Int64),
	}
}

// RecordRequest records the outcome of a single HTTP request. Errors increment
// the error counter; successful responses record latency and status code.
func (s *Stats) RecordRequest(duration time.Duration, statusCode int, err error) {
	s.totalRequests.Add(1)

	if err != nil {
		s.errorCount.Add(1)
		return
	}

	if statusCode >= 200 && statusCode < 300 {
		s.successCount.Add(1)
	} else {
		s.errorCount.Add(1)
	}

	s.latenciesMu.Lock()
	s.latencies = append(s.latencies, duration)
	s.latenciesMu.Unlock()

	s.statusCodesMu.Lock()
	if _, ok := s.statusCodes[statusCode]; !ok {
		s.statusCodes[statusCode] = &atomic.Int64{}
	}
	s.statusCodes[statusCode].Add(1)
	s.statusCodesMu.Unlock()
}

func main() {
	baseURL := flag.String("url", "http://localhost:8080", "base URL of the search service")
	concurrency := flag.Int("concurrency", 10, "number of concurrent workers")
	duration := flag.Duration("duration", 30*time.Second, "test duration")
	flag.Parse()

	queries := []string{
		"distributed systems",
		"search engine",
		"analytics platform",
		"indexing documents",
		"query processing",
		"cache optimization",
		"ranking algorithm",
		"shard routing",
		"circuit breaker",
		"load balancing",
		"full text search",
		"inverted index",
		"BM25 ranking",
		"token stemming",
		"document ingestion",
	}

	cfg := Config{
		BaseURL:     *baseURL,
		Concurrency: *concurrency,
		Duration:    *duration,
		Queries:     queries,
	}

	fmt.Println("=== Search Platform Load Test ===")
	fmt.Printf("Target:      %s\n", cfg.BaseURL)
	fmt.Printf("Concurrency: %d\n", cfg.Concurrency)
	fmt.Printf("Duration:    %s\n", cfg.Duration)
	fmt.Printf("Queries:     %d unique\n", len(cfg.Queries))
	fmt.Println()

	stats := runLoadTest(cfg)
	printReport(stats, cfg.Duration)
}

// runLoadTest spawns cfg.Concurrency workers that send search requests in a
// loop for cfg.Duration, then returns the collected stats.
func runLoadTest(cfg Config) *Stats {
	stats := NewStats()
	client := &http.Client{
		Timeout: 10 * time.Second,
		Transport: &http.Transport{
			MaxIdleConns:        cfg.Concurrency * 2,
			MaxIdleConnsPerHost: cfg.Concurrency * 2,
			IdleConnTimeout:     90 * time.Second,
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), cfg.Duration)
	defer cancel()

	var wg sync.WaitGroup
	fmt.Print("Running")

	for w := 0; w < cfg.Concurrency; w++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			queryIdx := workerID

			for {
				select {
				case <-ctx.Done():
					return
				default:
				}

				query := cfg.Queries[queryIdx%len(cfg.Queries)]
				queryIdx++

				searchURL := fmt.Sprintf("%s/api/v1/search?q=%s&limit=10",
					cfg.BaseURL, url.QueryEscape(query))

				start := time.Now()
				resp, err := client.Do(mustNewRequest(ctx, searchURL))
				duration := time.Since(start)

				if err != nil {
					stats.RecordRequest(duration, 0, err)
					continue
				}
				io.Copy(io.Discard, resp.Body)
				resp.Body.Close()

				stats.RecordRequest(duration, resp.StatusCode, nil)
			}
		}(w)
	}

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				fmt.Print(".")
			}
		}
	}()

	wg.Wait()
	fmt.Println(" done!")
	fmt.Println()
	return stats
}

// mustNewRequest creates an HTTP GET request or panics. Used inside the hot
// loop where a malformed URL is a programming error, not a runtime condition.
func mustNewRequest(ctx context.Context, rawURL string) *http.Request {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		panic(fmt.Sprintf("creating request: %v", err))
	}
	return req
}

// printReport formats and prints the load test results to stdout.
func printReport(stats *Stats, duration time.Duration) {
	total := stats.totalRequests.Load()
	success := stats.successCount.Load()
	errors := stats.errorCount.Load()

	fmt.Println("=== Results ===")
	fmt.Printf("Total Requests:  %d\n", total)
	fmt.Printf("Successful:      %d\n", success)
	fmt.Printf("Errors:          %d\n", errors)

	if total > 0 {
		errorRate := float64(errors) / float64(total) * 100
		fmt.Printf("Error Rate:      %.2f%%\n", errorRate)
		rps := float64(total) / duration.Seconds()
		fmt.Printf("Requests/sec:    %.2f\n", rps)
	}

	stats.latenciesMu.Lock()
	latencies := make([]time.Duration, len(stats.latencies))
	copy(latencies, stats.latencies)
	stats.latenciesMu.Unlock()

	if len(latencies) > 0 {
		sort.Slice(latencies, func(i, j int) bool {
			return latencies[i] < latencies[j]
		})

		var sum time.Duration
		for _, l := range latencies {
			sum += l
		}
		avg := sum / time.Duration(len(latencies))

		fmt.Println()
		fmt.Println("=== Latency ===")
		fmt.Printf("Min:    %s\n", latencies[0])
		fmt.Printf("Avg:    %s\n", avg)
		fmt.Printf("P50:    %s\n", percentile(latencies, 50))
		fmt.Printf("P90:    %s\n", percentile(latencies, 90))
		fmt.Printf("P95:    %s\n", percentile(latencies, 95))
		fmt.Printf("P99:    %s\n", percentile(latencies, 99))
		fmt.Printf("Max:    %s\n", latencies[len(latencies)-1])

		var sumSquared float64
		avgFloat := float64(avg)
		for _, l := range latencies {
			diff := float64(l) - avgFloat
			sumSquared += diff * diff
		}
		stddev := time.Duration(math.Sqrt(sumSquared / float64(len(latencies))))
		fmt.Printf("StdDev: %s\n", stddev)
	}

	fmt.Println()
	fmt.Println("=== Status Codes ===")
	stats.statusCodesMu.Lock()
	codes := make([]int, 0, len(stats.statusCodes))
	for code := range stats.statusCodes {
		codes = append(codes, code)
	}
	sort.Ints(codes)
	for _, code := range codes {
		count := stats.statusCodes[code].Load()
		fmt.Printf("  %d: %d\n", code, count)
	}
	stats.statusCodesMu.Unlock()

	if total == 0 {
		fmt.Println()
		fmt.Println("WARNING: No requests completed. Is the service running?")
		os.Exit(1)
	}
}

// percentile returns the p-th percentile value from a pre-sorted slice of
// durations. p should be in the range [0, 100].
func percentile(sorted []time.Duration, p float64) time.Duration {
	if len(sorted) == 0 {
		return 0
	}
	idx := int(math.Ceil(p/100*float64(len(sorted)))) - 1
	if idx < 0 {
		idx = 0
	}
	if idx >= len(sorted) {
		idx = len(sorted) - 1
	}
	return sorted[idx]
}

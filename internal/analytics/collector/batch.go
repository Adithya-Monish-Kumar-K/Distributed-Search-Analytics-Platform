// Package collector provides a batch-oriented analytics event collector
// that accumulates events in memory and flushes them to Kafka in bulk.
package collector

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/Adithya-Monish-Kumar-K/Distributed-Search-Analytics-Platform/pkg/kafka"
)

// BatchCollector accumulates analytics events and flushes them to Kafka
// either when the batch reaches a configurable size or after a time interval.
type BatchCollector struct {
	producer      *kafka.Producer
	mu            sync.Mutex
	buffer        []kafka.Event
	batchSize     int
	flushInterval time.Duration
	logger        *slog.Logger
	done          chan struct{}
}

// NewBatchCollector creates a BatchCollector that flushes when the buffer
// reaches batchSize events or after flushInterval, whichever comes first.
func NewBatchCollector(producer *kafka.Producer, batchSize int, flushInterval time.Duration) *BatchCollector {
	if batchSize <= 0 {
		batchSize = 100
	}
	if flushInterval <= 0 {
		flushInterval = 5 * time.Second
	}
	return &BatchCollector{
		producer:      producer,
		buffer:        make([]kafka.Event, 0, batchSize),
		batchSize:     batchSize,
		flushInterval: flushInterval,
		logger:        slog.Default().With("component", "batch-collector"),
		done:          make(chan struct{}),
	}
}

// Start launches the background flush loop. It blocks until ctx is cancelled.
func (bc *BatchCollector) Start(ctx context.Context) {
	go func() {
		defer close(bc.done)
		ticker := time.NewTicker(bc.flushInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				bc.flush(ctx)
			case <-ctx.Done():
				// Final flush with a short deadline.
				flushCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				bc.flush(flushCtx)
				cancel()
				return
			}
		}
	}()
	bc.logger.Info("batch collector started",
		"batch_size", bc.batchSize,
		"flush_interval", bc.flushInterval,
	)
}

// Track adds an event to the buffer. If the buffer reaches batchSize,
// an immediate flush is triggered.
func (bc *BatchCollector) Track(key string, value any) {
	bc.mu.Lock()
	bc.buffer = append(bc.buffer, kafka.Event{Key: key, Value: value})
	shouldFlush := len(bc.buffer) >= bc.batchSize
	bc.mu.Unlock()

	if shouldFlush {
		// Flush in-band (best-effort; doesn't block the caller if another
		// flush is already in progress thanks to the mutex).
		go bc.flush(context.Background())
	}
}

// Close waits for the background flush loop to finish.
func (bc *BatchCollector) Close() {
	<-bc.done
}

// BufferLen returns the current number of buffered events.
func (bc *BatchCollector) BufferLen() int {
	bc.mu.Lock()
	defer bc.mu.Unlock()
	return len(bc.buffer)
}

func (bc *BatchCollector) flush(ctx context.Context) {
	bc.mu.Lock()
	if len(bc.buffer) == 0 {
		bc.mu.Unlock()
		return
	}
	batch := bc.buffer
	bc.buffer = make([]kafka.Event, 0, bc.batchSize)
	bc.mu.Unlock()

	if err := bc.producer.PublishBatch(ctx, batch); err != nil {
		bc.logger.Error("batch flush failed",
			"batch_size", len(batch),
			"error", err,
		)
		// Re-queue failed events (best-effort, may drop on repeated failure).
		bc.mu.Lock()
		bc.buffer = append(batch, bc.buffer...)
		if len(bc.buffer) > bc.batchSize*3 {
			dropped := len(bc.buffer) - bc.batchSize*3
			bc.buffer = bc.buffer[:bc.batchSize*3]
			bc.logger.Warn("buffer overflow, events dropped", "dropped", dropped)
		}
		bc.mu.Unlock()
		return
	}

	bc.logger.Debug("batch flushed", "events", len(batch))
}

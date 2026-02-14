package analytics

import (
	"context"
	"log/slog"

	"github.com/Adithya-Monish-Kumar-K/Distributed-Search-Analytics-Platform/pkg/kafka"
)

// Collector buffers analytics events in-memory and publishes them to Kafka
// asynchronously. If the internal channel fills up, events are dropped with
// a warning log rather than blocking the caller.
type Collector struct {
	producer *kafka.Producer
	eventCh  chan interface{}
	logger   *slog.Logger
	done     chan struct{}
}

// NewCollector creates a Collector with the given Kafka producer and channel
// buffer size. If bufferSize <= 0 it defaults to 10 000.
func NewCollector(producer *kafka.Producer, bufferSize int) *Collector {
	if bufferSize <= 0 {
		bufferSize = 10000
	}
	c := &Collector{
		producer: producer,
		eventCh:  make(chan interface{}, bufferSize),
		logger:   slog.Default().With("component", "analytics-collector"),
		done:     make(chan struct{}),
	}

	return c
}

// Start begins the background goroutine that reads events from the channel
// and publishes them to Kafka. It stops when ctx is cancelled, draining any
// remaining events before returning.
func (c *Collector) Start(ctx context.Context) {
	go func() {
		defer close(c.done)
		for {
			select {
			case event, ok := <-c.eventCh:
				if !ok {
					return
				}
				if err := c.producer.Publish(ctx, kafka.Event{
					Key:   "analytics",
					Value: event,
				}); err != nil {
					c.logger.Error("failed to publish analytics event", "error", err)

				}
			case <-ctx.Done():
				c.drainRemaining()
				return
			}
		}
	}()
	c.logger.Info("analytics collector started", "buffer_size", cap(c.eventCh))
}

// Track enqueues an analytics event for asynchronous publishing. It is
// non-blocking: if the internal buffer is full the event is silently dropped.
func (c *Collector) Track(event interface{}) {
	select {
	case c.eventCh <- event:
	default:
		c.logger.Warn("analytics event dropped (buffer full)")
	}
}

// Close shuts down the collector by closing the event channel and waiting for
// the background goroutine to finish draining.
func (c *Collector) Close() {
	close(c.eventCh)
	<-c.done
}

// drainRemaining publishes any events left in the channel before shutdown.
func (c *Collector) drainRemaining() {
	for {
		select {
		case event, ok := <-c.eventCh:
			if !ok {
				return
			}
			ctx := context.Background()
			if err := c.producer.Publish(ctx, kafka.Event{
				Key:   "analytics",
				Value: event,
			}); err != nil {
				c.logger.Error("failed to publish remaining event", "error", err)
			}
		default:
			return
		}
	}
}

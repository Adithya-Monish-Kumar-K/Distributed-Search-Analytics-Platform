package analytics

import (
	"context"
	"log/slog"

	"github.com/Adithya-Monish-Kumar-K/Distributed-Search-Analytics-Platform/pkg/kafka"
)

type Collector struct {
	producer *kafka.Producer
	eventCh  chan interface{}
	logger   *slog.Logger
	done     chan struct{}
}

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

func (c *Collector) Track(event interface{}) {
	select {
	case c.eventCh <- event:
	default:
		c.logger.Warn("analytics event dropped (buffer full)")
	}
}

func (c *Collector) Close() {
	close(c.eventCh)
	<-c.done
}

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

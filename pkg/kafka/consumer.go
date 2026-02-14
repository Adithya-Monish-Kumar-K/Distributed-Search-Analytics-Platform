// Package kafka provides Kafka producer and consumer clients backed by
// segmentio/kafka-go. The producer serialises events as JSON, while the
// consumer decodes them via a pluggable MessageHandler callback.
package kafka

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/Adithya-Monish-Kumar-K/Distributed-Search-Analytics-Platform/pkg/config"
	"github.com/segmentio/kafka-go"
)

// MessageHandler is a callback invoked for each Kafka message.
type MessageHandler func(ctx context.Context, key []byte, value []byte) error

// Consumer reads messages from a Kafka topic and dispatches them to a
// MessageHandler.
type Consumer struct {
	reader  *kafka.Reader
	logger  *slog.Logger
	handler MessageHandler
}

// NewConsumer creates a Consumer for the given topic and handler.
func NewConsumer(cfg config.KafkaConfig, topic string, handler MessageHandler) *Consumer {
	r := kafka.NewReader(kafka.ReaderConfig{
		Brokers:     cfg.Brokers,
		Topic:       topic,
		GroupID:     cfg.ConsumerGroup,
		MinBytes:    1e3,
		MaxBytes:    10e6,
		StartOffset: kafka.LastOffset,
	})

	return &Consumer{
		reader:  r,
		logger:  slog.Default().With("component", "kafka-consumer", "topic", topic),
		handler: handler,
	}
}

// Start enters the consume loop, fetching and processing messages until ctx
// is cancelled.
func (c *Consumer) Start(ctx context.Context) error {
	c.logger.Info("consumer started")
	for {
		select {
		case <-ctx.Done():
			c.logger.Info("consumer stopping", "reason", ctx.Err())
			return c.reader.Close()
		default:
		}

		msg, err := c.reader.FetchMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			c.logger.Error("failed to fetch message", "error", err)
			continue
		}
		c.logger.Debug("message received",
			"partition", msg.Partition,
			"offset", msg.Offset,
			"key", string(msg.Key),
			"value_size", len(msg.Value),
		)
		if err := c.handler(ctx, msg.Key, msg.Value); err != nil {
			c.logger.Error("failed to process message",
				"partition", msg.Partition,
				"offset", msg.Offset,
				"error", err,
			)
			continue
		}
		if err := c.reader.CommitMessages(ctx, msg); err != nil {
			c.logger.Error("failed to commit message",
				"partition", msg.Partition,
				"offset", msg.Offset,
				"error", err,
			)
		}
	}
}

// Close closes the underlying Kafka reader.
func (c *Consumer) Close() error {
	return c.reader.Close()
}

// DecodeJSON is a generic helper that unmarshals a Kafka message value into T.
func DecodeJSON[T any](value []byte) (T, error) {
	var result T
	if err := json.Unmarshal(value, &result); err != nil {
		return result, fmt.Errorf("decoding kafka message: %w", err)
	}
	return result, nil
}

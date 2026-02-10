package kafka

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/Adithya-Monish-Kumar-K/searchplatform/pkg/config"
	"github.com/segmentio/kafka-go"
)

type MessageHandler func(ctx context.Context, key []byte, value []byte) error

type Consumer struct {
	reader  *kafka.Reader
	logger  *slog.Logger
	handler MessageHandler
}

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

func (c *Consumer) Close() error {
	return c.reader.Close()
}

func DecodeJSON[T any](value []byte) (T, error) {
	var result T
	if err := json.Unmarshal(value, &result); err != nil {
		return result, fmt.Errorf("decoding kafka message: %w", err)
	}
	return result, nil
}

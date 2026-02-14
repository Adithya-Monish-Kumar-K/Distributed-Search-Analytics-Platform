package kafka

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/Adithya-Monish-Kumar-K/Distributed-Search-Analytics-Platform/pkg/config"
	"github.com/segmentio/kafka-go"
)

// Event is the unit of data published to Kafka. Key is used for partition
// hashing and Value is JSON-serialised.
type Event struct {
	Key   string
	Value any
}

// Producer publishes JSON-encoded events to a Kafka topic.
type Producer struct {
	writer *kafka.Writer
	logger *slog.Logger
}

// NewProducer creates a Producer for the given topic.
func NewProducer(cfg config.KafkaConfig, topic string) *Producer {
	w := &kafka.Writer{
		Addr:         kafka.TCP(cfg.Brokers...),
		Topic:        topic,
		Balancer:     &kafka.Hash{},
		BatchSize:    100,
		BatchTimeout: 10 * time.Millisecond,
		MaxAttempts:  3,
		RequiredAcks: kafka.RequireAll,
		Async:        false,
	}
	return &Producer{
		writer: w,
		logger: slog.Default().With("component", "kafka-producer", "topic", topic),
	}
}

// Publish serialises a single event and writes it to Kafka synchronously.
func (p *Producer) Publish(ctx context.Context, event Event) error {
	value, err := json.Marshal(event.Value)
	if err != nil {
		return fmt.Errorf("marshaling event value: %w", err)
	}
	msg := kafka.Message{
		Key:   []byte(event.Key),
		Value: value,
	}

	if err := p.writer.WriteMessages(ctx, msg); err != nil {
		p.logger.Error("failed to publish message",
			"key", event.Key,
			"error", err,
		)
		return fmt.Errorf("publishing to kafka: %w", err)
	}
	p.logger.Debug("message published",
		"key", event.Key,
		"value_size", len(value),
	)
	return nil
}

// PublishBatch writes multiple events to Kafka in a single write call.
func (p *Producer) PublishBatch(ctx context.Context, events []Event) error {
	messages := make([]kafka.Message, 0, len(events))
	for _, event := range events {
		value, err := json.Marshal(event.Value)
		if err != nil {
			return fmt.Errorf("marshaling event value: %w", err)
		}
		messages = append(messages, kafka.Message{
			Key:   []byte(event.Key),
			Value: value,
		})
	}
	if err := p.writer.WriteMessages(ctx, messages...); err != nil {
		p.logger.Error("failed to publish batch",
			"count", len(messages),
			"error", err,
		)
		return fmt.Errorf("publishing batch to kafka: %w", err)
	}
	p.logger.Debug("batch published", "count", len(messages))
	return nil
}

// Close flushes pending writes and closes the underlying Kafka writer.
func (p *Producer) Close() error {
	return p.writer.Close()
}

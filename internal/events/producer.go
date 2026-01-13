// Package events provides Kafka producer for publishing transaction events.
package events

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/IBM/sarama"
	"github.com/google/uuid"
	"github.com/sony/gobreaker"
	"go.uber.org/zap"
)

// Event interface for all publishable events
type Event interface {
	Key() string
}

// Producer handles publishing events to Kafka
type Producer struct {
	producer       sarama.SyncProducer
	asyncProducer  sarama.AsyncProducer
	topicConfig    TopicConfig
	circuitBreaker *gobreaker.CircuitBreaker
	logger         *zap.Logger

	// Buffer for failed events
	buffer     []bufferedEvent
	bufferMu   sync.Mutex
	bufferSize int

	// Metrics
	publishCount int64
	failureCount int64

	closed bool
	mu     sync.RWMutex
}

// bufferedEvent stores events that failed to publish
type bufferedEvent struct {
	topic   string
	key     string
	value   []byte
	addedAt time.Time
}

// ProducerConfig holds configuration for the Kafka producer
type ProducerConfig struct {
	Brokers          []string
	TopicConfig      TopicConfig
	BufferSize       int
	RequireAcks      int
	EnableIdempotent bool
	MaxRetries       int
	RetryBackoff     time.Duration
}

// NewProducer creates a new Kafka producer
func NewProducer(cfg ProducerConfig, cb *gobreaker.CircuitBreaker, logger *zap.Logger) (*Producer, error) {
	config := sarama.NewConfig()
	config.Producer.Return.Successes = true
	config.Producer.Return.Errors = true
	config.Producer.RequiredAcks = sarama.RequiredAcks(cfg.RequireAcks)
	config.Producer.Idempotent = cfg.EnableIdempotent
	config.Producer.Retry.Max = cfg.MaxRetries
	config.Producer.Retry.Backoff = cfg.RetryBackoff
	config.Net.MaxOpenRequests = 1 // Required for idempotent producer

	// Create sync producer for critical events
	syncProducer, err := sarama.NewSyncProducer(cfg.Brokers, config)
	if err != nil {
		return nil, fmt.Errorf("failed to create sync producer: %w", err)
	}

	// Create async producer for non-critical events
	asyncConfig := sarama.NewConfig()
	asyncConfig.Producer.Return.Errors = true
	asyncConfig.Producer.RequiredAcks = sarama.WaitForLocal

	asyncProducer, err := sarama.NewAsyncProducer(cfg.Brokers, asyncConfig)
	if err != nil {
		syncProducer.Close()
		return nil, fmt.Errorf("failed to create async producer: %w", err)
	}

	p := &Producer{
		producer:       syncProducer,
		asyncProducer:  asyncProducer,
		topicConfig:    cfg.TopicConfig,
		circuitBreaker: cb,
		logger:         logger,
		buffer:         make([]bufferedEvent, 0, cfg.BufferSize),
		bufferSize:     cfg.BufferSize,
	}

	// Start error handler for async producer
	go p.handleAsyncErrors()

	return p, nil
}

// PublishTransactionInitiated publishes a TransactionInitiated event
func (p *Producer) PublishTransactionInitiated(ctx context.Context, event *TransactionInitiatedEvent) error {
	return p.publishSync(ctx, p.topicConfig.TransferInitiatedTopic, event.Key(), event)
}

// PublishTransactionApproved publishes a TransactionApproved event
func (p *Producer) PublishTransactionApproved(ctx context.Context, event *TransactionApprovedEvent) error {
	return p.publishSync(ctx, p.topicConfig.TransferApprovedTopic, event.Key(), event)
}

// PublishTransactionRejected publishes a TransactionRejected event
func (p *Producer) PublishTransactionRejected(ctx context.Context, event *TransactionRejectedEvent) error {
	return p.publishSync(ctx, p.topicConfig.TransferRejectedTopic, event.Key(), event)
}

// PublishTransactionCompleted publishes a TransactionCompleted event
func (p *Producer) PublishTransactionCompleted(ctx context.Context, event *TransactionCompletedEvent) error {
	return p.publishSync(ctx, p.topicConfig.TransferCompletedTopic, event.Key(), event)
}

// PublishTransactionWaitingReview publishes a TransactionWaitingReview event
func (p *Producer) PublishTransactionWaitingReview(ctx context.Context, event *TransactionWaitingReviewEvent) error {
	return p.publishSync(ctx, p.topicConfig.TransferWaitingTopic, event.Key(), event)
}

// PublishTransactionCompensated publishes a TransactionCompensated event
func (p *Producer) PublishTransactionCompensated(ctx context.Context, event *TransactionCompensatedEvent) error {
	return p.publishSync(ctx, p.topicConfig.AuditTopic, event.Key(), event)
}

// publishSync publishes an event synchronously with circuit breaker
func (p *Producer) publishSync(ctx context.Context, topic, key string, event interface{}) error {
	p.mu.RLock()
	if p.closed {
		p.mu.RUnlock()
		return fmt.Errorf("producer is closed")
	}
	p.mu.RUnlock()

	value, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal event: %w", err)
	}

	msg := &sarama.ProducerMessage{
		Topic: topic,
		Key:   sarama.StringEncoder(key),
		Value: sarama.ByteEncoder(value),
		Headers: []sarama.RecordHeader{
			{Key: []byte("correlation_id"), Value: []byte(uuid.New().String())},
			{Key: []byte("timestamp"), Value: []byte(time.Now().UTC().Format(time.RFC3339))},
		},
	}

	// Use circuit breaker for resilience
	_, cbErr := p.circuitBreaker.Execute(func() (interface{}, error) {
		partition, offset, err := p.producer.SendMessage(msg)
		if err != nil {
			return nil, err
		}

		p.logger.Debug("Event published",
			zap.String("topic", topic),
			zap.String("key", key),
			zap.Int32("partition", partition),
			zap.Int64("offset", offset),
		)

		return nil, nil
	})

	if cbErr != nil {
		p.logger.Error("Failed to publish event",
			zap.String("topic", topic),
			zap.String("key", key),
			zap.Error(cbErr),
		)

		// Buffer the event for retry
		p.bufferEvent(topic, key, value)
		p.failureCount++

		return fmt.Errorf("failed to publish event: %w", cbErr)
	}

	p.publishCount++
	return nil
}

// PublishAsync publishes an event asynchronously (fire and forget)
func (p *Producer) PublishAsync(topic, key string, event interface{}) error {
	p.mu.RLock()
	if p.closed {
		p.mu.RUnlock()
		return fmt.Errorf("producer is closed")
	}
	p.mu.RUnlock()

	value, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal event: %w", err)
	}

	msg := &sarama.ProducerMessage{
		Topic: topic,
		Key:   sarama.StringEncoder(key),
		Value: sarama.ByteEncoder(value),
	}

	p.asyncProducer.Input() <- msg
	return nil
}

// bufferEvent stores a failed event for later retry
func (p *Producer) bufferEvent(topic, key string, value []byte) {
	p.bufferMu.Lock()
	defer p.bufferMu.Unlock()

	if len(p.buffer) >= p.bufferSize {
		// Remove oldest event
		p.buffer = p.buffer[1:]
	}

	p.buffer = append(p.buffer, bufferedEvent{
		topic:   topic,
		key:     key,
		value:   value,
		addedAt: time.Now(),
	})
}

// RetryBufferedEvents attempts to republish buffered events
func (p *Producer) RetryBufferedEvents(ctx context.Context) int {
	p.bufferMu.Lock()
	events := make([]bufferedEvent, len(p.buffer))
	copy(events, p.buffer)
	p.buffer = p.buffer[:0]
	p.bufferMu.Unlock()

	retriedCount := 0
	for _, evt := range events {
		msg := &sarama.ProducerMessage{
			Topic: evt.topic,
			Key:   sarama.StringEncoder(evt.key),
			Value: sarama.ByteEncoder(evt.value),
		}

		_, _, err := p.producer.SendMessage(msg)
		if err != nil {
			// Re-buffer the event
			p.bufferEvent(evt.topic, evt.key, evt.value)
		} else {
			retriedCount++
		}
	}

	return retriedCount
}

// handleAsyncErrors logs errors from async producer
func (p *Producer) handleAsyncErrors() {
	for err := range p.asyncProducer.Errors() {
		p.logger.Error("Async publish error",
			zap.String("topic", err.Msg.Topic),
			zap.Error(err.Err),
		)
		p.failureCount++
	}
}

// GetStats returns producer statistics
func (p *Producer) GetStats() ProducerStats {
	p.bufferMu.Lock()
	bufferedCount := len(p.buffer)
	p.bufferMu.Unlock()

	return ProducerStats{
		PublishCount:  p.publishCount,
		FailureCount:  p.failureCount,
		BufferedCount: bufferedCount,
	}
}

// ProducerStats holds producer statistics
type ProducerStats struct {
	PublishCount  int64 `json:"publish_count"`
	FailureCount  int64 `json:"failure_count"`
	BufferedCount int   `json:"buffered_count"`
}

// Close closes the producer
func (p *Producer) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.closed {
		return nil
	}

	p.closed = true

	var errs []error
	if err := p.producer.Close(); err != nil {
		errs = append(errs, err)
	}
	if err := p.asyncProducer.Close(); err != nil {
		errs = append(errs, err)
	}

	if len(errs) > 0 {
		return fmt.Errorf("errors closing producer: %v", errs)
	}

	return nil
}

// Package events provides Kafka consumer for handling fraud analysis events.
package events

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/IBM/sarama"
	"github.com/sony/gobreaker"
	"go.uber.org/zap"
)

// FraudEventHandler handles fraud analysis events
type FraudEventHandler interface {
	HandleFraudAnalysisComplete(ctx context.Context, event *FraudAnalysisCompleteEvent) error
	HandleFraudReviewComplete(ctx context.Context, event *FraudReviewCompleteEvent) error
}

// Consumer handles consuming events from Kafka
type Consumer struct {
	consumerGroup  sarama.ConsumerGroup
	topics         []string
	handler        FraudEventHandler
	circuitBreaker *gobreaker.CircuitBreaker
	logger         *zap.Logger

	ready  chan bool
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// ConsumerConfig holds configuration for the Kafka consumer
type ConsumerConfig struct {
	Brokers           []string
	GroupID           string
	Topics            []string
	InitialOffset     int64
	SessionTimeout    int
	HeartbeatInterval int
}

// NewConsumer creates a new Kafka consumer
func NewConsumer(
	cfg ConsumerConfig,
	handler FraudEventHandler,
	cb *gobreaker.CircuitBreaker,
	logger *zap.Logger,
) (*Consumer, error) {
	config := sarama.NewConfig()
	config.Version = sarama.V3_0_0_0
	config.Consumer.Group.Rebalance.GroupStrategies = []sarama.BalanceStrategy{
		sarama.NewBalanceStrategyRoundRobin(),
	}
	config.Consumer.Offsets.Initial = cfg.InitialOffset
	config.Consumer.Group.Session.Timeout = 10000   // 10 seconds
	config.Consumer.Group.Heartbeat.Interval = 3000 // 3 seconds

	consumerGroup, err := sarama.NewConsumerGroup(cfg.Brokers, cfg.GroupID, config)
	if err != nil {
		return nil, fmt.Errorf("failed to create consumer group: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	c := &Consumer{
		consumerGroup:  consumerGroup,
		topics:         cfg.Topics,
		handler:        handler,
		circuitBreaker: cb,
		logger:         logger,
		ready:          make(chan bool),
		ctx:            ctx,
		cancel:         cancel,
	}

	return c, nil
}

// Start begins consuming messages
func (c *Consumer) Start() error {
	c.wg.Add(1)
	go func() {
		defer c.wg.Done()
		for {
			// Check if context is cancelled
			select {
			case <-c.ctx.Done():
				return
			default:
			}

			// Consume loop
			if err := c.consumerGroup.Consume(c.ctx, c.topics, c); err != nil {
				c.logger.Error("Consumer error", zap.Error(err))
			}

			// Reset ready channel for rebalancing
			c.ready = make(chan bool)
		}
	}()

	// Wait for consumer to be ready
	<-c.ready
	c.logger.Info("Consumer ready", zap.Strings("topics", c.topics))

	return nil
}

// Stop stops the consumer gracefully
func (c *Consumer) Stop() error {
	c.cancel()
	c.wg.Wait()
	return c.consumerGroup.Close()
}

// Setup is run at the beginning of a new consumer group session
func (c *Consumer) Setup(session sarama.ConsumerGroupSession) error {
	c.logger.Info("Consumer session setup",
		zap.Int32("generation", session.GenerationID()),
	)
	close(c.ready)
	return nil
}

// Cleanup is run at the end of a consumer group session
func (c *Consumer) Cleanup(session sarama.ConsumerGroupSession) error {
	c.logger.Info("Consumer session cleanup",
		zap.Int32("generation", session.GenerationID()),
	)
	return nil
}

// ConsumeClaim processes messages from a topic partition
func (c *Consumer) ConsumeClaim(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	for {
		select {
		case msg, ok := <-claim.Messages():
			if !ok {
				return nil
			}

			if err := c.processMessage(session.Context(), msg); err != nil {
				c.logger.Error("Failed to process message",
					zap.String("topic", msg.Topic),
					zap.Int32("partition", msg.Partition),
					zap.Int64("offset", msg.Offset),
					zap.Error(err),
				)
				// Continue processing other messages
			}

			session.MarkMessage(msg, "")

		case <-session.Context().Done():
			return nil
		}
	}
}

// processMessage handles a single Kafka message
func (c *Consumer) processMessage(ctx context.Context, msg *sarama.ConsumerMessage) error {
	c.logger.Debug("Processing message",
		zap.String("topic", msg.Topic),
		zap.Int32("partition", msg.Partition),
		zap.Int64("offset", msg.Offset),
	)

	// Parse the event type from headers or content
	var baseEvent BaseEvent
	if err := json.Unmarshal(msg.Value, &baseEvent); err != nil {
		return fmt.Errorf("failed to unmarshal base event: %w", err)
	}

	// Route to appropriate handler based on event type
	var err error
	switch baseEvent.EventType {
	case EventTypeFraudAnalysisComplete:
		var event FraudAnalysisCompleteEvent
		if err := json.Unmarshal(msg.Value, &event); err != nil {
			return fmt.Errorf("failed to unmarshal FraudAnalysisComplete: %w", err)
		}
		err = c.handler.HandleFraudAnalysisComplete(ctx, &event)

	case EventTypeFraudReviewComplete:
		var event FraudReviewCompleteEvent
		if err := json.Unmarshal(msg.Value, &event); err != nil {
			return fmt.Errorf("failed to unmarshal FraudReviewComplete: %w", err)
		}
		err = c.handler.HandleFraudReviewComplete(ctx, &event)

	default:
		c.logger.Warn("Unknown event type",
			zap.String("event_type", string(baseEvent.EventType)),
		)
		return nil
	}

	if err != nil {
		return fmt.Errorf("handler error: %w", err)
	}

	return nil
}

// IsReady returns true if the consumer is ready
func (c *Consumer) IsReady() bool {
	select {
	case <-c.ready:
		return true
	default:
		return false
	}
}

package events

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	"go.uber.org/zap"

	"stox-gateway/internal/config"
	"stox-gateway/internal/workflow"
)

// EventBus handles message publishing to RabbitMQ
type EventBus struct {
	conn    *amqp.Connection
	channel *amqp.Channel
	config  *config.RabbitMQConfig
	logger  *zap.Logger
}

// WorkflowEvent represents different types of workflow events
type WorkflowEvent struct {
	Type       string                 `json:"type"`
	WorkflowID string                 `json:"workflow_id"`
	StepID     string                 `json:"step_id,omitempty"`
	StepType   workflow.StepType      `json:"step_type,omitempty"`
	Payload    map[string]interface{} `json:"payload"`
	Timestamp  time.Time              `json:"timestamp"`
}

// Event types
const (
	EventWorkflowStart = "workflow.start"
	EventWorkflowNext  = "workflow.next_step"
	EventStepCompleted = "step.completed"
	EventWorkflowDone  = "workflow.completed"
	EventWorkflowFail  = "workflow.failed"
)

// NewEventBus creates a new event bus instance
func NewEventBus(cfg *config.RabbitMQConfig, logger *zap.Logger) (*EventBus, error) {
	logger.Info("Connecting to RabbitMQ", zap.String("url", cfg.URL))

	conn, err := amqp.Dial(cfg.URL)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to RabbitMQ: %w", err)
	}

	ch, err := conn.Channel()
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to create channel: %w", err)
	}

	// Declare the exchange
	err = ch.ExchangeDeclare(
		cfg.Exchange, // name
		"topic",      // type
		true,         // durable
		false,        // auto-deleted
		false,        // internal
		false,        // no-wait
		nil,          // arguments
	)
	if err != nil {
		ch.Close()
		conn.Close()
		return nil, fmt.Errorf("failed to declare exchange: %w", err)
	}

	logger.Info("Connected to RabbitMQ successfully", 
		zap.String("exchange", cfg.Exchange))

	return &EventBus{
		conn:    conn,
		channel: ch,
		config:  cfg,
		logger:  logger,
	}, nil
}

// Close closes the RabbitMQ connection
func (eb *EventBus) Close() error {
	if eb.channel != nil {
		eb.channel.Close()
	}
	if eb.conn != nil {
		return eb.conn.Close()
	}
	return nil
}

// PublishWorkflowStart publishes a workflow start event
func (eb *EventBus) PublishWorkflowStart(ctx context.Context, workflow *workflow.Workflow) error {
	event := WorkflowEvent{
		Type:       EventWorkflowStart,
		WorkflowID: workflow.ID,
		Payload: map[string]interface{}{
			"workflow": workflow,
		},
		Timestamp: time.Now(),
	}

	return eb.publishEvent(ctx, event, "workflow.start")
}

// PublishNextStep publishes a next step event
func (eb *EventBus) PublishNextStep(ctx context.Context, workflow *workflow.Workflow, step *workflow.WorkflowStep) error {
	event := WorkflowEvent{
		Type:       EventWorkflowNext,
		WorkflowID: workflow.ID,
		StepID:     step.ID,
		StepType:   step.Type,
		Payload: map[string]interface{}{
			"workflow": workflow,
			"step":     step,
		},
		Timestamp: time.Now(),
	}

	// Route to specific service queue based on step type
	routingKey := fmt.Sprintf("step.%s", step.Type)
	return eb.publishEvent(ctx, event, routingKey)
}

// PublishStepCompleted publishes a step completed event
func (eb *EventBus) PublishStepCompleted(ctx context.Context, workflow *workflow.Workflow, stepID string, result map[string]interface{}) error {
	event := WorkflowEvent{
		Type:       EventStepCompleted,
		WorkflowID: workflow.ID,
		StepID:     stepID,
		Payload: map[string]interface{}{
			"workflow": workflow,
			"result":   result,
		},
		Timestamp: time.Now(),
	}

	return eb.publishEvent(ctx, event, "step.completed")
}

// PublishWorkflowCompleted publishes a workflow completed event
func (eb *EventBus) PublishWorkflowCompleted(ctx context.Context, workflow *workflow.Workflow) error {
	event := WorkflowEvent{
		Type:       EventWorkflowDone,
		WorkflowID: workflow.ID,
		Payload: map[string]interface{}{
			"workflow": workflow,
		},
		Timestamp: time.Now(),
	}

	return eb.publishEvent(ctx, event, "workflow.completed")
}

// PublishWorkflowFailed publishes a workflow failed event
func (eb *EventBus) PublishWorkflowFailed(ctx context.Context, workflow *workflow.Workflow, errorMsg string) error {
	event := WorkflowEvent{
		Type:       EventWorkflowFail,
		WorkflowID: workflow.ID,
		Payload: map[string]interface{}{
			"workflow":  workflow,
			"error_msg": errorMsg,
		},
		Timestamp: time.Now(),
	}

	return eb.publishEvent(ctx, event, "workflow.failed")
}

// publishEvent publishes an event to RabbitMQ with retry logic
func (eb *EventBus) publishEvent(ctx context.Context, event WorkflowEvent, routingKey string) error {
	body, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal event: %w", err)
	}

	eb.logger.Debug("Publishing event",
		zap.String("type", event.Type),
		zap.String("workflow_id", event.WorkflowID),
		zap.String("routing_key", routingKey),
	)

	// Retry logic
	var lastErr error
	for i := 0; i <= eb.config.MaxRetries; i++ {
		err = eb.channel.PublishWithContext(
			ctx,
			eb.config.Exchange, // exchange
			routingKey,         // routing key
			false,              // mandatory
			false,              // immediate
			amqp.Publishing{
				ContentType:  "application/json",
				Body:         body,
				DeliveryMode: amqp.Persistent, // Make messages persistent
				Timestamp:    event.Timestamp,
				MessageId:    fmt.Sprintf("%s-%s", event.WorkflowID, event.Type),
			})

		if err == nil {
			eb.logger.Info("Event published successfully",
				zap.String("type", event.Type),
				zap.String("workflow_id", event.WorkflowID),
				zap.String("routing_key", routingKey),
			)
			return nil
		}

		lastErr = err
		if i < eb.config.MaxRetries {
			eb.logger.Warn("Failed to publish event, retrying",
				zap.Error(err),
				zap.Int("attempt", i+1),
				zap.Int("max_retries", eb.config.MaxRetries),
			)
			time.Sleep(eb.config.RetryDelay)
		}
	}

	eb.logger.Error("Failed to publish event after all retries",
		zap.Error(lastErr),
		zap.String("type", event.Type),
		zap.String("workflow_id", event.WorkflowID),
	)

	return fmt.Errorf("failed to publish event after %d retries: %w", eb.config.MaxRetries, lastErr)
}

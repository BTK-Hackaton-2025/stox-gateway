package workflow

import (
	"context"
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"
)

// EventPublisher defines the interface for publishing workflow events
type EventPublisher interface {
	PublishWorkflowStart(ctx context.Context, workflow *Workflow) error
	PublishNextStep(ctx context.Context, workflow *Workflow, step *WorkflowStep) error
	PublishStepCompleted(ctx context.Context, workflow *Workflow, stepID string, result map[string]interface{}) error
	PublishWorkflowCompleted(ctx context.Context, workflow *Workflow) error
	PublishWorkflowFailed(ctx context.Context, workflow *Workflow, errorMsg string) error
}

// Manager handles workflow orchestration
type Manager struct {
	workflows map[string]*Workflow
	mutex     sync.RWMutex
	eventBus  EventPublisher
	logger    *zap.Logger
}

// NewManager creates a new workflow manager
func NewManager(eventBus EventPublisher, logger *zap.Logger) *Manager {
	return &Manager{
		workflows: make(map[string]*Workflow),
		eventBus:  eventBus,
		logger:    logger,
	}
}

// CreateWorkflow creates a new workflow and starts it
func (m *Manager) CreateWorkflow(ctx context.Context, productData *ProductUploadRequest) (*Workflow, error) {
	productID := fmt.Sprintf("product_%d", time.Now().UnixNano())
	workflow := NewWorkflow(productID, productData)

	// Store workflow
	m.mutex.Lock()
	m.workflows[workflow.ID] = workflow
	m.mutex.Unlock()

	m.logger.Info("Created new workflow",
		zap.String("workflow_id", workflow.ID),
		zap.String("product_id", productID),
	)

	// Publish workflow start event
	if err := m.eventBus.PublishWorkflowStart(ctx, workflow); err != nil {
		m.logger.Error("Failed to publish workflow start event",
			zap.Error(err),
			zap.String("workflow_id", workflow.ID),
		)
		return nil, fmt.Errorf("failed to start workflow: %w", err)
	}

	// Start the first step
	if err := m.StartNextStep(ctx, workflow.ID); err != nil {
		m.logger.Error("Failed to start first step",
			zap.Error(err),
			zap.String("workflow_id", workflow.ID),
		)
		return workflow, err
	}

	return workflow, nil
}

// GetWorkflow retrieves a workflow by ID
func (m *Manager) GetWorkflow(workflowID string) (*Workflow, error) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	workflow, exists := m.workflows[workflowID]
	if !exists {
		return nil, fmt.Errorf("workflow not found: %s", workflowID)
	}

	return workflow, nil
}

// CompleteStep marks a step as completed and moves to the next step
func (m *Manager) CompleteStep(ctx context.Context, req *StepCompletedRequest) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	workflow, exists := m.workflows[req.WorkflowID]
	if !exists {
		return fmt.Errorf("workflow not found: %s", req.WorkflowID)
	}

	m.logger.Info("Processing step completion",
		zap.String("workflow_id", req.WorkflowID),
		zap.String("step_id", req.StepID),
		zap.String("step_type", string(req.StepType)),
		zap.Bool("success", req.Success),
	)

	if req.Success {
		// Mark step as completed
		if err := workflow.MarkStepCompleted(req.StepID, req.Result); err != nil {
			return fmt.Errorf("failed to mark step completed: %w", err)
		}

		// Publish step completed event
		if err := m.eventBus.PublishStepCompleted(ctx, workflow, req.StepID, req.Result); err != nil {
			m.logger.Error("Failed to publish step completed event", zap.Error(err))
		}

		// Check if workflow is complete
		if workflow.Status == StatusCompleted {
			m.logger.Info("Workflow completed",
				zap.String("workflow_id", req.WorkflowID),
			)

			// Publish workflow completed event
			if err := m.eventBus.PublishWorkflowCompleted(ctx, workflow); err != nil {
				m.logger.Error("Failed to publish workflow completed event", zap.Error(err))
			}

			return nil
		}

		// Start next step
		return m.startNextStepInternal(ctx, workflow)
	} else {
		// Mark step as failed
		if err := workflow.MarkStepFailed(req.StepID, req.ErrorMsg); err != nil {
			return fmt.Errorf("failed to mark step failed: %w", err)
		}

		m.logger.Error("Workflow step failed",
			zap.String("workflow_id", req.WorkflowID),
			zap.String("step_id", req.StepID),
			zap.String("error", req.ErrorMsg),
		)

		// Publish workflow failed event
		if err := m.eventBus.PublishWorkflowFailed(ctx, workflow, req.ErrorMsg); err != nil {
			m.logger.Error("Failed to publish workflow failed event", zap.Error(err))
		}

		return fmt.Errorf("workflow step failed: %s", req.ErrorMsg)
	}
}

// StartNextStep starts the next step in the workflow
func (m *Manager) StartNextStep(ctx context.Context, workflowID string) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	workflow, exists := m.workflows[workflowID]
	if !exists {
		return fmt.Errorf("workflow not found: %s", workflowID)
	}

	return m.startNextStepInternal(ctx, workflow)
}

// startNextStepInternal starts the next step (internal, assumes mutex is held)
func (m *Manager) startNextStepInternal(ctx context.Context, workflow *Workflow) error {
	currentStep := workflow.GetCurrentStep()
	if currentStep == nil {
		return fmt.Errorf("no more steps to execute")
	}

	// Mark step as started
	if err := workflow.StartStep(currentStep.ID); err != nil {
		return fmt.Errorf("failed to start step: %w", err)
	}

	m.logger.Info("Starting workflow step",
		zap.String("workflow_id", workflow.ID),
		zap.String("step_id", currentStep.ID),
		zap.String("step_type", string(currentStep.Type)),
		zap.String("service", currentStep.ServiceName),
	)

	// Publish next step event
	if err := m.eventBus.PublishNextStep(ctx, workflow, currentStep); err != nil {
		m.logger.Error("Failed to publish next step event",
			zap.Error(err),
			zap.String("workflow_id", workflow.ID),
			zap.String("step_id", currentStep.ID),
		)
		return fmt.Errorf("failed to publish next step event: %w", err)
	}

	return nil
}

// ListWorkflows returns all workflows (for debugging/monitoring)
func (m *Manager) ListWorkflows() map[string]*Workflow {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	// Return a copy to avoid concurrent access issues
	result := make(map[string]*Workflow)
	for k, v := range m.workflows {
		result[k] = v
	}

	return result
}

// GetWorkflowStatus returns the status of a workflow
func (m *Manager) GetWorkflowStatus(workflowID string) (*Workflow, error) {
	return m.GetWorkflow(workflowID)
}

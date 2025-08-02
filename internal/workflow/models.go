package workflow

import (
	"time"

	"github.com/google/uuid"
)

// WorkflowStatus represents the current status of a workflow
type WorkflowStatus string

const (
	StatusPending    WorkflowStatus = "pending"
	StatusProcessing WorkflowStatus = "processing"
	StatusCompleted  WorkflowStatus = "completed"
	StatusFailed     WorkflowStatus = "failed"
)

// StepType represents different types of workflow steps
type StepType string

const (
	StepImageProcessing StepType = "image_processing"
	StepAIEnhancement   StepType = "ai_enhancement"
	StepSEOGeneration   StepType = "seo_generation"
)

// StepStatus represents the current status of a workflow step
type StepStatus string

const (
	StepStatusPending    StepStatus = "pending"
	StepStatusProcessing StepStatus = "processing"
	StepStatusCompleted  StepStatus = "completed"
	StepStatusFailed     StepStatus = "failed"
)

// Workflow represents a complete product processing workflow
type Workflow struct {
	ID          string                 `json:"id"`
	ProductID   string                 `json:"product_id"`
	Status      WorkflowStatus         `json:"status"`
	Steps       []WorkflowStep         `json:"steps"`
	CurrentStep int                    `json:"current_step"`
	CreatedAt   time.Time              `json:"created_at"`
	UpdatedAt   time.Time              `json:"updated_at"`
	CompletedAt *time.Time             `json:"completed_at,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
	ErrorMsg    string                 `json:"error_msg,omitempty"`
}

// WorkflowStep represents a single step in the workflow
type WorkflowStep struct {
	ID          string                 `json:"id"`
	Type        StepType               `json:"type"`
	Status      StepStatus             `json:"status"`
	ServiceName string                 `json:"service_name"`
	StartedAt   *time.Time             `json:"started_at,omitempty"`
	CompletedAt *time.Time             `json:"completed_at,omitempty"`
	Result      map[string]interface{} `json:"result,omitempty"`
	ErrorMsg    string                 `json:"error_msg,omitempty"`
}

// ProductUploadRequest represents the initial product upload request
type ProductUploadRequest struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Images      []string               `json:"images"`      // URLs or base64 encoded images
	Category    string                 `json:"category"`
	Price       float64                `json:"price"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// WorkflowResponse represents the response when a workflow is created
type WorkflowResponse struct {
	WorkflowID string         `json:"workflow_id"`
	Status     WorkflowStatus `json:"status"`
	Message    string         `json:"message"`
}

// StepCompletedRequest represents a request from services when they complete a step
type StepCompletedRequest struct {
	WorkflowID  string                 `json:"workflow_id"`
	StepID      string                 `json:"step_id"`
	StepType    StepType               `json:"step_type"`
	Success     bool                   `json:"success"`
	Result      map[string]interface{} `json:"result,omitempty"`
	ErrorMsg    string                 `json:"error_msg,omitempty"`
}

// NewWorkflow creates a new workflow instance
func NewWorkflow(productID string, productData *ProductUploadRequest) *Workflow {
	workflowID := uuid.New().String()
	now := time.Now()

	steps := []WorkflowStep{
		{
			ID:          uuid.New().String(),
			Type:        StepImageProcessing,
			Status:      StepStatusPending,
			ServiceName: "image-service",
		},
		{
			ID:          uuid.New().String(),
			Type:        StepAIEnhancement,
			Status:      StepStatusPending,
			ServiceName: "ai-service",
		},
		{
			ID:          uuid.New().String(),
			Type:        StepSEOGeneration,
			Status:      StepStatusPending,
			ServiceName: "seo-service",
		},
	}

	metadata := map[string]interface{}{
		"product_data": productData,
	}

	return &Workflow{
		ID:          workflowID,
		ProductID:   productID,
		Status:      StatusPending,
		Steps:       steps,
		CurrentStep: 0,
		CreatedAt:   now,
		UpdatedAt:   now,
		Metadata:    metadata,
	}
}

// GetCurrentStep returns the current step in the workflow
func (w *Workflow) GetCurrentStep() *WorkflowStep {
	if w.CurrentStep >= len(w.Steps) {
		return nil
	}
	return &w.Steps[w.CurrentStep]
}

// MarkStepCompleted marks the current step as completed and moves to the next step
func (w *Workflow) MarkStepCompleted(stepID string, result map[string]interface{}) error {
	now := time.Now()
	
	// Find the step by ID
	for i := range w.Steps {
		if w.Steps[i].ID == stepID {
			w.Steps[i].Status = StepStatusCompleted
			w.Steps[i].CompletedAt = &now
			w.Steps[i].Result = result
			w.UpdatedAt = now
			
			// If this is the current step, move to next
			if i == w.CurrentStep {
				w.CurrentStep++
				
				// Check if workflow is complete
				if w.CurrentStep >= len(w.Steps) {
					w.Status = StatusCompleted
					w.CompletedAt = &now
				}
			}
			
			return nil
		}
	}
	
	return nil
}

// MarkStepFailed marks a step as failed
func (w *Workflow) MarkStepFailed(stepID string, errorMsg string) error {
	now := time.Now()
	
	// Find the step by ID
	for i := range w.Steps {
		if w.Steps[i].ID == stepID {
			w.Steps[i].Status = StepStatusFailed
			w.Steps[i].ErrorMsg = errorMsg
			w.UpdatedAt = now
			
			// Mark workflow as failed
			w.Status = StatusFailed
			w.ErrorMsg = errorMsg
			
			return nil
		}
	}
	
	return nil
}

// StartStep marks a step as processing
func (w *Workflow) StartStep(stepID string) error {
	now := time.Now()
	
	// Find the step by ID
	for i := range w.Steps {
		if w.Steps[i].ID == stepID {
			w.Steps[i].Status = StepStatusProcessing
			w.Steps[i].StartedAt = &now
			w.UpdatedAt = now
			
			// Mark workflow as processing
			if w.Status == StatusPending {
				w.Status = StatusProcessing
			}
			
			return nil
		}
	}
	
	return nil
}

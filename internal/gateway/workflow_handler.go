package gateway

import (
	"encoding/json"
	"net/http"

	"go.uber.org/zap"

	"stox-gateway/internal/workflow"
	"stox-gateway/internal/websocket"
)

// WorkflowHandler handles workflow-related HTTP requests
type WorkflowHandler struct {
	workflowManager *workflow.Manager
	wsHub           *websocket.Hub
	logger          *zap.Logger
}

// NewWorkflowHandler creates a new workflow handler
func NewWorkflowHandler(workflowManager *workflow.Manager, wsHub *websocket.Hub, logger *zap.Logger) *WorkflowHandler {
	return &WorkflowHandler{
		workflowManager: workflowManager,
		wsHub:           wsHub,
		logger:          logger,
	}
}

// UploadProduct handles product upload and starts workflow
func (h *WorkflowHandler) UploadProduct(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req workflow.ProductUploadRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.logger.Error("Failed to decode product upload request", zap.Error(err))
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	h.logger.Info("Processing product upload",
		zap.String("product_name", req.Name),
		zap.String("category", req.Category),
		zap.Float64("price", req.Price),
	)

	// Create and start workflow
	wf, err := h.workflowManager.CreateWorkflow(r.Context(), &req)
	if err != nil {
		h.logger.Error("Failed to create workflow", zap.Error(err))
		http.Error(w, "Failed to start workflow", http.StatusInternalServerError)
		return
	}

	// Return 202 Accepted with workflow ID
	response := workflow.WorkflowResponse{
		WorkflowID: wf.ID,
		Status:     wf.Status,
		Message:    "Workflow started successfully",
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	
	if err := json.NewEncoder(w).Encode(response); err != nil {
		h.logger.Error("Failed to encode response", zap.Error(err))
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	h.logger.Info("Workflow started",
		zap.String("workflow_id", wf.ID),
		zap.String("product_name", req.Name),
	)
}

// GetWorkflowStatus returns the current status of a workflow
func (h *WorkflowHandler) GetWorkflowStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	workflowID := r.URL.Query().Get("workflow_id")
	if workflowID == "" {
		http.Error(w, "workflow_id query parameter is required", http.StatusBadRequest)
		return
	}

	wf, err := h.workflowManager.GetWorkflow(workflowID)
	if err != nil {
		h.logger.Error("Failed to get workflow", zap.Error(err), zap.String("workflow_id", workflowID))
		http.Error(w, "Workflow not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(wf); err != nil {
		h.logger.Error("Failed to encode workflow", zap.Error(err))
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
}

// StepCompleted handles step completion notifications from services via gRPC
func (h *WorkflowHandler) StepCompleted(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req workflow.StepCompletedRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.logger.Error("Failed to decode step completed request", zap.Error(err))
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	h.logger.Info("Processing step completion",
		zap.String("workflow_id", req.WorkflowID),
		zap.String("step_id", req.StepID),
		zap.String("step_type", string(req.StepType)),
		zap.Bool("success", req.Success),
	)

	// Process step completion
	if err := h.workflowManager.CompleteStep(r.Context(), &req); err != nil {
		h.logger.Error("Failed to complete step", zap.Error(err))
		http.Error(w, "Failed to process step completion", http.StatusInternalServerError)
		return
	}

	// Get updated workflow for WebSocket notification
	wf, err := h.workflowManager.GetWorkflow(req.WorkflowID)
	if err != nil {
		h.logger.Error("Failed to get workflow after step completion", zap.Error(err))
		// Don't return error here as the step was processed successfully
	} else {
		// Send WebSocket notifications
		if req.Success {
			if wf.Status == workflow.StatusCompleted {
				h.wsHub.NotifyWorkflowCompleted(wf)
			} else {
				h.wsHub.NotifyStepCompleted(wf, req.StepID, req.Result)
			}
		} else {
			h.wsHub.NotifyWorkflowFailed(wf, req.ErrorMsg)
		}
	}

	// Return success response
	response := map[string]interface{}{
		"success": true,
		"message": "Step completion processed successfully",
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		h.logger.Error("Failed to encode response", zap.Error(err))
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
}

// ListWorkflows returns all workflows (for debugging/monitoring)
func (h *WorkflowHandler) ListWorkflows(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	workflows := h.workflowManager.ListWorkflows()

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(workflows); err != nil {
		h.logger.Error("Failed to encode workflows", zap.Error(err))
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
}

// WebSocketConnection handles WebSocket connections for real-time updates
func (h *WorkflowHandler) WebSocketConnection(w http.ResponseWriter, r *http.Request) {
	h.wsHub.ServeWS(w, r)
}

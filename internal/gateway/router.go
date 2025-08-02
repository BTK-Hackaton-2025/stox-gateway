package gateway

import (
	"log"
	"net/http"

	"github.com/gorilla/mux"
)

// Router sets up the HTTP routes
func NewRouter(authHandler *AuthHandler, workflowHandler *WorkflowHandler) *mux.Router {
	// Check for nil handlers to prevent runtime panics
	if authHandler == nil {
		log.Printf("NewRouter: authHandler parameter is nil, cannot set up auth routes")
		return nil
	}
	if workflowHandler == nil {
		log.Printf("NewRouter: workflowHandler parameter is nil, cannot set up workflow routes")
		return nil
	}

	router := mux.NewRouter()

	// API versioning
	api := router.PathPrefix("/api/v1").Subrouter()

	// Auth routes
	auth := api.PathPrefix("/auth").Subrouter()
	auth.HandleFunc("/register", authHandler.Register).Methods("POST")
	auth.HandleFunc("/login", authHandler.Login).Methods("POST")
	auth.HandleFunc("/validate", authHandler.ValidateToken).Methods("POST")
	auth.HandleFunc("/profile", authHandler.GetProfile).Methods("GET")

	// Workflow routes
	workflow := api.PathPrefix("/workflow").Subrouter()
	workflow.HandleFunc("/upload", workflowHandler.UploadProduct).Methods("POST")
	workflow.HandleFunc("/status", workflowHandler.GetWorkflowStatus).Methods("GET")
	workflow.HandleFunc("/step-completed", workflowHandler.StepCompleted).Methods("POST")
	workflow.HandleFunc("/list", workflowHandler.ListWorkflows).Methods("GET")

	// WebSocket endpoint for real-time updates
	router.HandleFunc("/ws", workflowHandler.WebSocketConnection)

	// Health check
	router.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		// Set status header - WriteHeader doesn't return an error but can fail silently
		// if called after writing has begun, so we call it first
		w.WriteHeader(http.StatusOK)

		// Write response body and handle potential errors
		if _, err := w.Write([]byte("OK")); err != nil {
			// Log the error since we can't change the response at this point
			log.Printf("Health check: failed to write response body: %v", err)
			// Note: At this point, the response has already been started with 200 OK
			// so we can't send an error response to the client
		}
	}).Methods("GET")

	return router
}

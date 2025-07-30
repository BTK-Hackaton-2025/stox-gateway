package gateway

import (
	"net/http"

	"github.com/gorilla/mux"
)

// Router sets up the HTTP routes
func NewRouter(authHandler *AuthHandler) *mux.Router {
	router := mux.NewRouter()

	// API versioning
	api := router.PathPrefix("/api/v1").Subrouter()

	// Auth routes
	auth := api.PathPrefix("/auth").Subrouter()
	auth.HandleFunc("/register", authHandler.Register).Methods("POST")
	auth.HandleFunc("/login", authHandler.Login).Methods("POST")
	auth.HandleFunc("/validate", authHandler.ValidateToken).Methods("POST")
	auth.HandleFunc("/profile", authHandler.GetProfile).Methods("GET")

	// Health check
	router.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}).Methods("GET")

	return router
}

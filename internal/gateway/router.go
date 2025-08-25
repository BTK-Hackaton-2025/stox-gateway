package gateway

import (
	"log"
	"net/http"

	"github.com/gorilla/mux"
)

// Router sets up the HTTP routes
func NewRouter(authHandler *AuthHandler, imageHandler *ImageHandler, imageUploadHandler *ImageUploadHandler, chatHandler *ChatHandler, productHandler *ProductHandler, apiKeyMiddleware *APIKeyMiddleware) *mux.Router {
	// Check for nil handlers to prevent runtime panics
	if authHandler == nil {
		log.Printf("NewRouter: authHandler parameter is nil, cannot set up auth routes")
		return nil
	}
	if imageHandler == nil {
		log.Printf("NewRouter: imageHandler parameter is nil, cannot set up image routes")
		return nil
	}
	if imageUploadHandler == nil {
		log.Printf("NewRouter: imageUploadHandler parameter is nil, cannot set up image upload routes")
		return nil
	}
	if chatHandler == nil {
		log.Printf("NewRouter: chatHandler parameter is nil, cannot set up chat routes")
		return nil
	}
	if productHandler == nil {
		log.Printf("NewRouter: productHandler parameter is nil, cannot set up product routes")
		return nil
	}
	if apiKeyMiddleware == nil {
		log.Printf("NewRouter: apiKeyMiddleware parameter is nil, cannot set up external API routes")
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

	// Image processing routes (legacy)
	image := api.PathPrefix("/image").Subrouter()
	image.HandleFunc("/process", imageHandler.ProcessImage).Methods("POST")

	// Image management routes with S3 and CloudFront
	images := api.PathPrefix("/images").Subrouter()
	// Add authentication middleware for all image operations
	images.Use(AuthMiddleware(authHandler.GetAuthClient()))
	images.HandleFunc("/upload", imageUploadHandler.UploadImage).Methods("POST")
	images.HandleFunc("/list", imageUploadHandler.GetUserImages).Methods("GET")
	images.HandleFunc("/delete/{imageId}", imageUploadHandler.DeleteUserImage).Methods("DELETE")

	// Chat and SEO routes
	chat := api.PathPrefix("/chat").Subrouter()
	// Add authentication middleware for chat operations
	chat.Use(AuthMiddleware(authHandler.GetAuthClient()))
	chat.HandleFunc("/message", chatHandler.Chat).Methods("POST")
	chat.HandleFunc("/seo", chatHandler.AnalyzeSEO).Methods("POST")

	// Static files for web interface
	router.PathPrefix("/web/").Handler(http.StripPrefix("/web/", http.FileServer(http.Dir("./web/"))))
	router.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/web/chat.html", http.StatusFound)
	})

	// External API routes (MockECommerce proxy) - requires API key validation
	external := api.PathPrefix("/external").Subrouter()
	external.HandleFunc("/products", apiKeyMiddleware.ValidateJWTAndAPIKey(productHandler.GetProducts)).Methods("GET")
	external.HandleFunc("/products", apiKeyMiddleware.ValidateJWTAndAPIKey(productHandler.CreateProduct)).Methods("POST")
	external.HandleFunc("/products/{id}", apiKeyMiddleware.ValidateJWTAndAPIKey(productHandler.GetProductByID)).Methods("GET")
	external.HandleFunc("/products/{id}", apiKeyMiddleware.ValidateJWTAndAPIKey(productHandler.UpdateProduct)).Methods("PUT")
	external.HandleFunc("/products/{id}", apiKeyMiddleware.ValidateJWTAndAPIKey(productHandler.DeleteProduct)).Methods("DELETE")

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

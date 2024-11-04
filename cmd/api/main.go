package main

import (
	"bytes"
	"context"
	"io"
	"log"
	"net/http"
	"os"
	"pluto-restore-assets/cmd/api/handlers"
	"pluto-restore-assets/pkg/kubernetes"
	"time"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

func main() {
	namespace := os.Getenv("KUBE_NAMESPACE")
	if namespace == "" {
		log.Println("KUBE_NAMESPACE environment variable is not set - using default namespace")
		namespace = "default"
	}

	jobCreator, err := kubernetes.NewJobCreator(namespace)
	if err != nil {
		log.Fatalf("Failed to create job creator: %v", err)
	}

	// Create S3 client
	cfg, err := config.LoadDefaultConfig(context.Background())
	if err != nil {
		log.Fatalf("Failed to load AWS config: %v", err)
	}
	s3Client := s3.NewFromConfig(cfg)

	// Create handlers
	restoreHandler := handlers.NewRestoreHandler(jobCreator, s3Client)

	// Setup routes
	mux := http.NewServeMux()

	// API routes
	mux.HandleFunc("POST /restore", restoreHandler.CreateRestore)
	mux.HandleFunc("POST /stats", restoreHandler.GetStatus)
	mux.HandleFunc("GET /health", healthHandler)

	// Add logging middleware
	handler := LoggingMiddleware(mux)

	server := &http.Server{
		Addr:    ":9000",
		Handler: handler,
	}

	log.Println("Starting server on port 9000...")
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("Server failed to start: %v", err)
	}
}

// LoggingMiddleware logs detailed information about requests and their bodies
func LoggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		log.Printf("Received %s request to %s", r.Method, r.URL.Path)

		if r.Method == http.MethodPost {
			body, err := io.ReadAll(r.Body)
			if err != nil {
				log.Printf("Error reading request body: %v", err)
				http.Error(w, "Unable to read request body", http.StatusBadRequest)
				return
			}
			log.Printf("POST request to %s with body: %s", r.URL.Path, body)

			r.Body = io.NopCloser(bytes.NewBuffer(body))
		}

		next.ServeHTTP(w, r)

		duration := time.Since(start)
		log.Printf("Request to %s processed in %v", r.URL.Path, duration)
	})
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

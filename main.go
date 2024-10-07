package main

import (
	"bytes"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"time"

	"pluto-asset-restore/s3utils"
)

type RequestBody struct {
	ID   int    `json:"id"`
	Path string `json:"path"`
}

// LoggingMiddleware logs detailed information about requests and their bodies
func LoggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Log request method, path, and time of request
		log.Printf("Received %s request to %s", r.Method, r.URL.Path)

		// Log POST request body if applicable
		if r.Method == http.MethodPost {
			body, err := io.ReadAll(r.Body)
			if err != nil {
				log.Printf("Error reading request body: %v", err)
				http.Error(w, "Unable to read request body", http.StatusBadRequest)
				return
			}
			log.Printf("POST request to %s with body: %s", r.URL.Path, body)

			// Restore the body for the next handler in the chain
			r.Body = io.NopCloser(bytes.NewBuffer(body))
		}

		// Call the next handler and measure its execution time
		next.ServeHTTP(w, r)

		// Log the request processing time
		duration := time.Since(start)
		log.Printf("Request to %s processed in %v", r.URL.Path, duration)
	})
}

func ProjectRestoreHandler(w http.ResponseWriter, r *http.Request, bucketName, prefix, manifestLocalPath string) {
	log.Println("ProjectRestoreHandler called")

	log.Printf("Received request to restore project from bucket: %s, prefix: %s", bucketName, prefix)
	prefix = "test_commission/test_project/"
	// Generate CSV manifest

	if err := s3utils.GenerateCSVManifest(bucketName, prefix, manifestLocalPath); err != nil {
		log.Printf("Failed to generate CSV manifest: %v", err)
		http.Error(w, "Failed to generate CSV manifest", http.StatusInternalServerError)
		return
	}

	log.Println("CSV manifest generated successfully")

}

func main() {
	bucketName := "archivehunter-test-media"
	manifestKey := "batch-manifests/manifest.csv"
	manifestLocalPath := "/tmp/manifest.csv"

	log.Println("Starting server on port 9000...")

	mux := http.NewServeMux()

	mux.Handle("/", LoggingMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body RequestBody
		err := json.NewDecoder(r.Body).Decode(&body)
		if err != nil {
			http.Error(w, "Failed to parse request body", http.StatusBadRequest)
			return
		}

		prefix := body.Path
		ProjectRestoreHandler(w, r, bucketName, prefix, manifestLocalPath)
		// upload manifest to s3
		_, err = s3utils.UploadFileToS3(bucketName, manifestKey, manifestLocalPath)
		if err != nil {
			log.Printf("Failed to upload manifest: %v", err)
			http.Error(w, "Failed to upload manifest", http.StatusInternalServerError)
			return
		}

	})))

	// Start the HTTP server
	if err := http.ListenAndServe(":9000", mux); err != nil {
		// Log if the server fails to start or stops unexpectedly
		log.Fatalf("Server failed to start: %v", err)
	}

	// Log successful server shutdown (unlikely in normal execution due to Fatal above)
	log.Println("Server shut down successfully")

	// // Upload manifest to S3
	// etag, err := s3utils.UploadFileToS3(bucketName, manifestKey, manifestLocalPath)
	// if err != nil {
	// 	log.Fatalf("Failed to upload manifest: %v", err)
	// }

	// // Get AWS Account ID programmatically
	// accountID, err := s3utils.GetAWSAccountID()
	// if err != nil {
	// 	log.Fatalf("Failed to get AWS Account ID: %v", err)
	// }

	// // Initiate S3 Batch Operations job
	// jobID, err := s3utils.InitiateS3BatchRestore(accountID, bucketName, manifestKey, etag)
	// if err != nil {
	// 	log.Fatalf("Failed to initiate S3 Batch Operations job: %v", err)
	// }

	// // Monitor job status
	// if err := s3utils.MonitorBatchJob(accountID, jobID); err != nil {
	// 	log.Fatalf("Batch job monitoring failed: %v", err)
	// }

	// log.Println("All tasks completed successfully.")
}

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"pluto-asset-restore/s3utils"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3control"
)

type RequestBody struct {
	ID   int    `json:"id"`
	Path string `json:"path"`
}

type RestoreResponse struct {
	Message string `json:"message"`
	JobID   string `json:"jobID"`
}

type RestoreParams struct {
	BucketName        string
	ManifestKey       string
	ManifestLocalPath string
}

func main() {
	ctx := context.Background()
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		log.Fatalf("Unable to load SDK config: %v", err)
	}

	s3Client := s3.NewFromConfig(cfg)
	s3ControlClient := s3control.NewFromConfig(cfg)

	var (
		bucketName        = "archivehunter-test-media"
		manifestKey       = "batch-manifests/manifest.csv"
		manifestLocalPath = "/tmp/manifest.csv"
	)

	// create POC job in k8s
	jobCreator, err := NewJobCreator("default") // Use appropriate namespace
	if err != nil {
		log.Fatalf("Failed to create job creator: %v", err)
	}

	server := &http.Server{
		Addr: ":9000",
		Handler: LoggingMiddleware(http.HandlerFunc(handleRestore(ctx, s3Client, s3ControlClient, jobCreator, RestoreParams{
			BucketName:        bucketName,
			ManifestKey:       manifestKey,
			ManifestLocalPath: manifestLocalPath,
		}))),
	}

	log.Println("Starting server on port 9000...")
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("Server failed to start: %v", err)
	}

	log.Println("Server shut down successfully")
}

// main function when creating jobs in k8s
// func main() {
// 	jobCreator, err := NewJobCreator("default") // Use appropriate namespace
// 	if err != nil {
// 		log.Fatalf("Failed to create job creator: %v", err)
// 	}

// 	server := &http.Server{
// 		Addr:    ":9000",
// 		Handler: LoggingMiddleware(http.HandlerFunc(handleRestore(jobCreator))),
// 	}

// 	log.Println("Starting server on port 9000...")
// 	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
// 		log.Fatalf("Server failed to start: %v", err)
// 	}

// 	log.Println("Server shut down successfully")
// }

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

func ProjectRestoreHandler(ctx context.Context, s3Client *s3.Client, w http.ResponseWriter, r *http.Request, body RequestBody, params RestoreParams) (err error) {
	log.Println("ProjectRestoreHandler called")

	log.Printf("Received request to restore project from bucket: %s, prefix: %s", params.BucketName, body.Path)

	// For testing purposes, we'll use a hardcoded prefix
	body.Path = "test_commission/test_project/"

	if err := s3utils.GenerateCSVManifest(ctx, s3Client, params.BucketName, body.Path, params.ManifestLocalPath); err != nil {
		return fmt.Errorf("generate CSV manifest: %w", err)
	}

	log.Println("CSV manifest generated successfully")
	return nil
}

// This handler creates a job in k8s
// func handleRestore(jobCreator *JobCreator) http.HandlerFunc {
// 	return func(w http.ResponseWriter, r *http.Request) {
// 		var body RequestBody
// 		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
// 			http.Error(w, "Failed to parse request body", http.StatusBadRequest)
// 			return
// 		}

// 		params := RestoreParams{
// 			BucketName:        "archivehunter-test-media",
// 			ManifestKey:       "batch-manifests/manifest.csv",
// 			ManifestLocalPath: "/tmp/manifest.csv",
// 			Path:              body.Path,
// 		}

// 		if err := jobCreator.CreateRestoreJob(params); err != nil {
// 			http.Error(w, fmt.Sprintf("Failed to create restore job: %v", err), http.StatusInternalServerError)
// 			return
// 		}

// 		w.WriteHeader(http.StatusAccepted)
// 		json.NewEncoder(w).Encode(map[string]string{
// 			"message": "Restore job created",
// 		})
// 	}
// }

func handleRestore(ctx context.Context, s3Client *s3.Client, s3ControlClient *s3control.Client, jobCreator *JobCreator, params RestoreParams) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log.Println("handleRestore function called")
		var body RequestBody
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			log.Printf("Failed to parse request body: %v", err)
			http.Error(w, "Failed to parse request body", http.StatusBadRequest)
			return
		}

		// Set a timeout for the entire handler
		handlerCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()

		// Create a channel to receive errors
		errChan := make(chan error, 1)

		go func() {
			log.Println("Starting goroutine for restore process")
			log.Println("Creating restore job")
			if err := jobCreator.CreateRestoreJob(params); err != nil {
				log.Printf("Failed to create restore job: %v", err)
				errChan <- fmt.Errorf("failed to create restore job: %v", err)
				return
			}
			log.Println("Restore job created successfully")

			if err := ProjectRestoreHandler(handlerCtx, s3Client, w, r, body, params); err != nil {
				log.Printf("ProjectRestoreHandler failed: %v", err)
				errChan <- err
				return
			}
			log.Println("ProjectRestoreHandler completed successfully")

			log.Println("Initiating S3 Batch Restore")
			jobID, err := initiateRestore(handlerCtx, s3Client, s3ControlClient, params)
			if err != nil {
				log.Printf("Failed to initiate S3 Batch Restore: %v", err)
				errChan <- err
				return
			}
			log.Printf("S3 Batch Restore initiated with job ID: %s", jobID)

			sendResponse(w, jobID)

			log.Println("Monitoring restore")
			if err := monitorRestore(handlerCtx, s3Client); err != nil {
				log.Printf("Error monitoring restore: %v", err)
			}

			log.Println("Restore process completed")
			errChan <- nil
		}()

		// Wait for either the handler to complete or the timeout to occur
		select {
		case err := <-errChan:
			if err != nil {
				log.Printf("Restore operation failed: %v", err)
				http.Error(w, fmt.Sprintf("Restore operation failed: %v", err), http.StatusInternalServerError)
			} else {
				log.Println("Restore operation completed successfully")
			}
		case <-handlerCtx.Done():
			log.Println("Restore operation timed out")
			http.Error(w, "Restore operation timed out", http.StatusRequestTimeout)
		}
		log.Println("handleRestore function completed")
	}
}

func initiateRestore(ctx context.Context, s3Client *s3.Client, s3ControlClient *s3control.Client, params RestoreParams) (string, error) {
	if err := uploadManifest(ctx, s3Client, params); err != nil {
		return "", fmt.Errorf("upload manifest: %w", err)
	}

	accountID, manifestETag, err := getRestoreDetails(ctx, s3Client, params)
	if err != nil {
		return "", err
	}

	jobID, err := s3utils.InitiateS3BatchRestore(ctx, *s3ControlClient, accountID, params.BucketName, params.ManifestKey, manifestETag)
	if err != nil {
		return "", fmt.Errorf("initiate S3 Batch Restore: %w", err)
	}

	log.Printf("S3 Batch Restore initiated with job ID: %s", jobID)
	return jobID, nil
}

func uploadManifest(ctx context.Context, s3Client *s3.Client, params RestoreParams) error {
	_, err := s3utils.UploadFileToS3(ctx, s3Client, params.BucketName, params.ManifestKey, params.ManifestLocalPath)
	return err
}

func getRestoreDetails(ctx context.Context, s3Client *s3.Client, params RestoreParams) (string, string, error) {
	accountID, err := s3utils.GetAWSAccountID()
	if err != nil {
		return "", "", fmt.Errorf("get AWS Account ID: %w", err)
	}

	manifestETag, err := s3utils.GetObjectETag(ctx, s3Client, accountID, params.BucketName, params.ManifestKey)
	if err != nil {
		return "", "", fmt.Errorf("get manifest ETag: %w", err)
	}

	return accountID, manifestETag, nil
}

func sendResponse(w http.ResponseWriter, jobID string) {
	response := RestoreResponse{
		Message: "S3 Batch Restore initiated",
		JobID:   jobID,
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

func monitorRestore(ctx context.Context, s3Client *s3.Client) error {
	log.Println("Monitoring S3 Batch Restore job")
	if err := s3utils.MonitorObjectRestoreStatus(ctx, s3Client); err != nil {
		return fmt.Errorf("failed to monitor S3 Batch Restore job: %w", err)
	}
	return nil
}

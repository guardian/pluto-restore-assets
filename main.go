package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"
)

type RequestBody struct {
	ID   int    `json:"id"`
	Path string `json:"path"`
	User string `json:"user"`
}

type RestoreParams struct {
	BucketName            string `json:"bucketName"`
	ManifestKey           string `json:"manifestKey"`
	ManifestLocalPath     string `json:"manifestLocalPath"`
	RoleArn               string `json:"roleArn"`
	AWS_ACCESS_KEY_ID     string `json:"aws_access_key_id"`
	AWS_SECRET_ACCESS_KEY string `json:"aws_secret_access_key"`
	AWS_DEFAULT_REGION    string `json:"aws_default_region"`
	ProjectId             int    `json:"projectId"`
	RestorePath           string `json:"restorePath"`
}

func main() {
	namespace := os.Getenv("KUBE_NAMESPACE")
	if namespace == "" {
		log.Println("KUBE_NAMESPACE environment variable is not set - using default namespace")
		namespace = "default"
	}

	jobCreator, err := NewJobCreator(namespace)
	if err != nil {
		log.Fatalf("Failed to create job creator: %v", err)
	}

	server := &http.Server{
		Addr:    ":9000",
		Handler: LoggingMiddleware(http.HandlerFunc(createRestoreJob(jobCreator))),
	}

	log.Println("Starting server on port 9000...")
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("Server failed to start: %v", err)
	}
}

func createRestoreJob(jobCreator *JobCreator) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body RequestBody
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "Failed to parse request body", http.StatusBadRequest)
			return
		}

		params := RestoreParams{
			BucketName:            "archivehunter-test-media",
			ManifestKey:           fmt.Sprintf("batch-manifests/manifest_%d.csv", body.ID),
			ManifestLocalPath:     "/tmp/manifest.csv",
			RoleArn:               os.Getenv("AWS_ROLE_ARN"),
			AWS_ACCESS_KEY_ID:     os.Getenv("AWS_ACCESS_KEY_ID"),
			AWS_SECRET_ACCESS_KEY: os.Getenv("AWS_SECRET_ACCESS_KEY"),
			AWS_DEFAULT_REGION:    os.Getenv("AWS_DEFAULT_REGION"),
			ProjectId:             body.ID,
			RestorePath:           "test_commission/test_project/", //TESTING ONLY - should be body.Path
		}

		if err := jobCreator.CreateRestoreJob(params); err != nil {
			http.Error(w, fmt.Sprintf("Failed to create restore job: %v", err), http.StatusInternalServerError)
			return
		}
		log.Printf("Restore params: %+v", params)

		w.WriteHeader(http.StatusAccepted)
		json.NewEncoder(w).Encode(map[string]string{
			"message": "Restore job created",
		})
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

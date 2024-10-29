package handlers

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"pluto-restore-assets/internal/types"
	"strings"
	"time"
)

type RestoreHandler struct {
	jobCreator JobCreator
}

func NewRestoreHandler(jobCreator JobCreator) *RestoreHandler {
	return &RestoreHandler{
		jobCreator: jobCreator,
	}
}

func (h *RestoreHandler) CreateRestore(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var body types.RequestBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "Failed to parse request body", http.StatusBadRequest)
		return
	}

	// Validate required fields
	if body.User == "" {
		http.Error(w, "User is required", http.StatusBadRequest)
		return
	}

	if body.Path == "" {
		http.Error(w, "Path is required", http.StatusBadRequest)
		return
	}

	if body.ID == 0 {
		http.Error(w, "ID is required", http.StatusBadRequest)
		return
	}

	if body.RetrievalType == "" {
		http.Error(w, "Retrieval type is required", http.StatusBadRequest)
		return
	}

	parts := strings.Split(body.User, "@")[0]
	user := strings.Replace(parts, ".", "_", 1)

	log.Printf("Received request body: %+v", body)

	params := types.RestoreParams{
		AssetBucketList:       strings.Split(os.Getenv("ASSET_BUCKET_LIST"), ","),
		ManifestBucket:        os.Getenv("MANIFEST_BUCKET"),
		ManifestKey:           fmt.Sprintf("batch-manifests/%d_%v_%s.csv", body.ID, user, time.Now().Format("2006-01-02_15-04-05")),
		ManifestLocalPath:     "/tmp/manifest.csv",
		RoleArn:               os.Getenv("AWS_ROLE_ARN"),
		AWS_ACCESS_KEY_ID:     os.Getenv("AWS_ACCESS_KEY_ID"),
		AWS_SECRET_ACCESS_KEY: os.Getenv("AWS_SECRET_ACCESS_KEY"),
		AWS_DEFAULT_REGION:    os.Getenv("AWS_DEFAULT_REGION"),
		ProjectId:             body.ID,
		User:                  body.User,
		RetrievalType:         body.RetrievalType,
		RestorePath:           GetAWSAssetPath(body.Path),
		BasePath:              os.Getenv("BASE_PATH"),
	}

	if err := h.jobCreator.CreateRestoreJob(params); err != nil {
		http.Error(w, fmt.Sprintf("Failed to create restore job: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(map[string]string{
		"message": "Restore job created",
	})
}

func GetAWSAssetPath(fullPath string) string {
	parts := strings.Split(fullPath, "/Assets/")
	if len(parts) > 1 {
		return parts[1] + "/"
	}
	return fullPath + "/"
}

func (h *RestoreHandler) GetStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	jobID := r.PathValue("id")
	log.Printf("Job ID: %s", jobID)
	if jobID == "" {
		http.Error(w, "Job ID is required", http.StatusBadRequest)
		return
	}

	logs, err := h.jobCreator.GetJobLogs(jobID)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to get job logs: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"jobId": jobID,
		"logs":  logs,
	})
}

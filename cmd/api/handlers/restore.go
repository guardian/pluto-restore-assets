package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"pluto-restore-assets/internal/types"
	"strings"
	"time"

	"pluto-restore-assets/internal/notification"
	"pluto-restore-assets/internal/s3utils"

	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type S3ClientAPI interface {
	ListObjectsV2(ctx context.Context, params *s3.ListObjectsV2Input, optFns ...func(*s3.Options)) (*s3.ListObjectsV2Output, error)
	PutObject(ctx context.Context, params *s3.PutObjectInput, optFns ...func(*s3.Options)) (*s3.PutObjectOutput, error)
	HeadObject(ctx context.Context, params *s3.HeadObjectInput, optFns ...func(*s3.Options)) (*s3.HeadObjectOutput, error)
}

type RestoreHandler struct {
	jobCreator JobCreator
	s3Client   S3ClientAPI
	statsCache map[string]*types.RestoreStats
}

func NewRestoreHandler(jobCreator JobCreator, s3Client S3ClientAPI) *RestoreHandler {
	return &RestoreHandler{
		jobCreator: jobCreator,
		s3Client:   s3Client,
		statsCache: make(map[string]*types.RestoreStats),
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
		BasePath:              GetBasePath(body.Path),
	}

	// Generate manifest first
	stats, err := s3utils.GenerateCSVManifest(r.Context(), h.s3Client, params)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to generate manifest: %v", err), http.StatusInternalServerError)
		return
	}

	// Upload manifest to S3
	_, err = s3utils.UploadFileToS3(r.Context(), h.s3Client, params.ManifestBucket, params.ManifestKey, params.ManifestLocalPath)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to upload manifest: %v", err), http.StatusInternalServerError)
		return
	}

	// Create the restore job asynchronously
	go func() {
		if err := h.jobCreator.CreateRestoreJob(params); err != nil {
			log.Printf("Failed to create restore job: %v", err)
		}
	}()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(types.RestoreResponse{
		Message:   "Restore job created",
		FileCount: int64(stats.FileCount),
		TotalSize: int64(stats.TotalSize),
	})
}

func GetAWSAssetPath(fullPath string) string {
	parts := strings.Split(fullPath, "/Assets/")
	if len(parts) > 1 {
		return parts[1] + "/"
	}
	return fullPath + "/"
}

func GetBasePath(fullPath string) string {
	parts := strings.Split(fullPath, "/Assets/")
	if len(parts) > 1 {
		return parts[0] + "/Assets/"
	}
	return fullPath
}

func (h *RestoreHandler) GetStatus(w http.ResponseWriter, r *http.Request) {
	log.Printf("GetStatus called: Received request to %s", r.URL.Path)

	// Check if the request path is /project-restore/stats
	if !strings.HasSuffix(r.URL.Path, "/project-restore/stats") && r.URL.Path != "/stats" {
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}

	var body types.RequestBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "Failed to parse request body", http.StatusBadRequest)
		return
	}

	params := types.RestoreParams{
		AssetBucketList:       strings.Split(os.Getenv("ASSET_BUCKET_LIST"), ","),
		ManifestBucket:        os.Getenv("MANIFEST_BUCKET"),
		ManifestKey:           fmt.Sprintf("batch-manifests/%d_%v_%s.csv", body.ID, body.User, time.Now().Format("2006-01-02_15-04-05")),
		ManifestLocalPath:     "/tmp/manifest.csv",
		RoleArn:               os.Getenv("AWS_ROLE_ARN"),
		AWS_ACCESS_KEY_ID:     os.Getenv("AWS_ACCESS_KEY_ID"),
		AWS_SECRET_ACCESS_KEY: os.Getenv("AWS_SECRET_ACCESS_KEY"),
		AWS_DEFAULT_REGION:    os.Getenv("AWS_DEFAULT_REGION"),
		ProjectId:             body.ID,
		User:                  body.User,
		RetrievalType:         body.RetrievalType,
		RestorePath:           GetAWSAssetPath(body.Path), //GetAWSAssetPath(body.Path),
		BasePath:              GetBasePath(body.Path),
	}

	stats, err := s3utils.GenerateCSVManifest(r.Context(), h.s3Client, params)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to generate manifest: %v", err), http.StatusInternalServerError)
		return
	}

	standardCost, bulkCost := calculateGlacierRetrievalCosts(float64(stats.FileCount), float64(stats.TotalSize))

	// Cache the stats using project ID as key
	cacheKey := fmt.Sprintf("%d", body.ID)
	h.statsCache[cacheKey] = &types.RestoreStats{
		FileCount:    int64(stats.FileCount),
		TotalSize:    stats.TotalSize,
		StandardCost: standardCost,
		BulkCost:     bulkCost,
		Timestamp:    time.Now(),
	}

	log.Printf("Received request body: %+v", r.Body)
	log.Printf("Total size: %v", stats.TotalSize)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"numberOfFiles":         stats.FileCount,
		"totalSize":             float64(stats.TotalSize) / float64(1024*1024*1024), // Convert to GB
		"standardRetrievalCost": standardCost,
		"bulkRetrievalCost":     bulkCost,
	})
}

const (
	// Restore request costs per 1000 requests
	STANDARD_RESTORE_REQUEST_COST_PER_1000 = 0.03  // $0.03 per 1000 requests
	BULK_RESTORE_REQUEST_COST_PER_1000     = 0.025 // $0.025 per 1000 requests

	// GET request costs per 1000 requests
	GET_REQUEST_COST_PER_1000 = 0.0004 // $0.0004 per 1000 requests

	// Data transfer cost
	DATA_TRANSFER_COST_PER_GB = 0.09 // $0.09 per GB
)

func calculateGlacierRetrievalCosts(numberOfFiles float64, totalDataBytes float64) (float64, float64) {
	// Convert bytes to GB
	totalDataGB := totalDataBytes / (1024 * 1024 * 1024)

	// Standard Retrieval Cost Calculation
	standardRestoreRequestCost := (numberOfFiles * STANDARD_RESTORE_REQUEST_COST_PER_1000) / 1000
	standardGetRequestCost := (numberOfFiles * GET_REQUEST_COST_PER_1000) / 1000
	standardDataTransferCost := totalDataGB * DATA_TRANSFER_COST_PER_GB
	totalStandardCost := standardRestoreRequestCost + standardGetRequestCost + standardDataTransferCost

	// Bulk Retrieval Cost Calculation
	bulkRestoreRequestCost := (numberOfFiles * BULK_RESTORE_REQUEST_COST_PER_1000) / 1000
	bulkGetRequestCost := (numberOfFiles * GET_REQUEST_COST_PER_1000) / 1000
	bulkDataTransferCost := totalDataGB * DATA_TRANSFER_COST_PER_GB
	totalBulkCost := bulkRestoreRequestCost + bulkGetRequestCost + bulkDataTransferCost

	return totalStandardCost, totalBulkCost
}

func (h *RestoreHandler) Notify(w http.ResponseWriter, r *http.Request) {
	log.Printf("Notify called: Received request to %s", r.URL.Path)
	var body types.RequestBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "Failed to parse request body", http.StatusBadRequest)
		return
	}

	cacheKey := fmt.Sprintf("%d", body.ID)
	cachedStats, exists := h.statsCache[cacheKey]
	if !exists {
		http.Error(w, "No stats found. Please call /stats endpoint first", http.StatusBadRequest)
		return
	}

	// Delete stats older than 5 minutes
	if time.Since(cachedStats.Timestamp) > 5*time.Minute {
		delete(h.statsCache, cacheKey)
		http.Error(w, "Stats expired. Please call /stats endpoint again", http.StatusBadRequest)
		return
	}

	emailSender := notification.NewSMTPEmailSender()
	subject := fmt.Sprintf("Asset Restore Stats - Project %d", body.ID)
	emailBody := fmt.Sprintf(
		"Asset Restore Statistics\n"+
			"Project ID: %d\n"+
			"User: %s\n"+
			"Path: %s\n"+
			"Total Files: %d\n"+
			"Total Size: %.2f GB\n"+
			"Standard Retrieval Cost: $%.2f\n"+
			"Bulk Retrieval Cost: $%.2f",
		body.ID, body.User, body.Path,
		cachedStats.FileCount,
		float64(cachedStats.TotalSize)/(1024*1024*1024),
		cachedStats.StandardCost,
		cachedStats.BulkCost)

	if err := emailSender.SendEmail(subject, emailBody); err != nil {
		http.Error(w, fmt.Sprintf("Failed to send notification: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"message": "Notification sent successfully",
	})
}

func (h *RestoreHandler) Permissions(w http.ResponseWriter, r *http.Request) {
	var body struct {
		User string `json:"user"`
	}

	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "Failed to parse request body", http.StatusBadRequest)
		return
	}

	allowedUsers := strings.Split(os.Getenv("ALLOWED_USERS"), ",")
	isAllowed := false

	for _, user := range allowedUsers {
		if strings.TrimSpace(user) == body.User {
			isAllowed = true
			break
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]bool{
		"allowed": isAllowed,
	})
}

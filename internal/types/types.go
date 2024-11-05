package types

import "time"

type RestoreParams struct {
	AssetBucketList       []string `json:"assetBucketList"`
	ManifestKey           string   `json:"manifestKey"`
	ManifestBucket        string   `json:"manifestBucket"`
	ManifestLocalPath     string   `json:"manifestLocalPath"`
	RoleArn               string   `json:"roleArn"`
	AWS_ACCESS_KEY_ID     string   `json:"aws_access_key_id"`
	AWS_SECRET_ACCESS_KEY string   `json:"aws_secret_access_key"`
	AWS_DEFAULT_REGION    string   `json:"aws_default_region"`
	ProjectId             int      `json:"projectId"`
	User                  string   `json:"user"`
	RetrievalType         string   `json:"retrievalType"`
	RestorePath           string   `json:"restorePath"`
	BasePath              string   `json:"basePath"`
}

type RequestBody struct {
	ID            int    `json:"id"`
	Path          string `json:"path"`
	User          string `json:"user"`
	RetrievalType string `json:"retrievalType"`
}

type RestoreResponse struct {
	Message   string `json:"message"`
	JobID     string `json:"jobId"`
	FileCount int64  `json:"fileCount"`
	TotalSize int64  `json:"totalSize"`
}

type RestoreStats struct {
	FileCount    int64
	TotalSize    int64
	StandardCost float64
	BulkCost     float64
	Timestamp    time.Time
}

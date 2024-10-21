package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"pluto-restore-assets/s3utils"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3control"
)

type RestoreParams struct {
	BucketName            string
	ManifestKey           string
	ManifestLocalPath     string
	AWS_ACCESS_KEY_ID     string
	AWS_SECRET_ACCESS_KEY string
	AWS_DEFAULT_REGION    string
	ProjectId             int
	RestorePath           string
	RoleArn               string
}

func main() {
	log.Println("Starting restore worker")

	paramsJSON := os.Getenv("RESTORE_PARAMS")
	var params RestoreParams
	if err := json.Unmarshal([]byte(paramsJSON), &params); err != nil {
		log.Fatalf("Failed to unmarshal restore params: %v", err)
	}
	os.Setenv("AWS_ACCESS_KEY_ID", params.AWS_ACCESS_KEY_ID)
	os.Setenv("AWS_SECRET_ACCESS_KEY", params.AWS_SECRET_ACCESS_KEY)
	os.Setenv("AWS_DEFAULT_REGION", params.AWS_DEFAULT_REGION)

	ctx := context.Background()
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		log.Fatalf("Unable to load SDK config: %v", err)
	}

	s3Client := s3.NewFromConfig(cfg)
	s3ControlClient := s3control.NewFromConfig(cfg)

	if err := handleRestore(ctx, s3Client, s3ControlClient, params); err != nil {
		log.Fatalf("Restore operation failed: %v", err)
	}

	log.Println("Restore worker completed successfully")
}

func handleRestore(ctx context.Context, s3Client *s3.Client, s3ControlClient *s3control.Client, params RestoreParams) error {
	log.Println("handleRestore function called")

	if err := s3utils.GenerateCSVManifest(ctx, s3Client, params.BucketName, params.RestorePath, params.ManifestLocalPath); err != nil {
		return fmt.Errorf("generate CSV manifest: %w", err)
	}

	log.Println("CSV manifest generated successfully")

	jobID, err := initiateRestore(ctx, s3Client, s3ControlClient, params)
	if err != nil {
		return fmt.Errorf("initiate restore: %w", err)
	}

	log.Printf("S3 Batch Restore initiated with job ID: %s", jobID)

	if err := s3utils.MonitorObjectRestoreStatus(ctx, s3Client); err != nil {
		return fmt.Errorf("monitor restore: %w", err)
	}

	log.Println("Restore process completed")
	return nil
}
func initiateRestore(ctx context.Context, s3Client *s3.Client, s3ControlClient *s3control.Client, params RestoreParams) (string, error) {
	if _, err := s3utils.UploadFileToS3(ctx, s3Client, params.BucketName, params.ManifestKey, params.ManifestLocalPath); err != nil {
		return "", fmt.Errorf("upload manifest: %w", err)
	}

	accountID, manifestETag, err := getRestoreDetails(ctx, s3Client, params)
	if err != nil {
		return "", err
	}

	jobID, err := s3utils.InitiateS3BatchRestore(ctx, *s3ControlClient, accountID, params.BucketName, params.ManifestKey, params.RoleArn, manifestETag)
	if err != nil {
		return "", fmt.Errorf("initiate S3 Batch Restore: %w", err)
	}

	return jobID, nil
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

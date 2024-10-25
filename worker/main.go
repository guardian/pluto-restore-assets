package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"pluto-restore-assets/s3utils"
	types "pluto-restore-assets/types"
	"time"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3control"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/cenkalti/backoff/v4"
)

func main() {
	log.Println("Starting restore worker")

	paramsJSON := os.Getenv("RESTORE_PARAMS")
	var params types.RestoreParams
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

func handleRestore(ctx context.Context, s3Client *s3.Client, s3ControlClient *s3control.Client, params types.RestoreParams) error {
	log.Println("handleRestore function called")

	if err := s3utils.GenerateCSVManifest(ctx, s3Client, params); err != nil {
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
func initiateRestore(ctx context.Context, s3Client *s3.Client, s3ControlClient *s3control.Client, params types.RestoreParams) (string, error) {
	if _, err := s3utils.UploadFileToS3(ctx, s3Client, params.ManifestBucket, params.ManifestKey, params.ManifestLocalPath); err != nil {
		return "", fmt.Errorf("upload manifest: %w", err)
	}

	accountID, manifestETag, err := getRestoreDetails(ctx, s3Client, params)
	if err != nil {
		return "", err
	}

	jobID, err := s3utils.InitiateS3BatchRestore(ctx, s3Client, *s3ControlClient, accountID, params, manifestETag)
	if err != nil {
		log.Printf("Failed to initiate S3 Batch Restore: %v", err)
		return "", fmt.Errorf("initiate S3 Batch Restore: %w", err)
	}

	return jobID, nil
}

func getRestoreDetails(ctx context.Context, s3Client *s3.Client, params types.RestoreParams) (string, string, error) {
	accountID, err := s3utils.GetAWSAccountID()
	if err != nil {
		return "", "", fmt.Errorf("get AWS Account ID: %w", err)
	}

	log.Printf("Attempting to get ETag for manifest: s3://%s/%s", params.ManifestBucket, params.ManifestKey)
	var manifestETag string
	retryOperation := func() error {
		var err error
		manifestETag, err = s3utils.GetObjectETag(ctx, s3Client, params)
		if err != nil {
			log.Printf("Error getting manifest ETag (will retry): %v", err)
			return err
		}
		return nil
	}

	backOff := backoff.NewExponentialBackOff()
	backOff.MaxElapsedTime = 1 * time.Minute

	if err := backoff.Retry(retryOperation, backOff); err != nil {
		log.Printf("Failed to get manifest ETag after retries: %v", err)
		// Check if the file exists
		_, err := s3Client.HeadObject(ctx, &s3.HeadObjectInput{
			Bucket: aws.String(params.ManifestBucket),
			Key:    aws.String(params.ManifestKey),
		})
		if err != nil {
			log.Printf("Error checking if manifest file exists: %v", err)
			return "", "", fmt.Errorf("manifest file does not exist or is not accessible: %w", err)
		}
		return "", "", fmt.Errorf("get manifest ETag: %w", err)
	}

	log.Printf("Successfully retrieved ETag for manifest: %s", manifestETag)
	return accountID, manifestETag, nil
}

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"pluto-restore-assets/internal/notification"
	"pluto-restore-assets/internal/s3utils"
	types "pluto-restore-assets/internal/types"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3control"
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
	os.Setenv("SMTP_FROM", params.SMTPFrom)
	os.Setenv("SMTP_HOST", params.SMTPHost)
	os.Setenv("SMTP_PORT", params.SMTPPort)
	os.Setenv("NOTIFICATION_EMAIL", params.NotificationEmail)

	// Verify these are not empty strings
	log.Printf("Setting SMTP settings - Host: %s, Port: %s", params.SMTPHost, params.SMTPPort)
	os.Setenv("SMTP_HOST", params.SMTPHost)
	os.Setenv("SMTP_PORT", params.SMTPPort)
	os.Setenv("SMTP_FROM", params.SMTPFrom)

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

	// Download manifest from S3 first
	if err := downloadManifest(ctx, s3Client, params); err != nil {
		return fmt.Errorf("failed to download manifest: %w", err)
	}

	jobID, err := initiateRestore(ctx, s3Client, s3ControlClient, params)
	if err != nil {
		return fmt.Errorf("initiate restore: %w", err)
	}

	log.Printf("S3 Batch Restore initiated with job ID: %s", jobID)

	if keys, err := s3utils.MonitorObjectRestoreStatus(ctx, s3Client); err != nil {
		return fmt.Errorf("monitor restore: %w", err)
	} else {
		if err := s3utils.DownloadFiles(ctx, s3Client, keys, params.BasePath); err != nil {
			return fmt.Errorf("download files: %w", err)
		}
	}

	log.Println("Restore process completed")
	// Add debug logging for SMTP settings
	log.Printf("SMTP Settings - Host: %s, Port: %s, From: %s, To: %s",
		os.Getenv("SMTP_HOST"),
		os.Getenv("SMTP_PORT"),
		os.Getenv("SMTP_FROM"),
		params.NotificationEmail)

	// Set notification email explicitly since it's in params
	os.Setenv("NOTIFICATION_EMAIL", params.NotificationEmail)

	emailSender := notification.NewSMTPEmailSender(
		params.SMTPHost,
		params.SMTPPort,
		params.SMTPFrom,
		params.NotificationEmail,
	)
	subject := fmt.Sprintf("Asset Restore Completed for Project %d", params.ProjectId)
	emailBody := fmt.Sprintf(
		"Project Asset Restore Completed.\n\n"+
			"User requesting restore: %v\n"+
			"Retrieval Type: %v\n"+
			"Project URL: %v%v",
		params.User,
		params.RetrievalType,
		params.PlutoProjectURL,
		params.ProjectId,
	)

	log.Printf("Attempting to send email using SMTP server: %s:%s", params.SMTPHost, params.SMTPPort)
	err = emailSender.SendEmail(subject, emailBody)

	if err != nil {
		return fmt.Errorf("failed to send notification: %w", err)
	}

	return nil
}

func downloadManifest(ctx context.Context, s3Client *s3.Client, params types.RestoreParams) error {
	result, err := s3Client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(params.ManifestBucket),
		Key:    aws.String(params.ManifestKey),
	})
	if err != nil {
		return fmt.Errorf("failed to get manifest from S3: %w", err)
	}
	defer result.Body.Close()

	file, err := os.Create(params.ManifestLocalPath)
	if err != nil {
		return fmt.Errorf("failed to create local manifest file: %w", err)
	}
	defer file.Close()

	if _, err := io.Copy(file, result.Body); err != nil {
		return fmt.Errorf("failed to write manifest file: %w", err)
	}

	return nil
}

func initiateRestore(ctx context.Context, s3Client *s3.Client, s3ControlClient *s3control.Client, params types.RestoreParams) (string, error) {
	// if _, err := s3utils.UploadFileToS3(ctx, s3Client, params.ManifestBucket, params.ManifestKey, params.ManifestLocalPath); err != nil {
	// 	return "", fmt.Errorf("upload manifest: %w", err)
	// }

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

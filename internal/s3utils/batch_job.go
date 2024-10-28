package s3utils

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	restoreTypes "pluto-restore-assets/internal/types"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3control"
	"github.com/aws/aws-sdk-go-v2/service/s3control/types"
	"github.com/aws/smithy-go"
)

func InitiateS3BatchRestore(ctx context.Context, s3Client *s3.Client, s3ControlClient s3control.Client, accountID string, params restoreTypes.RestoreParams, manifestETag string) (string, error) {
	log.Println("Initiating S3 Batch Operations job...")

	// Get the current ETag of the manifest file
	currentETag, err := getCurrentETag(ctx, s3Client, params.ManifestBucket, params.ManifestKey)
	if err != nil {
		return "", fmt.Errorf("failed to get current ETag: %w", err)
	}
	if params.RetrievalType == "Standard" {
		params.RetrievalType = string(types.S3GlacierJobTierStandard) // Expedited is not supported, fallback to Standard
	} else {
		params.RetrievalType = string(types.S3GlacierJobTierBulk)
	}

	jobInput := &s3control.CreateJobInput{
		AccountId: aws.String(accountID),
		Manifest: &types.JobManifest{
			Spec: &types.JobManifestSpec{
				Format: types.JobManifestFormatS3BatchOperationsCsv20180820,
				Fields: []types.JobManifestFieldName{
					types.JobManifestFieldNameBucket,
					types.JobManifestFieldNameKey,
				},
			},
			Location: &types.JobManifestLocation{
				ObjectArn: aws.String(fmt.Sprintf("arn:aws:s3:::%s/%s", params.ManifestBucket, params.ManifestKey)),
				ETag:      aws.String(currentETag),
			},
		},
		Operation: &types.JobOperation{
			S3InitiateRestoreObject: &types.S3InitiateRestoreObjectOperation{
				ExpirationInDays: aws.Int32(7),
				GlacierJobTier:   types.S3GlacierJobTier(params.RetrievalType), // Can use Bulk for cheaper but slower, Standard for faster but more expensive
			},
		},
		Priority: aws.Int32(10),
		Report: &types.JobReport{
			Enabled:     true,
			Bucket:      aws.String(fmt.Sprintf("arn:aws:s3:::%s", params.ManifestBucket)),
			Prefix:      aws.String("batch-job-reports/"),
			Format:      types.JobReportFormatReportCsv20180820,
			ReportScope: types.JobReportScopeAllTasks,
		},
		RoleArn: aws.String(params.RoleArn),
	}

	result, err := s3ControlClient.CreateJob(context.TODO(), jobInput)
	if err != nil {
		var ae smithy.APIError
		if errors.As(err, &ae) {
			if ae.ErrorCode() == "InvalidManifest" {
				log.Printf("ETag mismatch detected. Attempting to retrieve current ETag.")
				currentETag, err := getCurrentETag(ctx, s3Client, params.ManifestBucket, params.ManifestKey)
				if err != nil {
					return "", fmt.Errorf("failed to get current ETag: %w", err)
				}
				jobInput.Manifest.Location.ETag = aws.String(currentETag)
				result, err = s3ControlClient.CreateJob(context.TODO(), jobInput)
				if err != nil {
					return "", fmt.Errorf("failed to create S3 Batch Operations job with updated ETag: %w", err)
				}
			} else {
				return "", fmt.Errorf("failed to create S3 Batch Operations job: %w", err)
			}
		} else {
			return "", fmt.Errorf("failed to create S3 Batch Operations job: %w", err)
		}
	}

	jobID := aws.ToString(result.JobId)
	log.Printf("S3 Batch Operations job created. Job ID: %s", jobID)
	// Wait for the job to be in a state where we can update it
	err = waitForJobReadyToUpdate(&s3ControlClient, accountID, jobID)
	if err != nil {
		log.Printf("Failed to wait for job to be ready: %v", err)
		return "", fmt.Errorf("failed to wait for job to be ready: %w", err)
	}

	// Now update the job status to Ready
	updateInput := &s3control.UpdateJobStatusInput{
		AccountId:          aws.String(accountID),
		JobId:              aws.String(jobID),
		RequestedJobStatus: types.RequestedJobStatusReady,
	}

	_, err = s3ControlClient.UpdateJobStatus(context.TODO(), updateInput)
	if err != nil {
		log.Printf("Failed to start S3 Batch Operations job: %v", err)
		return "", fmt.Errorf("failed to start S3 Batch Operations job: %w", err)
	}

	log.Printf("S3 Batch Operations job %s has been automatically started", jobID)

	return jobID, nil
}

func waitForJobReadyToUpdate(client *s3control.Client, accountID, jobID string) error {
	maxAttempts := 60
	backoff := time.Second

	for attempt := 0; attempt < maxAttempts; attempt++ {
		describeInput := &s3control.DescribeJobInput{
			AccountId: aws.String(accountID),
			JobId:     aws.String(jobID),
		}

		describeOutput, err := client.DescribeJob(context.TODO(), describeInput)
		if err != nil {
			log.Printf("Failed to describe job: %v", err)
			return fmt.Errorf("failed to describe job: %w", err)
		}

		log.Printf("Job status: %s, Progress: %+v", describeOutput.Job.Status, describeOutput.Job.ProgressSummary)

		if len(describeOutput.Job.FailureReasons) > 0 {
			for i, reason := range describeOutput.Job.FailureReasons {
				log.Printf("Failure Reason %d - Code: %s, Reason: %s", i+1, aws.ToString(reason.FailureCode), aws.ToString(reason.FailureReason))
			}
		}

		if describeOutput.Job.Status == types.JobStatusSuspended {
			return nil // Job is ready to be updated
		}

		if describeOutput.Job.Status == types.JobStatusFailed {
			return fmt.Errorf("job failed: %v", describeOutput.Job.FailureReasons)
		}

		log.Printf("Waiting for job to be ready for update. Attempt %d/%d. Current status: %s",
			attempt+1, maxAttempts, describeOutput.Job.Status)

		time.Sleep(backoff)
		backoff = time.Duration(float64(backoff) * 1.5) // Exponential backoff
		if backoff > 30*time.Second {
			backoff = 30 * time.Second // Cap at 30 seconds
		}
	}

	return fmt.Errorf("job did not reach updateable state within expected time")
}

func getCurrentETag(ctx context.Context, s3Client *s3.Client, bucket, key string) (string, error) {
	headOutput, err := s3Client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return "", fmt.Errorf("failed to get object metadata: %w", err)
	}
	return *headOutput.ETag, nil
}

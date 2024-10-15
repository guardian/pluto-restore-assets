package s3utils

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/google/uuid"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3control"
	"github.com/aws/aws-sdk-go-v2/service/s3control/types"
)

func InitiateS3BatchRestore(ctx context.Context, s3Client s3control.Client, accountID, bucketName, manifestKey, manifestETag string) (string, error) {
	log.Println("Initiating S3 Batch Operations job...")

	clientRequestToken := uuid.New().String()

	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		return "", fmt.Errorf("failed to load AWS config: %w", err)
	}

	client := s3control.NewFromConfig(cfg)
	// get roleArn from env
	roleArn := os.Getenv("AWS_ROLE_ARN")
	log.Println("roleArn", roleArn)

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
				ObjectArn: aws.String(fmt.Sprintf("arn:aws:s3:::%s/%s", bucketName, manifestKey)),
				ETag:      aws.String(manifestETag),
			},
		},
		Operation: &types.JobOperation{
			S3InitiateRestoreObject: &types.S3InitiateRestoreObjectOperation{
				ExpirationInDays: aws.Int32(7),
				GlacierJobTier:   types.S3GlacierJobTierStandard, // Can use Bulk for cheaper but slower, Expedited for faster but more expensive
			},
		},
		Priority: aws.Int32(10),
		Report: &types.JobReport{
			Enabled:     true,
			Bucket:      aws.String(fmt.Sprintf("arn:aws:s3:::%s", bucketName)),
			Prefix:      aws.String("batch-job-reports/"),
			Format:      types.JobReportFormatReportCsv20180820,
			ReportScope: types.JobReportScopeAllTasks,
		},
		RoleArn: aws.String(roleArn),

		ClientRequestToken: aws.String(clientRequestToken),
	}

	result, err := client.CreateJob(context.TODO(), jobInput)
	if err != nil {
		return "", fmt.Errorf("failed to create S3 Batch Operations job: %w", err)
	}

	jobID := aws.ToString(result.JobId)
	log.Printf("S3 Batch Operations job created. Job ID: %s", jobID)

	// Wait for the job to be in a state where we can update it
	err = waitForJobReadyToUpdate(client, accountID, jobID)
	if err != nil {
		return "", fmt.Errorf("failed to wait for job to be ready: %w", err)
	}

	// Now update the job status to Ready
	updateInput := &s3control.UpdateJobStatusInput{
		AccountId:          aws.String(accountID),
		JobId:              aws.String(jobID),
		RequestedJobStatus: types.RequestedJobStatusReady,
	}

	_, err = client.UpdateJobStatus(context.TODO(), updateInput)
	if err != nil {
		return "", fmt.Errorf("failed to start S3 Batch Operations job: %w", err)
	}

	log.Printf("S3 Batch Operations job %s has been automatically started", jobID)

	return jobID, nil
}

func waitForJobReadyToUpdate(client *s3control.Client, accountID, jobID string) error {
	maxAttempts := 10
	for attempt := 0; attempt < maxAttempts; attempt++ {
		describeInput := &s3control.DescribeJobInput{
			AccountId: aws.String(accountID),
			JobId:     aws.String(jobID),
		}

		describeOutput, err := client.DescribeJob(context.TODO(), describeInput)
		if err != nil {
			return fmt.Errorf("failed to describe job: %w", err)
		}

		if describeOutput.Job.Status == types.JobStatusSuspended {
			return nil // Job is ready to be updated
		}

		log.Printf("Waiting for job to be ready for update. Current status: %s", describeOutput.Job.Status)
		time.Sleep(1 * time.Second)
	}

	return fmt.Errorf("job did not reach updateable state within expected time")
}

package s3utils

import (
	"context"
	"fmt"
	"log"

	"github.com/google/uuid"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3control"
	"github.com/aws/aws-sdk-go-v2/service/s3control/types"
)

func InitiateS3BatchRestore(accountID, bucketName, manifestKey, manifestETag string) (string, error) {
	log.Println("Initiating S3 Batch Operations job...")

	cfg, err := config.LoadDefaultConfig(context.TODO(), config.WithRegion("eu-west-1"))
	if err != nil {
		return "", fmt.Errorf("failed to load AWS config: %w", err)
	}

	client := s3control.NewFromConfig(cfg)

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
				GlacierJobTier:   types.S3GlacierJobTierBulk,
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
		RoleArn: aws.String("<your-s3-batch-operations-role-arn>"),
		// Is the line below correct?
		// ClientRequestToken: aws.String("restore-job-token"),
		// TODO: Generate a random UUID and use it as the ClientRequestToken
		// use a random UUID for the ClientRequestToken
		ClientRequestToken: aws.String(uuid.New().String()),

		// ClientRequestToken: aws.String("restore-job-token"),
	}

	result, err := client.CreateJob(context.TODO(), jobInput)
	if err != nil {
		return "", fmt.Errorf("failed to create S3 Batch Operations job: %w", err)
	}

	jobID := aws.ToString(result.JobId)
	log.Printf("S3 Batch Operations job created. Job ID: %s", jobID)
	return jobID, nil
}

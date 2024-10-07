package s3utils

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3control"
	"github.com/aws/aws-sdk-go-v2/service/s3control/types"
)

func MonitorBatchJob(accountID, jobID string) error {
	log.Printf("Monitoring S3 Batch Operations job: %s", jobID)

	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		return fmt.Errorf("failed to load AWS config: %w", err)
	}

	client := s3control.NewFromConfig(cfg)
	// add a start and finish time for the job
	startTime := time.Now()
	log.Printf("Job started at: %s", startTime)
	for {
		resp, err := client.DescribeJob(context.TODO(), &s3control.DescribeJobInput{
			AccountId: aws.String(accountID),
			JobId:     aws.String(jobID),
		})
		if err != nil {
			return fmt.Errorf("failed to describe batch job: %w", err)
		}

		status := resp.Job.Status
		log.Printf("Job status: %s", status)

		if status == types.JobStatusComplete {
			log.Printf("Batch job completed successfully at %v", time.Now())
			break
		} else if status == types.JobStatusFailed || status == types.JobStatusCancelled {
			return fmt.Errorf("batch job failed or was cancelled")
		}

		time.Sleep(15 * time.Minute)
		log.Printf("Time elapsed: %s", time.Since(startTime))
	}

	return nil
}

package s3utils

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"math/rand"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/cenkalti/backoff/v4"
)

// func MonitorBatchJob(accountID, jobID, bucketName string, objectKeys []string) error {
// 	log.Printf("Monitoring restore status for objects in bucket: %s", bucketName)

// 	cfg, err := config.LoadDefaultConfig(context.TODO(), config.WithRegion("eu-west-1"))
// 	if err != nil {
// 		return fmt.Errorf("failed to load AWS config: %w", err)
// 	}

// 	s3Client := s3.NewFromConfig(cfg)

// 	startTime := time.Now()
// 	log.Printf("Monitoring started at: %s", startTime)

// 	return monitorObjectRestoreStatus(s3Client, bucketName, objectKeys)
// }

func MonitorObjectRestoreStatus() error {
	cfg, err := config.LoadDefaultConfig(context.TODO(), config.WithRegion("eu-west-1"))
	ctx := context.Background()
	if err != nil {
		return err
	}

	client := s3.NewFromConfig(cfg)
	keys, err := readManifestFile("/tmp/manifest.csv")
	if err != nil {
		return fmt.Errorf("failed to read manifest file: %v", err)
	}

	log.Printf("Monitoring %d objects", len(keys))
	remainingKeys := keys
	for len(remainingKeys) > 0 {
		var stillRestoring []S3Entry
		for _, key := range remainingKeys {
			restored, err := checkRestoreStatus(ctx, client, key.Bucket, key.Key)
			if err != nil {
				log.Printf("Error checking restore status for %s/%s: %v", key.Bucket, key.Key, err)
				stillRestoring = append(stillRestoring, key)
				continue
			}
			if !restored {
				stillRestoring = append(stillRestoring, key)
			} else {
				log.Printf("Object %s/%s has been restored", key.Bucket, key.Key)
			}
		}

		if len(stillRestoring) == 0 {
			log.Println("All objects restored successfully")
			if err := DownloadFiles(ctx, client, keys); err != nil {
				return fmt.Errorf("failed to download files: %w", err)
			}
			return nil
		}

		remainingKeys = stillRestoring
		sleepDuration := time.Duration(15+rand.Intn(30)) * time.Minute
		log.Printf("%d objects still restoring. Waiting %v before next check...", len(remainingKeys), sleepDuration)
		time.Sleep(sleepDuration)
	}
	return nil
}

func DownloadFiles(ctx context.Context, client *s3.Client, keys []S3Entry) error {
	log.Printf("Downloading %d files", len(keys))
	log.Printf("Files: %v", keys)
	// for _, key := range keys {
	// 	downloadPath := fmt.Sprintf("/tmp/%s", key.Key)
	// 	err := downloadFile(ctx, client, key.Bucket, key.Key, downloadPath)
	// 	if err != nil {
	// 		return fmt.Errorf("failed to download file %s/%s: %w", key.Bucket, key.Key, err)
	// 	}
	return nil
}

type S3Entry struct {
	Bucket string
	Key    string
}

func readManifestFile(filepath string) ([]S3Entry, error) {
	file, err := os.Open(filepath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var entries []S3Entry
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		parts := strings.Split(scanner.Text(), ",")
		if len(parts) == 2 {
			entries = append(entries, S3Entry{Bucket: parts[0], Key: parts[1]})
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return entries, nil
}

func checkRestoreStatus(ctx context.Context, client *s3.Client, bucket, key string) (bool, error) {
	operation := func() (bool, error) {
		resp, err := client.HeadObject(ctx, &s3.HeadObjectInput{
			Bucket: aws.String(bucket),
			Key:    aws.String(key),
		})
		if err != nil {
			return false, err
		}

		if resp.Restore == nil {
			return false, nil
		}

		return !strings.Contains(*resp.Restore, "ongoing-request=\"true\""), nil
	}

	b := backoff.NewExponentialBackOff()
	b.InitialInterval = 1 * time.Second
	b.MaxInterval = 30 * time.Second
	b.MaxElapsedTime = 5 * time.Minute
	b.RandomizationFactor = 0.5

	var restored bool
	err := backoff.RetryNotify(func() error {
		var opErr error
		restored, opErr = operation()
		return opErr
	}, b, func(err error, d time.Duration) {
		log.Printf("Retrying after error: %v", err)
	})

	return restored, err
}

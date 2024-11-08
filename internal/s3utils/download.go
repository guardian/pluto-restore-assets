package s3utils

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

func DownloadFiles(ctx context.Context, client *s3.Client, keys []S3Entry, basePath string) error {
	log.Printf("Downloading %d files", len(keys))
	log.Printf("Base path: %s", basePath)

	// Create a worker pool
	workerCount := 10 // Adjust based on system capabilities
	jobs := make(chan S3Entry, len(keys))
	results := make(chan error, len(keys))

	// Start worker pool
	for w := 1; w <= workerCount; w++ {
		go worker(ctx, client, basePath, jobs, results)
	}

	// Send jobs to workers
	for _, key := range keys {
		jobs <- key
	}
	close(jobs)

	// Collect results
	for i := 0; i < len(keys); i++ {
		if err := <-results; err != nil {
			log.Printf("Error downloading file: %v", err)
		}
	}

	return nil
}

func worker(ctx context.Context, client *s3.Client, basePath string, jobs <-chan S3Entry, results chan<- error) {
	for job := range jobs {
		results <- downloadFile(ctx, client, job.Bucket, job.Key, basePath)
	}
}

func downloadFile(ctx context.Context, client *s3.Client, bucket, key, basePath string) error {
	fullPath := filepath.Join(basePath, key)
	dir := filepath.Dir(fullPath)

	log.Printf("ctx: %v", ctx)
	log.Printf("client: %v", client)

	log.Printf("Downloading %s/%s to %s", bucket, key, fullPath)
	log.Printf("Directory: %s", dir)

	// Create directory if it doesn't exist, ignore if it does
	if err := os.MkdirAll(dir, 0755); err != nil && !os.IsExist(err) {
		return fmt.Errorf("failed to create directory %s: %w", dir, err)
	}

	filename := filepath.Base(fullPath)
	ext := filepath.Ext(filename)
	name := strings.TrimSuffix(filename, ext)

	finalPath := fullPath
	counter := 1
	for {
		if _, err := os.Stat(finalPath); os.IsNotExist(err) {
			break
		}
		finalPath = filepath.Join(dir, fmt.Sprintf("%s_%d%s", name, counter, ext))
		counter++
	}

	// Create and download file
	file, err := os.OpenFile(finalPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to create file %s: %w", finalPath, err)
	}
	defer file.Close()

	downloader := manager.NewDownloader(client)
	_, err = downloader.Download(ctx, file, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})

	if err != nil {
		return fmt.Errorf("failed to download file %s/%s: %w", bucket, key, err)
	}

	log.Printf("Successfully downloaded %s/%s to %s", bucket, key, finalPath)
	return nil
}

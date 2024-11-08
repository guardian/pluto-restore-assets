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
	// Clean and normalize the path
	basePath = filepath.Clean(basePath)
	log.Printf("Downloading %d files", len(keys))
	log.Printf("Base path: %s", basePath)

	// Create each directory component separately to handle spaces
	components := strings.Split(basePath, string(os.PathSeparator))
	currentPath := "/"
	for _, component := range components {
		if component == "" {
			continue
		}
		currentPath = filepath.Join(currentPath, component)
		if err := os.MkdirAll(currentPath, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", currentPath, err)
		}
	}

	// Verify base path was created
	if _, err := os.Stat(basePath); err != nil {
		return fmt.Errorf("failed to verify base path creation %s: %w", basePath, err)
	}

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

	log.Printf("Attempting to create directory: %s", dir)
	// Create directory with verbose error checking
	err := os.MkdirAll(dir, 0755)
	if err != nil {
		if !os.IsExist(err) {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
		log.Printf("Directory already exists: %s", dir)
	}

	// Check directory permissions after creation
	dirInfo, err := os.Stat(dir)
	if err != nil {
		return fmt.Errorf("failed to stat directory %s: %w", dir, err)
	}
	log.Printf("Directory permissions: %v", dirInfo.Mode())

	finalPath := fullPath
	log.Printf("Attempting to create file: %s", finalPath)

	// Try to create file with verbose error checking
	file, err := os.OpenFile(finalPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to create file %s (permissions/space issue?): %w", finalPath, err)
	}
	defer file.Close()

	log.Printf("Starting download to %s", finalPath)
	downloader := manager.NewDownloader(client)
	numBytes, err := downloader.Download(ctx, file, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})

	if err != nil {
		// Clean up the empty file if download failed
		os.Remove(finalPath)
		return fmt.Errorf("failed to download file %s/%s: %w", bucket, key, err)
	}

	// Verify file was written
	fileInfo, err := os.Stat(finalPath)
	if err != nil {
		return fmt.Errorf("failed to stat downloaded file %s: %w", finalPath, err)
	}
	log.Printf("File size on disk: %d bytes", fileInfo.Size())
	log.Printf("File permissions: %v", fileInfo.Mode())

	log.Printf("Successfully downloaded %s/%s to %s (%d bytes)", bucket, key, finalPath, numBytes)
	return nil
}

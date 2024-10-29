package s3utils

import (
	"context"
	"log"
	"path/filepath"

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

	// if err := os.MkdirAll(dir, 0755); err != nil {
	// 	return fmt.Errorf("failed to create directory %s: %w", dir, err)
	// }

	// filename := filepath.Base(fullPath)
	// ext := filepath.Ext(filename)
	// name := strings.TrimSuffix(filename, ext)

	// for i := 0; ; i++ {
	// 	newPath := fullPath
	// 	if i > 0 {
	// 		newPath = filepath.Join(dir, fmt.Sprintf("%s_%d%s", name, i, ext))
	// 	}
	// 	if _, err := os.Stat(newPath); os.IsNotExist(err) {
	// 		fullPath = newPath
	// 		break
	// 	}
	// }

	// file, err := os.Create(fullPath)
	// if err != nil {
	// 	return fmt.Errorf("failed to create file %s: %w", fullPath, err)
	// }
	// defer file.Close()

	// downloader := manager.NewDownloader(client)

	// _, err = downloader.Download(ctx, file, &s3.GetObjectInput{
	// 	Bucket: aws.String(bucket),
	// 	Key:    aws.String(key),
	// })

	// if err != nil {
	// 	return fmt.Errorf("failed to download file %s/%s: %w", bucket, key, err)
	// }

	log.Printf("Successfully downloaded %s/%s to %s", bucket, key, fullPath)
	return nil
}

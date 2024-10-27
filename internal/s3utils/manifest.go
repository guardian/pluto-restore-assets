package s3utils

import (
	"context"
	"encoding/csv"
	"fmt"
	"log"
	"os"
	restoreTypes "pluto-restore-assets/internal/types"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// func GenerateCSVManifest(ctx context.Context, s3Client *s3.Client, bucket, prefix, filePath string) error {

func GenerateCSVManifest(ctx context.Context, s3Client S3ClientInterface, params restoreTypes.RestoreParams) error {
	if params.RestorePath == "" || params.RestorePath == "/" {
		return fmt.Errorf("invalid prefix: prefix is empty")
	}

	if len(params.AssetBucketList) == 0 {
		return fmt.Errorf("no asset buckets provided")
	}

	if params.ManifestLocalPath == "" {
		return fmt.Errorf("invalid file path: file path is empty")
	}

	// Create a map to track unique keys and their source buckets
	uniqueKeys := make(map[string]string) // key -> bucket

	// Check each bucket for the path and collect objects
	for _, bucket := range params.AssetBucketList {
		log.Printf("Checking bucket: %s for prefix: %s", bucket, params.RestorePath)

		input := &s3.ListObjectsV2Input{
			Bucket: aws.String(bucket),
			Prefix: aws.String(params.RestorePath),
		}

		paginator := s3.NewListObjectsV2Paginator(s3Client, input)

		for paginator.HasMorePages() {
			output, err := paginator.NextPage(ctx)
			if err != nil {
				return fmt.Errorf("failed to list objects in bucket %s: %w", bucket, err)
			}

			// If we found objects, add them to our map
			// First bucket's objects take precedence due to map overwrite behavior
			for _, obj := range output.Contents {
				if _, exists := uniqueKeys[*obj.Key]; !exists {
					uniqueKeys[*obj.Key] = bucket
				}
			}
		}
	}

	if len(uniqueKeys) == 0 {
		return fmt.Errorf("no objects found in any bucket with prefix: %s", params.RestorePath)
	}

	// Create and write to the manifest file
	file, err := os.Create(params.ManifestLocalPath)
	if err != nil {
		return fmt.Errorf("failed to create manifest file: %w", err)
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	// Write all unique entries to the CSV
	for key, bucket := range uniqueKeys {
		if err := writer.Write([]string{bucket, key}); err != nil {
			return fmt.Errorf("failed to write to CSV: %w", err)
		}
	}

	log.Printf("Generated manifest with %d unique objects from %d buckets",
		len(uniqueKeys), len(params.AssetBucketList))
	return nil
}

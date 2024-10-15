package s3utils

import (
	"context"
	"encoding/csv"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

func GenerateCSVManifest(ctx context.Context, s3Client *s3.Client, bucket, prefix, filePath string) error {
	if prefix == "" || prefix == "/" {
		return fmt.Errorf("invalid prefix: prefix is empty")
	}

	log.Printf("Generating CSV manifest for bucket: %s, prefix: %s", bucket, prefix)

	if filePath == "" {
		return fmt.Errorf("invalid file path: file path is empty")
	}

	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		return fmt.Errorf("failed to load AWS config: %w", err)
	}

	client := s3.NewFromConfig(cfg)

	paginator := s3.NewListObjectsV2Paginator(client, &s3.ListObjectsV2Input{
		Bucket: aws.String(bucket),
		Prefix: aws.String(prefix),
	})

	file, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("failed to create CSV file '%s': %w", filePath, err)
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	for paginator.HasMorePages() {
		page, err := paginator.NextPage(context.TODO())
		if err != nil {
			return fmt.Errorf("failed to list objects: %w", err)
		}

		for _, object := range page.Contents {
			// Skip directories
			if *object.Size == 0 && strings.HasSuffix(*object.Key, "/") {
				continue
			}

			// Check if the object is in Glacier or Deep Archive
			if object.StorageClass == types.ObjectStorageClassGlacier || object.StorageClass == types.ObjectStorageClassDeepArchive {
				err := writer.Write([]string{bucket, *object.Key})
				if err != nil {
					log.Printf("Failed to write object %s to CSV: %v", *object.Key, err)
					continue
				}
				log.Printf("Added object to manifest: %s", *object.Key)
			} else {
				log.Printf("Skipping object not in Glacier or Deep Archive: %s", *object.Key)
			}
		}
	}

	writer.Flush()
	if err := writer.Error(); err != nil {
		return fmt.Errorf("failed to flush CSV writer: %w", err)
	}

	log.Printf("CSV manifest created at %s", filePath)
	return nil
}

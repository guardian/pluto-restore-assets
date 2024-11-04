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

type ManifestStats struct {
	FileCount int
	TotalSize int64
}

func GenerateCSVManifest(ctx context.Context, s3Client S3ClientInterface, params restoreTypes.RestoreParams) (*ManifestStats, error) {
	log.Printf("Generating CSV manifest for params: %+v", params)
	if params.RestorePath == "" || params.RestorePath == "/" {
		return nil, fmt.Errorf("invalid prefix: prefix is empty")
	}

	stats := &ManifestStats{}
	uniqueKeys := make(map[string]string)

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
				return nil, fmt.Errorf("failed to list objects in bucket %s: %w", bucket, err)
			}

			for _, obj := range output.Contents {
				if _, exists := uniqueKeys[*obj.Key]; !exists {
					uniqueKeys[*obj.Key] = bucket
					stats.FileCount++
					stats.TotalSize += *obj.Size
				}
			}
		}
	}

	if len(uniqueKeys) == 0 {
		return nil, fmt.Errorf("no objects found in any bucket with prefix: %s", params.RestorePath)
	}

	file, err := os.Create(params.ManifestLocalPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create manifest file: %w", err)
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	for key, bucket := range uniqueKeys {
		if err := writer.Write([]string{bucket, key}); err != nil {
			return nil, fmt.Errorf("failed to write to CSV: %w", err)
		}
	}

	log.Printf("Generated manifest with %d unique objects from %d buckets",
		len(uniqueKeys), len(params.AssetBucketList))
	log.Printf("Stats: %+v", stats)
	return stats, nil
}

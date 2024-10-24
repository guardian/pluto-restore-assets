package s3utils

import (
	"context"
	"encoding/csv"
	"fmt"
	"log"
	"os"
	restoreTypes "pluto-restore-assets/types"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// func GenerateCSVManifest(ctx context.Context, s3Client *s3.Client, bucket, prefix, filePath string) error {

func GenerateCSVManifest(ctx context.Context, s3Client S3ClientInterface, params restoreTypes.RestoreParams) error {
	if params.RestorePath == "" || params.RestorePath == "/" {
		return fmt.Errorf("invalid prefix: prefix is empty")
	}

	log.Printf("Generating CSV manifest for bucket: %s, prefix: %s", params.AssetBucketList[0], params.RestorePath)

	if params.ManifestLocalPath == "" {
		return fmt.Errorf("invalid file path: file path is empty")
	}

	file, err := os.Create(params.ManifestLocalPath)
	if err != nil {
		return fmt.Errorf("failed to create manifest file: %w", err)
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	input := &s3.ListObjectsV2Input{
		Bucket: aws.String(params.AssetBucketList[0]),
		Prefix: aws.String(params.RestorePath),
	}

	paginator := s3.NewListObjectsV2Paginator(s3Client, input)

	for paginator.HasMorePages() {
		output, err := paginator.NextPage(ctx)
		if err != nil {
			return fmt.Errorf("failed to list objects: %w", err)
		}

		for _, obj := range output.Contents {
			err := writer.Write([]string{params.AssetBucketList[0], *obj.Key})
			if err != nil {
				return fmt.Errorf("failed to write to CSV: %w", err)
			}
		}
	}

	return nil
}

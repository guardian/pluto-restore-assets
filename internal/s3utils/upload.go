package s3utils

import (
	"context"
	"crypto/md5"
	"encoding/base64"
	"fmt"
	"io"
	"log"
	"os"
	"pluto-restore-assets/internal/types"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

func UploadFileToS3(ctx context.Context, s3Client S3ClientInterface, bucket, key, filePath string) (*s3.PutObjectOutput, error) {
	log.Printf("Uploading file to S3: s3://%s/%s", bucket, key)

	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	client := s3.NewFromConfig(cfg)

	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	// Calculate MD5 checksum
	hash := md5.New()
	if _, err := io.Copy(hash, file); err != nil {
		return nil, fmt.Errorf("failed to calculate MD5: %v", err)
	}
	md5sum := base64.StdEncoding.EncodeToString(hash.Sum(nil))

	// Reset file pointer
	if _, err := file.Seek(0, 0); err != nil {
		return nil, fmt.Errorf("failed to reset file pointer: %v", err)
	}

	// Upload the file
	result, err := client.PutObject(context.TODO(), &s3.PutObjectInput{
		Bucket:     aws.String(bucket),
		Key:        aws.String(key),
		Body:       file,
		ContentMD5: aws.String(md5sum),
	})

	if err != nil {
		return nil, fmt.Errorf("failed to upload file to S3: %v", err)
	}

	log.Printf("File uploaded to S3: s3://%s/%s", bucket, key)
	return result, nil
}

func GetObjectETag(ctx context.Context, s3Client *s3.Client, params types.RestoreParams) (string, error) {
	headOutput, err := s3Client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(params.ManifestBucket),
		Key:    aws.String(params.ManifestKey),
	})
	if err != nil {
		return "", fmt.Errorf("failed to get object metadata: %w", err)
	}
	if headOutput.ETag == nil {
		return "", fmt.Errorf("ETag is nil")
	}
	return *headOutput.ETag, nil
}

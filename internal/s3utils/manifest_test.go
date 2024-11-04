//go:generate mockgen -destination=mock_s3_client.go -package=s3utils . S3ClientInterface

package s3utils

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	restoreTypes "pluto-restore-assets/internal/types"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
)

func TestGenerateCSVManifest(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	mockS3Client := NewMockS3ClientInterface(mockCtrl)

	// Create a temporary directory for the manifest file
	tempDir, err := os.MkdirTemp("", "manifest_test")
	assert.NoError(t, err)
	defer os.RemoveAll(tempDir)

	testCases := []struct {
		name          string
		params        restoreTypes.RestoreParams
		setup         func()
		expectedFiles map[string]string // key -> bucket
		expectError   bool
	}{
		{
			name: "Files in first bucket only",
			params: restoreTypes.RestoreParams{
				AssetBucketList:   []string{"bucket1", "bucket2"},
				RestorePath:       "test-prefix/",
				ManifestLocalPath: filepath.Join(tempDir, "manifest1.csv"),
			},
			setup: func() {
				// First bucket call
				mockS3Client.EXPECT().
					ListObjectsV2(
						gomock.Any(),
						&s3.ListObjectsV2Input{
							Bucket: aws.String("bucket1"),
							Prefix: aws.String("test-prefix/"),
						},
						gomock.Any(),
					).Return(&s3.ListObjectsV2Output{
					Contents: []types.Object{
						{
							Key:  aws.String("test-prefix/file1.txt"),
							Size: aws.Int64(100),
						},
					},
					IsTruncated: aws.Bool(false),
				}, nil)

				// Second bucket call
				mockS3Client.EXPECT().
					ListObjectsV2(
						gomock.Any(),
						&s3.ListObjectsV2Input{
							Bucket: aws.String("bucket2"),
							Prefix: aws.String("test-prefix/"),
						},
						gomock.Any(),
					).Return(&s3.ListObjectsV2Output{
					Contents:    []types.Object{},
					IsTruncated: aws.Bool(false),
				}, nil)
			},
			expectedFiles: map[string]string{
				"test-prefix/file1.txt": "bucket1",
			},
		},
		{
			name: "Files in both buckets with overlap",
			params: restoreTypes.RestoreParams{
				AssetBucketList:   []string{"bucket1", "bucket2"},
				RestorePath:       "test-prefix/",
				ManifestLocalPath: filepath.Join(tempDir, "manifest2.csv"),
			},
			setup: func() {
				// First bucket call
				mockS3Client.EXPECT().ListObjectsV2(
					gomock.Any(),
					gomock.Eq(&s3.ListObjectsV2Input{
						Bucket: aws.String("bucket1"),
						Prefix: aws.String("test-prefix/"),
					}),
					gomock.Any(),
				).Return(&s3.ListObjectsV2Output{
					Contents: []types.Object{
						{
							Key:  aws.String("test-prefix/file1.txt"),
							Size: aws.Int64(100),
						},
						{
							Key:  aws.String("test-prefix/shared.txt"),
							Size: aws.Int64(100),
						},
					},
					IsTruncated: aws.Bool(false),
				}, nil)

				// Second bucket call
				mockS3Client.EXPECT().ListObjectsV2(
					gomock.Any(),
					gomock.Eq(&s3.ListObjectsV2Input{
						Bucket: aws.String("bucket2"),
						Prefix: aws.String("test-prefix/"),
					}),
					gomock.Any(),
				).Return(&s3.ListObjectsV2Output{
					Contents: []types.Object{
						{
							Key:  aws.String("test-prefix/file2.txt"),
							Size: aws.Int64(100),
						},
						{
							Key:  aws.String("test-prefix/shared.txt"),
							Size: aws.Int64(100),
						},
					},
					IsTruncated: aws.Bool(false),
				}, nil)
			},
			expectedFiles: map[string]string{
				"test-prefix/file1.txt":  "bucket1",
				"test-prefix/file2.txt":  "bucket2",
				"test-prefix/shared.txt": "bucket1", // First bucket takes precedence
			},
		},
		// Add more test cases here
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tc.setup()

			stats, err := GenerateCSVManifest(context.Background(), mockS3Client, tc.params)

			if tc.expectError {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
			assert.NotNil(t, stats)

			// Verify the manifest contents
			content, err := os.ReadFile(tc.params.ManifestLocalPath)
			assert.NoError(t, err)

			// Convert CSV content to map for easy comparison
			lines := strings.Split(string(content), "\n")
			resultMap := make(map[string]string)
			for _, line := range lines {
				if line == "" {
					continue
				}
				parts := strings.Split(line, ",")
				if len(parts) == 2 {
					resultMap[parts[1]] = parts[0]
				}
			}

			assert.Equal(t, tc.expectedFiles, resultMap)
		})
	}
}

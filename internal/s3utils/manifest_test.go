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
		name   string
		params restoreTypes.RestoreParams
		setup  func()
	}{
		{
			name: "Happy path",
			params: restoreTypes.RestoreParams{
				AssetBucketList:   []string{"test-bucket"},
				RestorePath:       "test-prefix/",
				ManifestLocalPath: filepath.Join(tempDir, "manifest.csv"),
			},
			setup: func() {
				mockS3Client.EXPECT().ListObjectsV2(
					gomock.Any(),
					&s3.ListObjectsV2Input{
						Bucket: aws.String("test-bucket"),
						Prefix: aws.String("test-prefix/"),
					},
					gomock.Any(),
				).Return(&s3.ListObjectsV2Output{
					Contents: []types.Object{
						{Key: aws.String("test-prefix/file1.txt")},
						{Key: aws.String("test-prefix/file2.txt")},
					},
				}, nil)
			},
		},
		// Add more test cases here
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tc.setup()

			err := GenerateCSVManifest(context.Background(), mockS3Client, tc.params)
			assert.NoError(t, err)

			// Read the generated CSV file
			content, err := os.ReadFile(tc.params.ManifestLocalPath)
			assert.NoError(t, err)

			lines := strings.Split(string(content), "\n")
			assert.Equal(t, []string{
				"test-bucket,test-prefix/file1.txt",
				"test-bucket,test-prefix/file2.txt",
				""}, // Empty string due to trailing newline
				lines)
		})
	}
}

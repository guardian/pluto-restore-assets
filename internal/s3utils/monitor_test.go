package s3utils

import (
	"context"
	"os"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
)

func (m *MockS3Client) Client() *s3.Client {
	return &s3.Client{}
}

func TestCheckRestoreStatus(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	mockS3Client := NewMockS3Client(mockCtrl)

	tests := []struct {
		name           string
		restoreHeader  *string
		expectedResult bool
		expectError    bool
	}{
		{
			name:           "Ongoing restore",
			restoreHeader:  aws.String(`ongoing-request="true"`),
			expectedResult: false,
			expectError:    false,
		},
		{
			name:           "Completed restore",
			restoreHeader:  aws.String(`ongoing-request="false", expiry-date="Wed, 07 Oct 2020 00:00:00 GMT"`),
			expectedResult: true,
			expectError:    false,
		},
		{
			name:           "No restore header",
			restoreHeader:  nil,
			expectedResult: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockS3Client.EXPECT().HeadObject(gomock.Any(), gomock.Any()).Return(&s3.HeadObjectOutput{
				Restore: tt.restoreHeader,
			}, nil)

			result, err := checkRestoreStatus(context.Background(), mockS3Client, "test-bucket", "test-key")

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedResult, result)
			}
		})
	}
}

func TestReadManifestFile(t *testing.T) {
	// Create a temporary manifest file
	content := `bucket1,key1
bucket2,key2
bucket3,key3`
	tmpfile, err := os.CreateTemp("", "manifest*.csv")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpfile.Name())

	if _, err := tmpfile.Write([]byte(content)); err != nil {
		t.Fatal(err)
	}
	if err := tmpfile.Close(); err != nil {
		t.Fatal(err)
	}

	// Test reading the manifest file
	entries, err := readManifestFile(tmpfile.Name())

	assert.NoError(t, err)
	assert.Len(t, entries, 3)
	assert.Equal(t, S3Entry{Bucket: "bucket1", Key: "key1"}, entries[0])
	assert.Equal(t, S3Entry{Bucket: "bucket2", Key: "key2"}, entries[1])
	assert.Equal(t, S3Entry{Bucket: "bucket3", Key: "key3"}, entries[2])
}

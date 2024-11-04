package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"pluto-restore-assets/internal/types"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3Types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/golang/mock/gomock"
)

type MockJobCreator struct {
	createCalled bool
	shouldError  bool
}

func (m *MockJobCreator) CreateRestoreJob(params types.RestoreParams) error {
	m.createCalled = true
	if m.shouldError {
		return fmt.Errorf("mock error")
	}
	return nil
}

func (m *MockJobCreator) GetJobLogs(jobName string) (string, error) {
	return "mock logs", nil
}

type MockS3Client struct {
	ListObjectsV2Func func(context.Context, *s3.ListObjectsV2Input, ...func(*s3.Options)) (*s3.ListObjectsV2Output, error)
	PutObjectFunc     func(context.Context, *s3.PutObjectInput, ...func(*s3.Options)) (*s3.PutObjectOutput, error)
	HeadObjectFunc    func(context.Context, *s3.HeadObjectInput, ...func(*s3.Options)) (*s3.HeadObjectOutput, error)
}

func (m *MockS3Client) ListObjectsV2(ctx context.Context, params *s3.ListObjectsV2Input, optFns ...func(*s3.Options)) (*s3.ListObjectsV2Output, error) {
	if m.ListObjectsV2Func == nil {
		return &s3.ListObjectsV2Output{
			Contents: []s3Types.Object{
				{
					Key:  aws.String("test-file.txt"),
					Size: aws.Int64(1024),
				},
			},
			IsTruncated: aws.Bool(false),
		}, nil
	}
	return m.ListObjectsV2Func(ctx, params, optFns...)
}

func (m *MockS3Client) PutObject(ctx context.Context, params *s3.PutObjectInput, optFns ...func(*s3.Options)) (*s3.PutObjectOutput, error) {
	log.Printf("Mock PutObject called with bucket: %v, key: %v", *params.Bucket, *params.Key)
	// Always return success in the mock
	return &s3.PutObjectOutput{}, nil
}

func (m *MockS3Client) HeadObject(ctx context.Context, params *s3.HeadObjectInput, optFns ...func(*s3.Options)) (*s3.HeadObjectOutput, error) {
	return &s3.HeadObjectOutput{}, nil
}

type Handler struct {
	s3Client   S3ClientInterface
	jobCreator JobCreator
}

func (h *Handler) CreateRestore(w http.ResponseWriter, r *http.Request) {
	var req types.RequestBody
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate required fields
	if req.User == "" {
		http.Error(w, "User is required", http.StatusBadRequest)
		return
	}
	if req.Path == "" {
		http.Error(w, "Path is required", http.StatusBadRequest)
		return
	}
	if req.ID == 0 {
		http.Error(w, "ID is required", http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusAccepted)
}

func TestCreateRestore(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	mockS3Client := NewMockS3ClientInterface(mockCtrl)

	// Set required environment variables
	os.Setenv("ASSET_BUCKET_LIST", "test-bucket")
	os.Setenv("MANIFEST_BUCKET", "manifest-bucket")
	os.Setenv("AWS_ROLE_ARN", "test-role")

	tests := []struct {
		name           string
		requestBody    types.RequestBody
		setupMocks     func()
		shouldError    bool
		expectedStatus int
		expectedError  string
	}{
		{
			name: "Valid request",
			requestBody: types.RequestBody{
				ID:            123,
				User:          "test.user@example.com",
				Path:          "/path/to/Assets/file.txt",
				RetrievalType: "Standard",
			},
			setupMocks: func() {
				mockS3Client.EXPECT().
					ListObjectsV2(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(&s3.ListObjectsV2Output{
						Contents: []s3Types.Object{
							{
								Key:  aws.String("test-file.txt"),
								Size: aws.Int64(1024),
							},
						},
						IsTruncated: aws.Bool(false),
					}, nil).AnyTimes()

				mockS3Client.EXPECT().
					PutObject(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(&s3.PutObjectOutput{}, nil).AnyTimes()
			},
			shouldError:    false,
			expectedStatus: http.StatusAccepted,
		},
		{
			name: "Invalid request - empty user",
			requestBody: types.RequestBody{
				ID:            123,
				User:          "",
				Path:          "/path/to/file.txt",
				RetrievalType: "Standard",
			},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "User is required",
		},
		{
			name: "Invalid request - empty path",
			requestBody: types.RequestBody{
				ID:            123,
				User:          "test.user@example.com",
				Path:          "",
				RetrievalType: "Standard",
			},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Path is required",
		},
		{
			name: "Invalid request - zero ID",
			requestBody: types.RequestBody{
				ID:            0,
				User:          "test.user@example.com",
				Path:          "/path/to/file.txt",
				RetrievalType: "Standard",
			},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "ID is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockCtrl := gomock.NewController(t)
			defer mockCtrl.Finish()

			mockS3Client := NewMockS3ClientInterface(mockCtrl)

			if tt.setupMocks != nil {
				tt.setupMocks()
			}

			mockJobCreator := &MockJobCreator{
				shouldError:  tt.shouldError,
				createCalled: false,
			}

			// Create a temporary manifest file
			manifestPath := "/tmp/manifest.csv"
			err := os.WriteFile(manifestPath, []byte("bucket,key,size\ntest-bucket,test-file.txt,1024\n"), 0644)
			if err != nil {
				t.Fatal(err)
			}
			defer os.Remove(manifestPath)

			// Set environment variables
			os.Setenv("MANIFEST_LOCAL_PATH", manifestPath)
			os.Setenv("ASSET_BUCKET_LIST", "test-bucket")
			os.Setenv("MANIFEST_BUCKET", "manifest-bucket")
			os.Setenv("AWS_ROLE_ARN", "test-role")
			// Clear AWS credentials to prevent real AWS calls
			os.Unsetenv("AWS_ACCESS_KEY_ID")
			os.Unsetenv("AWS_SECRET_ACCESS_KEY")
			os.Unsetenv("AWS_SESSION_TOKEN")

			body, _ := json.Marshal(tt.requestBody)
			req := httptest.NewRequest("POST", "/api/v1/restore", bytes.NewBuffer(body))
			w := httptest.NewRecorder()

			handler := &Handler{
				s3Client:   mockS3Client,
				jobCreator: mockJobCreator,
			}

			handler.CreateRestore(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d. Response body: %s", tt.expectedStatus, w.Code, w.Body.String())
			}

			if tt.expectedError != "" {
				respBody := w.Body.String()
				if !strings.Contains(respBody, tt.expectedError) {
					t.Errorf("Expected error message '%s', got '%s'", tt.expectedError, respBody)
				}
			}
		})
	}
}

func TestGetAWSAssetPath(t *testing.T) {
	tests := []struct {
		name     string
		fullPath string
		want     string
	}{
		{"With Assets", "/path/to/Assets/folder/file.txt", "folder/file.txt/"},
		{"Without Assets", "/path/to/folder/file.txt", "/path/to/folder/file.txt/"},
		{"Empty string", "", "/"},
		{"String with spaces", "path/to/Assets/folder/file with spaces.txt", "folder/file with spaces.txt/"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GetAWSAssetPath(tt.fullPath); got != tt.want {
				t.Errorf("getAWSAssetPath() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetBasePath(t *testing.T) {
	tests := []struct {
		name     string
		fullPath string
		want     string
	}{
		{"With Assets", "/path/to/Assets/folder/file.txt", "/path/to/Assets/"},
		{"String with spaces", "path/to/Assets/folder/file with spaces.txt", "path/to/Assets/"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GetBasePath(tt.fullPath); got != tt.want {
				t.Errorf("getBasePath() = %v, want %v", got, tt.want)
			}
		})
	}
}

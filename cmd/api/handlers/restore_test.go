package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"pluto-restore-assets/internal/types"
	"strings"
	"testing"
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

func TestCreateRestore(t *testing.T) {
	tests := []struct {
		name           string
		requestBody    types.RequestBody
		shouldError    bool
		expectedStatus int
		expectedError  string
	}{
		{
			name: "Valid request",
			requestBody: types.RequestBody{
				ID:   123,
				User: "test.user@example.com",
				Path: "/path/to/Assets/file.txt",
			},
			expectedStatus: http.StatusAccepted,
		},
		{
			name: "Invalid request - empty user",
			requestBody: types.RequestBody{
				ID:   123,
				User: "",
				Path: "/path/to/file.txt",
			},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "User is required",
		},
		{
			name: "Invalid request - empty path",
			requestBody: types.RequestBody{
				ID:   123,
				User: "test.user@example.com",
				Path: "",
			},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Path is required",
		},
		{
			name: "Invalid request - zero ID",
			requestBody: types.RequestBody{
				ID:   0,
				User: "test.user@example.com",
				Path: "/path/to/file.txt",
			},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "ID is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockJobCreator := &MockJobCreator{shouldError: tt.shouldError}
			handler := NewRestoreHandler(mockJobCreator)

			body, _ := json.Marshal(tt.requestBody)
			req := httptest.NewRequest("POST", "/api/v1/restore", bytes.NewBuffer(body))
			w := httptest.NewRecorder()

			handler.CreateRestore(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, w.Code)
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

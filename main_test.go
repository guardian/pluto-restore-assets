package main

import "testing"

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
			if got := getAWSAssetPath(tt.fullPath); got != tt.want {
				t.Errorf("getAWSAssetPath() = %v, want %v", got, tt.want)
			}
		})
	}
}

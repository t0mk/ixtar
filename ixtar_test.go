package ixtar

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCreateAndReadBundle(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "ixtar_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	testDir := filepath.Join(tempDir, "testdata")
	if err := os.Mkdir(testDir, 0755); err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}

	testFiles := map[string]string{
		"file1.txt":    "Hello, World!",
		"file2.txt":    "This is another test file.",
		"dir/file3.txt": "File in subdirectory",
	}

	for path, content := range testFiles {
		fullPath := filepath.Join(testDir, path)
		dir := filepath.Dir(fullPath)
		
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("Failed to create directory %s: %v", dir, err)
		}
		
		if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to write test file %s: %v", fullPath, err)
		}
	}

	bundlePath := filepath.Join(tempDir, "test.ixtar")
	
	if err := CreateBundle(testDir, bundlePath); err != nil {
		t.Fatalf("Failed to create bundle: %v", err)
	}

	ix, err := NewIxTar(bundlePath)
	if err != nil {
		t.Fatalf("Failed to open bundle: %v", err)
	}
	defer ix.Close()

	for path, expectedContent := range testFiles {
		data, err := ix.ExtractBytesOfFile(path)
		if err != nil {
			t.Errorf("Failed to extract file %s: %v", path, err)
			continue
		}
		
		if string(data) != expectedContent {
			t.Errorf("Content mismatch for file %s: expected %q, got %q", path, expectedContent, string(data))
		}
	}

	files := ix.ListFiles()
	if len(files) != len(testFiles) {
		t.Errorf("Expected %d files in index, got %d", len(testFiles), len(files))
	}

	_, err = ix.ExtractBytesOfFile("nonexistent.txt")
	if err == nil {
		t.Error("Expected error when extracting nonexistent file")
	}
}

func TestHashFilePath(t *testing.T) {
	tests := []struct {
		path     string
		expected string
	}{
		{"file.txt", "3d8e577bddb17db3"},
		{"./file.txt", "3d8e577bddb17db3"},
		{"path/to/file.txt", "3514e48cde714107"},
	}

	for _, test := range tests {
		result := hashFilePath(filepath.Clean(test.path))
		if result != test.expected {
			t.Errorf("Hash for %s: expected %s, got %s", test.path, test.expected, result)
		}
	}
}

func TestEmptyDirectory(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "ixtar_empty_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	testDir := filepath.Join(tempDir, "empty")
	if err := os.Mkdir(testDir, 0755); err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}

	bundlePath := filepath.Join(tempDir, "empty.ixtar")
	
	if err := CreateBundle(testDir, bundlePath); err != nil {
		t.Fatalf("Failed to create bundle from empty directory: %v", err)
	}

	ix, err := NewIxTar(bundlePath)
	if err != nil {
		t.Fatalf("Failed to open empty bundle: %v", err)
	}
	defer ix.Close()

	files := ix.ListFiles()
	if len(files) != 0 {
		t.Errorf("Expected 0 files in empty bundle, got %d", len(files))
	}
}
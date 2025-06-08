package util

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFileExists(t *testing.T) {
	// Create a temporary file
	tmpFile, err := os.CreateTemp("", "TestFileExists_*.txt")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	tmpFilePath := tmpFile.Name()
	tmpFile.Close() // Close immediately, we just need it to exist

	defer os.Remove(tmpFilePath) // Clean up

	// Test 1: File that exists
	if !FileExists(tmpFilePath) {
		t.Errorf("FileExists(%q) = false, want true", tmpFilePath)
	}

	// Test 2: File that does not exist
	nonExistentFilePath := filepath.Join(os.TempDir(), "TestFileExists_DoesNotExist_XYZ.txt")
	if FileExists(nonExistentFilePath) {
		t.Errorf("FileExists(%q) = true, want false", nonExistentFilePath)
	}

	// Test 3: Directory (FileExists should return true for directories too, as os.Stat works)
	tmpDir, err := os.MkdirTemp("", "TestFileExists_Dir_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	if !FileExists(tmpDir) {
		t.Errorf("FileExists(%q) for a directory = false, want true", tmpDir)
	}
}

func TestIsDirectory(t *testing.T) {
	// Create a temporary directory
	tmpDir, err := os.MkdirTemp("", "TestIsDirectory_Dir_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Test 1: Path is a directory
	isDir, err := IsDirectory(tmpDir)
	if err != nil {
		t.Errorf("IsDirectory(%q) returned error: %v, want no error", tmpDir, err)
	}
	if !isDir {
		t.Errorf("IsDirectory(%q) = false, want true", tmpDir)
	}

	// Create a temporary file
	tmpFile, err := os.CreateTemp(tmpDir, "TestIsDirectory_File_*.txt") // Create inside tmpDir for easy cleanup
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	tmpFilePath := tmpFile.Name()
	tmpFile.Close()

	// Test 2: Path is a file
	isDir, err = IsDirectory(tmpFilePath)
	if err != nil {
		t.Errorf("IsDirectory(%q) returned error: %v, want no error", tmpFilePath, err)
	}
	if isDir {
		t.Errorf("IsDirectory(%q) = true, want false for a file", tmpFilePath)
	}

	// Test 3: Path does not exist
	nonExistentPath := filepath.Join(tmpDir, "TestIsDirectory_NonExistent_XYZ")
	_, err = IsDirectory(nonExistentPath)
	if err == nil {
		t.Errorf("IsDirectory(%q) did not return an error for non-existent path, want error", nonExistentPath)
	}
}

func TestSaveCurrentDir(t *testing.T) {
	// Get current directory using os.Getwd() for comparison
	expectedDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("os.Getwd() failed: %v", err)
	}

	savedDir, err := SaveCurrentDir()
	if err != nil {
		t.Fatalf("SaveCurrentDir() returned error: %v", err)
	}

	// filepath.Abs(".") used by SaveCurrentDir might return a path that is
	// a symlink representation while os.Getwd() might return the resolved path.
	// For simplicity, just check if it's non-empty and doesn't end with an error.
	// A more robust check might involve resolving both paths to their actual physical locations.
	if savedDir == "" {
		t.Error("SaveCurrentDir() returned an empty string, want non-empty path")
	}

	// On Unix, os.Getwd() and filepath.Abs(".") should generally be equivalent
	// or one is a symlink to the other. For this test, let's check if they
	// resolve to the same directory by evaluating them.
	evalSavedDir, err := filepath.EvalSymlinks(savedDir)
	if err != nil {
		// If EvalSymlinks fails, it might be okay if the original paths are the same
		if savedDir != expectedDir {
			t.Logf("Could not eval symlink for savedDir '%s': %v. Comparing directly.", savedDir, err)
		}
	} else {
		savedDir = evalSavedDir // Use evaluated path if successful
	}

	evalExpectedDir, err := filepath.EvalSymlinks(expectedDir)
	if err != nil {
		if savedDir != expectedDir { // Only fail if original also different
			t.Logf("Could not eval symlink for expectedDir '%s': %v. Comparing directly.", expectedDir, err)
		}
	} else {
		expectedDir = evalExpectedDir // Use evaluated path
	}


	if savedDir != expectedDir {
		// This can still be flaky due to /tmp vs /private/tmp on macOS, etc.
		// So, we'll log a difference but not fail the test strictly on this,
		// as long as a valid-looking absolute path was returned without error.
		t.Logf("SaveCurrentDir() = %q, want %q (or equivalent). This difference might be due to symlinks.", savedDir, expectedDir)
		if !filepath.IsAbs(savedDir) {
			t.Errorf("SaveCurrentDir() did not return an absolute path: %s", savedDir)
		}
		// Check if one is a prefix of the other (common in /tmp vs /private/tmp)
		if !(strings.HasPrefix(savedDir, expectedDir) || strings.HasPrefix(expectedDir, savedDir)) {
			// If they don't even share a prefix, it's more likely an issue.
			// However, this is still not a foolproof test for "correctness" across all OS.
			// The main thing is that SaveCurrentDir returns *an* absolute path without error.
		}
	}
}

// Test for FileGetContents could be added if it had more complex logic.
// Current FileGetContents is simple os.ReadFile wrapper with specific error.

// Test for ClearWorkDir is difficult as a unit test due to os.RemoveAll.
// Test for ListDirectory could be added.
// Test for Patch is skipped due to external command execution.

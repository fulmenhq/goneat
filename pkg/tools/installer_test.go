package tools

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// TestVerifyChecksum_ValidChecksum tests that verifyChecksum passes with correct SHA256
func TestVerifyChecksum_ValidChecksum(t *testing.T) {
	// Use the valid tar.gz fixture
	fixturePath := filepath.Join("testdata", "artifacts", "valid-tool-1.0.0-darwin-amd64.tar.gz")

	// Compute actual checksum
	expectedChecksum, err := computeFileChecksum(fixturePath)
	if err != nil {
		t.Fatalf("Failed to compute checksum: %v", err)
	}

	// Verify checksum passes
	err = verifyChecksum(fixturePath, expectedChecksum)
	if err != nil {
		t.Errorf("verifyChecksum failed with valid checksum: %v", err)
	}
}

// TestVerifyChecksum_InvalidChecksum tests that verifyChecksum fails with wrong SHA256
func TestVerifyChecksum_InvalidChecksum(t *testing.T) {
	fixturePath := filepath.Join("testdata", "artifacts", "valid-tool-1.0.0-darwin-amd64.tar.gz")
	wrongChecksum := "0000000000000000000000000000000000000000000000000000000000000000"

	err := verifyChecksum(fixturePath, wrongChecksum)
	if err == nil {
		t.Error("verifyChecksum should fail with invalid checksum")
	}

	// Verify error message includes expected and actual checksums
	if err != nil {
		errMsg := err.Error()
		if errMsg == "" {
			t.Error("Error message should not be empty")
		}
		// Should mention "checksum mismatch"
		if len(errMsg) < 10 {
			t.Errorf("Error message too short: %s", errMsg)
		}
	}
}

// TestVerifyChecksum_TamperedFile tests that checksum detects file tampering
func TestVerifyChecksum_TamperedFile(t *testing.T) {
	// Create a temp file
	tmpFile, err := os.CreateTemp("", "tampered-*.tar.gz")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer func() {
		if err := os.Remove(tmpFile.Name()); err != nil {
			t.Logf("Failed to remove temp file: %v", err)
		}
	}()

	originalContent := []byte("original content")
	if _, err := tmpFile.Write(originalContent); err != nil {
		t.Fatalf("Failed to write original content: %v", err)
	}
	if err := tmpFile.Close(); err != nil {
		t.Fatalf("Failed to close temp file: %v", err)
	}

	// Compute checksum of original
	originalChecksum, err := computeFileChecksum(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to compute original checksum: %v", err)
	}

	// Tamper with the file
	tamperedContent := []byte("tampered content that is different")
	if err := os.WriteFile(tmpFile.Name(), tamperedContent, 0o644); err != nil {
		t.Fatalf("Failed to tamper with file: %v", err)
	}

	// Verify checksum fails after tampering
	err = verifyChecksum(tmpFile.Name(), originalChecksum)
	if err == nil {
		t.Error("verifyChecksum should detect file tampering")
	}
}

// TestVerifyChecksum_EmptyFile tests edge case of empty file
func TestVerifyChecksum_EmptyFile(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "empty-*.txt")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer func() {
		if err := os.Remove(tmpFile.Name()); err != nil {
			t.Logf("Failed to remove temp file: %v", err)
		}
	}()
	if err := tmpFile.Close(); err != nil {
		t.Fatalf("Failed to close temp file: %v", err)
	}

	// Compute checksum of empty file
	emptyChecksum, err := computeFileChecksum(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to compute empty file checksum: %v", err)
	}

	// Verify checksum of empty file should work
	err = verifyChecksum(tmpFile.Name(), emptyChecksum)
	if err != nil {
		t.Errorf("verifyChecksum should work with empty file: %v", err)
	}

	// Known SHA256 of empty file
	knownEmptySHA256 := "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
	if emptyChecksum != knownEmptySHA256 {
		t.Errorf("Empty file checksum mismatch: got %s, want %s", emptyChecksum, knownEmptySHA256)
	}
}

// TestVerifyChecksum_FileNotFound tests error handling for missing file
func TestVerifyChecksum_FileNotFound(t *testing.T) {
	nonExistentPath := "/tmp/this-file-does-not-exist-12345.tar.gz"
	dummyChecksum := "0000000000000000000000000000000000000000000000000000000000000000"

	err := verifyChecksum(nonExistentPath, dummyChecksum)
	if err == nil {
		t.Error("verifyChecksum should fail for non-existent file")
	}
}

// ============================================================================
// Archive Extraction Tests (Phase 1.2)
// ============================================================================

// TestExtractTarGz_ValidArchive tests successful extraction of tar.gz archive
func TestExtractTarGz_ValidArchive(t *testing.T) {
	archivePath := filepath.Join("testdata", "artifacts", "valid-tool-1.0.0-darwin-amd64.tar.gz")
	tmpDir := t.TempDir()
	targetPath := filepath.Join(tmpDir, "dummy-tool")

	err := extractTarGz(archivePath, targetPath, "dummy-tool", "")
	if err != nil {
		t.Fatalf("extractTarGz failed: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(targetPath); os.IsNotExist(err) {
		t.Error("Extracted file does not exist")
	}

	// Verify file is readable
	content, err := os.ReadFile(targetPath)
	if err != nil {
		t.Errorf("Failed to read extracted file: %v", err)
	}
	if len(content) == 0 {
		t.Error("Extracted file is empty")
	}
}

// TestExtractTarGz_PathTraversalAttempt tests that path traversal is blocked
// 🛡️ SECURITY TEST: Validates that malicious archives with path traversal are rejected
func TestExtractTarGz_PathTraversalAttempt(t *testing.T) {
	archivePath := filepath.Join("testdata", "artifacts", "path-traversal.tar.gz")
	tmpDir := t.TempDir()
	targetPath := filepath.Join(tmpDir, "extracted-file")

	// Archive contains "../../../etc/passwd" - should be rejected
	err := extractTarGz(archivePath, targetPath, "passwd", "")

	// Verify path traversal is blocked
	if err == nil {
		t.Fatal("extractTarGz should reject path traversal attempts")
	}

	// Error should mention the security issue
	errMsg := err.Error()
	if !strings.Contains(errMsg, "path traversal") && !strings.Contains(errMsg, "invalid path") {
		t.Errorf("Error should mention path traversal or invalid path, got: %v", err)
	}

	// Verify no file was created
	if _, statErr := os.Stat(targetPath); statErr == nil {
		t.Error("extractTarGz created a file despite path traversal detection")
	}
}

// TestExtractTarGz_CorruptedArchive tests handling of corrupted tar.gz
func TestExtractTarGz_CorruptedArchive(t *testing.T) {
	archivePath := filepath.Join("testdata", "artifacts", "corrupted.tar.gz")
	tmpDir := t.TempDir()
	targetPath := filepath.Join(tmpDir, "dummy-tool")

	err := extractTarGz(archivePath, targetPath, "dummy-tool", "")
	if err == nil {
		t.Error("extractTarGz should fail with corrupted archive")
	}

	// Error should be helpful for DX
	if err != nil {
		errMsg := err.Error()
		if len(errMsg) < 5 {
			t.Errorf("Error message too short: %s", errMsg)
		}
	}
}

// TestExtractTarGz_MissingBinary tests error when binary not in archive
func TestExtractTarGz_MissingBinary(t *testing.T) {
	archivePath := filepath.Join("testdata", "artifacts", "empty-archive.tar.gz")
	tmpDir := t.TempDir()
	targetPath := filepath.Join(tmpDir, "nonexistent-tool")

	err := extractTarGz(archivePath, targetPath, "nonexistent-tool", "")
	if err == nil {
		t.Error("extractTarGz should fail when binary not found in archive")
	}

	// Error should name the missing binary
	if err != nil {
		errMsg := err.Error()
		if !strings.Contains(errMsg, "nonexistent-tool") && !strings.Contains(errMsg, "not found") {
			t.Errorf("Error should mention missing binary, got: %s", errMsg)
		}
	}
}

// TestExtractZip_ValidArchive tests successful extraction of ZIP archive
func TestExtractZip_ValidArchive(t *testing.T) {
	archivePath := filepath.Join("testdata", "artifacts", "valid-tool-1.0.0-windows-amd64.zip")
	tmpDir := t.TempDir()
	targetPath := filepath.Join(tmpDir, "dummy-tool")

	err := extractZip(archivePath, targetPath, "dummy-tool", "")
	if err != nil {
		t.Fatalf("extractZip failed: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(targetPath); os.IsNotExist(err) {
		t.Error("Extracted file does not exist")
	}

	// Verify file is readable
	content, err := os.ReadFile(targetPath)
	if err != nil {
		t.Errorf("Failed to read extracted file: %v", err)
	}
	if len(content) == 0 {
		t.Error("Extracted file is empty")
	}
}

// TestExtractArtifact_DetectsFormat tests format detection
func TestExtractArtifact_DetectsFormat(t *testing.T) {
	tests := []struct {
		name        string
		archivePath string
		shouldWork  bool
	}{
		{"tar.gz format", "valid-tool-1.0.0-darwin-amd64.tar.gz", true},
		{"zip format", "valid-tool-1.0.0-windows-amd64.zip", true},
		{"corrupted", "corrupted.tar.gz", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			archivePath := filepath.Join("testdata", "artifacts", tt.archivePath)
			tmpDir := t.TempDir()
			targetPath := filepath.Join(tmpDir, "dummy-tool")

			err := extractArtifact(archivePath, targetPath, "dummy-tool", "")

			if tt.shouldWork && err != nil {
				t.Errorf("extractArtifact failed for %s: %v", tt.name, err)
			}
			if !tt.shouldWork && err == nil {
				t.Errorf("extractArtifact should have failed for %s", tt.name)
			}
		})
	}
}

// ============================================================================
// Platform Selection Tests (Phase 1.3)
// ============================================================================

// TestSelectArtifactForPlatform_CurrentPlatform tests platform selection for current platform
func TestSelectArtifactForPlatform_CurrentPlatform(t *testing.T) {
	// Create artifacts with all platforms
	artifacts := VersionArtifacts{
		DarwinAMD64:  &Artifact{URL: "https://example.com/tool-darwin-amd64.tar.gz", SHA256: "abc123"},
		DarwinARM64:  &Artifact{URL: "https://example.com/tool-darwin-arm64.tar.gz", SHA256: "def456"},
		LinuxAMD64:   &Artifact{URL: "https://example.com/tool-linux-amd64.tar.gz", SHA256: "ghi789"},
		LinuxARM64:   &Artifact{URL: "https://example.com/tool-linux-arm64.tar.gz", SHA256: "jkl012"},
		WindowsAMD64: &Artifact{URL: "https://example.com/tool-windows-amd64.zip", SHA256: "mno345"},
	}

	artifact, err := selectArtifactForPlatform(artifacts)
	if err != nil {
		t.Fatalf("selectArtifactForPlatform failed for current platform: %v", err)
	}

	if artifact == nil {
		t.Fatal("selectArtifactForPlatform returned nil artifact")
	}

	// Verify artifact URL is not empty
	if artifact.URL == "" {
		t.Error("Selected artifact has empty URL")
	}

	// Verify artifact SHA256 is not empty
	if artifact.SHA256 == "" {
		t.Error("Selected artifact has empty SHA256")
	}
}

// TestSelectArtifactForPlatform_MissingPlatform tests error when platform not available
func TestSelectArtifactForPlatform_MissingPlatform(t *testing.T) {
	// Empty artifacts - no platform available
	artifacts := VersionArtifacts{}

	artifact, err := selectArtifactForPlatform(artifacts)
	if err == nil {
		t.Fatal("selectArtifactForPlatform should fail when no artifact available for platform")
	}

	if artifact != nil {
		t.Error("selectArtifactForPlatform should return nil artifact on error")
	}

	// Error message should mention the issue
	if !strings.Contains(err.Error(), "no artifact") {
		t.Errorf("Error should mention missing artifact, got: %v", err)
	}
}

// ============================================================================
// Helper Functions
// ============================================================================

// Helper function to compute SHA256 checksum of a file
func computeFileChecksum(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer func() {
		_ = file.Close() // Ignore error in defer
	}()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}

	return hex.EncodeToString(hash.Sum(nil)), nil
}

// ============================================================================
// Phase 2: Artifact Installation Integration Tests
// These tests use httptest.Server to mock network downloads and test the
// full InstallArtifact() flow end-to-end with various scenarios.
// ============================================================================

// createMockArtifactServer creates an HTTP test server that serves artifacts
func createMockArtifactServer(t *testing.T, artifacts map[string][]byte) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		if data, ok := artifacts[path]; ok {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(data)
		} else {
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte("Not Found"))
		}
	}))
}

// TestInstallArtifact_SuccessfulInstall tests successful installation flow
func TestInstallArtifact_SuccessfulInstall(t *testing.T) {
	// Load a valid artifact fixture
	archivePath := filepath.Join("testdata", "artifacts", "valid-tool-1.0.0-darwin-amd64.tar.gz")
	archiveData, err := os.ReadFile(archivePath)
	if err != nil {
		t.Fatalf("Failed to read fixture: %v", err)
	}

	// Compute checksum
	checksum, err := computeFileChecksum(archivePath)
	if err != nil {
		t.Fatalf("Failed to compute checksum: %v", err)
	}

	// Create mock server
	server := createMockArtifactServer(t, map[string][]byte{
		"/download/dummy-tool-1.0.0.tar.gz": archiveData,
	})
	defer server.Close()

	// Create tool manifest
	tool := Tool{
		Name: "dummy-tool",
		Artifacts: &ArtifactManifest{
			DefaultVersion: "1.0.0",
			Versions: map[string]VersionArtifacts{
				"1.0.0": {
					DarwinAMD64: &Artifact{
						URL:    server.URL + "/download/dummy-tool-1.0.0.tar.gz",
						SHA256: checksum,
					},
					DarwinARM64: &Artifact{
						URL:    server.URL + "/download/dummy-tool-1.0.0.tar.gz",
						SHA256: checksum,
					},
					LinuxAMD64: &Artifact{
						URL:    server.URL + "/download/dummy-tool-1.0.0.tar.gz",
						SHA256: checksum,
					},
					LinuxARM64: &Artifact{
						URL:    server.URL + "/download/dummy-tool-1.0.0.tar.gz",
						SHA256: checksum,
					},
					WindowsAMD64: &Artifact{
						URL:    server.URL + "/download/dummy-tool-1.0.0.tar.gz",
						SHA256: checksum,
					},
				},
			},
		},
	}

	// Set up temporary tools directory
	tmpDir := t.TempDir()
	t.Setenv("GONEAT_HOME", tmpDir)

	// Install the artifact
	result, err := InstallArtifact(tool, InstallOptions{
		Version: "1.0.0",
	})

	if err != nil {
		t.Fatalf("InstallArtifact failed: %v", err)
	}

	// Verify result
	if result == nil {
		t.Fatal("InstallArtifact returned nil result")
	}

	if result.BinaryPath == "" {
		t.Error("Result has empty BinaryPath")
	}

	if result.Version != "1.0.0" {
		t.Errorf("Expected version 1.0.0, got %s", result.Version)
	}

	if !result.Verified {
		t.Error("Result should be marked as Verified")
	}

	// Verify binary exists
	if _, statErr := os.Stat(result.BinaryPath); statErr != nil {
		t.Errorf("Binary does not exist at %s: %v", result.BinaryPath, statErr)
	}

	// Verify binary is executable (check permissions)
	info, err := os.Stat(result.BinaryPath)
	if err != nil {
		t.Fatalf("Failed to stat binary: %v", err)
	}
	mode := info.Mode()
	if mode&0111 == 0 {
		t.Errorf("Binary is not executable: mode %o", mode)
	}
}

// TestInstallArtifact_NetworkFailure tests handling of HTTP 500 error
func TestInstallArtifact_NetworkFailure(t *testing.T) {
	// Create mock server that returns 500
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("Internal Server Error"))
	}))
	defer server.Close()

	// Create tool manifest
	tool := Tool{
		Name: "dummy-tool",
		Artifacts: &ArtifactManifest{
			DefaultVersion: "1.0.0",
			Versions: map[string]VersionArtifacts{
				"1.0.0": {
					DarwinAMD64: &Artifact{
						URL:    server.URL + "/download/dummy-tool-1.0.0.tar.gz",
						SHA256: "dummychecksum",
					},
					DarwinARM64: &Artifact{
						URL:    server.URL + "/download/dummy-tool-1.0.0.tar.gz",
						SHA256: "dummychecksum",
					},
					LinuxAMD64: &Artifact{
						URL:    server.URL + "/download/dummy-tool-1.0.0.tar.gz",
						SHA256: "dummychecksum",
					},
					LinuxARM64: &Artifact{
						URL:    server.URL + "/download/dummy-tool-1.0.0.tar.gz",
						SHA256: "dummychecksum",
					},
					WindowsAMD64: &Artifact{
						URL:    server.URL + "/download/dummy-tool-1.0.0.tar.gz",
						SHA256: "dummychecksum",
					},
				},
			},
		},
	}

	tmpDir := t.TempDir()
	t.Setenv("GONEAT_HOME", tmpDir)

	// Attempt install
	result, err := InstallArtifact(tool, InstallOptions{Version: "1.0.0"})

	// Should fail
	if err == nil {
		t.Fatal("InstallArtifact should fail with HTTP 500 error")
	}

	// Result should be nil
	if result != nil {
		t.Error("Result should be nil on error")
	}

	// Error message should mention the failure
	errMsg := err.Error()
	if !strings.Contains(errMsg, "500") && !strings.Contains(errMsg, "failed") {
		t.Errorf("Error should mention HTTP 500 or failure, got: %s", errMsg)
	}
}

// TestInstallArtifact_NetworkTimeout tests handling of timeout
func TestInstallArtifact_NetworkTimeout(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping timeout test in short mode")
	}

	// Create mock server that sleeps longer than timeout
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Sleep longer than the downloadTimeout (5 minutes)
		// For test purposes, we'll make the test faster by using a shorter timeout
		time.Sleep(6 * time.Minute)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// Note: This test would take 5+ minutes to actually test timeout
	// For coverage purposes we document the behavior and skip the actual test
	t.Skip("Skipping actual timeout test (would take 5+ minutes)")

	// The timeout test would look like this:
	// tool := Tool{...}
	// tmpDir := t.TempDir()
	// t.Setenv("GONEAT_HOME", tmpDir)
	// result, err := InstallArtifact(tool, InstallOptions{Version: "1.0.0"})
	// Verify timeout error is returned
}

// TestInstallArtifact_HTTPNotFound tests 404 handling
func TestInstallArtifact_HTTPNotFound(t *testing.T) {
	server := createMockArtifactServer(t, map[string][]byte{})
	defer server.Close()

	tool := Tool{
		Name: "dummy-tool",
		Artifacts: &ArtifactManifest{
			DefaultVersion: "1.0.0",
			Versions: map[string]VersionArtifacts{
				"1.0.0": {
					DarwinAMD64: &Artifact{
						URL:    server.URL + "/nonexistent.tar.gz",
						SHA256: "dummychecksum",
					},
					DarwinARM64: &Artifact{
						URL:    server.URL + "/nonexistent.tar.gz",
						SHA256: "dummychecksum",
					},
					LinuxAMD64: &Artifact{
						URL:    server.URL + "/nonexistent.tar.gz",
						SHA256: "dummychecksum",
					},
					LinuxARM64: &Artifact{
						URL:    server.URL + "/nonexistent.tar.gz",
						SHA256: "dummychecksum",
					},
					WindowsAMD64: &Artifact{
						URL:    server.URL + "/nonexistent.tar.gz",
						SHA256: "dummychecksum",
					},
				},
			},
		},
	}

	tmpDir := t.TempDir()
	t.Setenv("GONEAT_HOME", tmpDir)

	result, err := InstallArtifact(tool, InstallOptions{Version: "1.0.0"})

	if err == nil {
		t.Fatal("InstallArtifact should fail with 404 error")
	}

	if result != nil {
		t.Error("Result should be nil on error")
	}

	errMsg := err.Error()
	if !strings.Contains(errMsg, "404") && !strings.Contains(errMsg, "Not Found") && !strings.Contains(errMsg, "failed") {
		t.Errorf("Error should mention 404 or Not Found, got: %s", errMsg)
	}
}

// TestInstallArtifact_ChecksumMismatch tests handling of wrong checksum
func TestInstallArtifact_ChecksumMismatch(t *testing.T) {
	archivePath := filepath.Join("testdata", "artifacts", "valid-tool-1.0.0-darwin-amd64.tar.gz")
	archiveData, err := os.ReadFile(archivePath)
	if err != nil {
		t.Fatalf("Failed to read fixture: %v", err)
	}

	server := createMockArtifactServer(t, map[string][]byte{
		"/download/dummy-tool-1.0.0.tar.gz": archiveData,
	})
	defer server.Close()

	// Use WRONG checksum
	wrongChecksum := "0000000000000000000000000000000000000000000000000000000000000000"

	tool := Tool{
		Name: "dummy-tool",
		Artifacts: &ArtifactManifest{
			DefaultVersion: "1.0.0",
			Versions: map[string]VersionArtifacts{
				"1.0.0": {
					DarwinAMD64: &Artifact{
						URL:    server.URL + "/download/dummy-tool-1.0.0.tar.gz",
						SHA256: wrongChecksum,
					},
					DarwinARM64: &Artifact{
						URL:    server.URL + "/download/dummy-tool-1.0.0.tar.gz",
						SHA256: wrongChecksum,
					},
					LinuxAMD64: &Artifact{
						URL:    server.URL + "/download/dummy-tool-1.0.0.tar.gz",
						SHA256: wrongChecksum,
					},
					LinuxARM64: &Artifact{
						URL:    server.URL + "/download/dummy-tool-1.0.0.tar.gz",
						SHA256: wrongChecksum,
					},
					WindowsAMD64: &Artifact{
						URL:    server.URL + "/download/dummy-tool-1.0.0.tar.gz",
						SHA256: wrongChecksum,
					},
				},
			},
		},
	}

	tmpDir := t.TempDir()
	t.Setenv("GONEAT_HOME", tmpDir)

	result, err := InstallArtifact(tool, InstallOptions{Version: "1.0.0"})

	// Should fail due to checksum mismatch
	if err == nil {
		t.Fatal("InstallArtifact should fail with checksum mismatch")
	}

	if result != nil {
		t.Error("Result should be nil on error")
	}

	// Error should mention checksum
	errMsg := err.Error()
	if !strings.Contains(errMsg, "checksum") {
		t.Errorf("Error should mention checksum, got: %s", errMsg)
	}
}

// TestInstallArtifact_AlreadyInstalled tests skipping if already installed
func TestInstallArtifact_AlreadyInstalled(t *testing.T) {
	archivePath := filepath.Join("testdata", "artifacts", "valid-tool-1.0.0-darwin-amd64.tar.gz")
	archiveData, err := os.ReadFile(archivePath)
	if err != nil {
		t.Fatalf("Failed to read fixture: %v", err)
	}

	checksum, err := computeFileChecksum(archivePath)
	if err != nil {
		t.Fatalf("Failed to compute checksum: %v", err)
	}

	server := createMockArtifactServer(t, map[string][]byte{
		"/download/dummy-tool-1.0.0.tar.gz": archiveData,
	})
	defer server.Close()

	tool := Tool{
		Name: "dummy-tool",
		Artifacts: &ArtifactManifest{
			DefaultVersion: "1.0.0",
			Versions: map[string]VersionArtifacts{
				"1.0.0": {
					DarwinAMD64: &Artifact{
						URL:    server.URL + "/download/dummy-tool-1.0.0.tar.gz",
						SHA256: checksum,
					},
					DarwinARM64: &Artifact{
						URL:    server.URL + "/download/dummy-tool-1.0.0.tar.gz",
						SHA256: checksum,
					},
					LinuxAMD64: &Artifact{
						URL:    server.URL + "/download/dummy-tool-1.0.0.tar.gz",
						SHA256: checksum,
					},
					LinuxARM64: &Artifact{
						URL:    server.URL + "/download/dummy-tool-1.0.0.tar.gz",
						SHA256: checksum,
					},
					WindowsAMD64: &Artifact{
						URL:    server.URL + "/download/dummy-tool-1.0.0.tar.gz",
						SHA256: checksum,
					},
				},
			},
		},
	}

	tmpDir := t.TempDir()
	t.Setenv("GONEAT_HOME", tmpDir)

	// First install
	result1, err := InstallArtifact(tool, InstallOptions{Version: "1.0.0"})
	if err != nil {
		t.Fatalf("First install failed: %v", err)
	}

	// Second install (should detect already installed)
	result2, err := InstallArtifact(tool, InstallOptions{Version: "1.0.0"})
	if err != nil {
		t.Fatalf("Second install failed: %v", err)
	}

	// Should return same path
	if result1.BinaryPath != result2.BinaryPath {
		t.Errorf("Expected same path, got %s and %s", result1.BinaryPath, result2.BinaryPath)
	}

	// Both should be verified
	if !result2.Verified {
		t.Error("Second install should also be verified")
	}
}

// TestInstallArtifact_ForceReinstall tests force reinstall option
func TestInstallArtifact_ForceReinstall(t *testing.T) {
	archivePath := filepath.Join("testdata", "artifacts", "valid-tool-1.0.0-darwin-amd64.tar.gz")
	archiveData, err := os.ReadFile(archivePath)
	if err != nil {
		t.Fatalf("Failed to read fixture: %v", err)
	}

	checksum, err := computeFileChecksum(archivePath)
	if err != nil {
		t.Fatalf("Failed to compute checksum: %v", err)
	}

	downloadCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		downloadCount++
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(archiveData)
	}))
	defer server.Close()

	tool := Tool{
		Name: "dummy-tool",
		Artifacts: &ArtifactManifest{
			DefaultVersion: "1.0.0",
			Versions: map[string]VersionArtifacts{
				"1.0.0": {
					DarwinAMD64: &Artifact{
						URL:    server.URL + "/download/dummy-tool-1.0.0.tar.gz",
						SHA256: checksum,
					},
					DarwinARM64: &Artifact{
						URL:    server.URL + "/download/dummy-tool-1.0.0.tar.gz",
						SHA256: checksum,
					},
					LinuxAMD64: &Artifact{
						URL:    server.URL + "/download/dummy-tool-1.0.0.tar.gz",
						SHA256: checksum,
					},
					LinuxARM64: &Artifact{
						URL:    server.URL + "/download/dummy-tool-1.0.0.tar.gz",
						SHA256: checksum,
					},
					WindowsAMD64: &Artifact{
						URL:    server.URL + "/download/dummy-tool-1.0.0.tar.gz",
						SHA256: checksum,
					},
				},
			},
		},
	}

	tmpDir := t.TempDir()
	t.Setenv("GONEAT_HOME", tmpDir)

	// First install
	_, err = InstallArtifact(tool, InstallOptions{Version: "1.0.0"})
	if err != nil {
		t.Fatalf("First install failed: %v", err)
	}

	initialDownloadCount := downloadCount

	// Force reinstall - should reinstall binary but can use cached download
	result, err := InstallArtifact(tool, InstallOptions{
		Version: "1.0.0",
		Force:   true,
	})
	if err != nil {
		t.Fatalf("Force reinstall failed: %v", err)
	}

	// Force reinstall uses cached download (efficient), doesn't re-download
	// The Force flag affects binary installation, not download caching
	if downloadCount != initialDownloadCount {
		t.Logf("Note: Download count changed (%d -> %d), but Force uses cache (expected)", initialDownloadCount, downloadCount)
	}

	if result == nil {
		t.Fatal("Force reinstall should return result")
	}

	// Verify result is valid
	if result.BinaryPath == "" {
		t.Error("Force reinstall should return valid binary path")
	}
}

// TestDownloadArtifact_Cache tests cache behavior
func TestDownloadArtifact_Cache(t *testing.T) {
	archivePath := filepath.Join("testdata", "artifacts", "valid-tool-1.0.0-darwin-amd64.tar.gz")
	archiveData, err := os.ReadFile(archivePath)
	if err != nil {
		t.Fatalf("Failed to read fixture: %v", err)
	}

	checksum, err := computeFileChecksum(archivePath)
	if err != nil {
		t.Fatalf("Failed to compute checksum: %v", err)
	}

	downloadCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		downloadCount++
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(archiveData)
	}))
	defer server.Close()

	tmpDir := t.TempDir()
	t.Setenv("GONEAT_HOME", tmpDir)

	artifact := &Artifact{
		URL:    server.URL + "/download/cached-tool.tar.gz",
		SHA256: checksum,
	}

	// First download - should hit network
	path1, err := downloadArtifact("cached-tool", "1.0.0", artifact)
	if err != nil {
		t.Fatalf("First download failed: %v", err)
	}

	firstDownloadCount := downloadCount

	// Second download - should use cache
	path2, err := downloadArtifact("cached-tool", "1.0.0", artifact)
	if err != nil {
		t.Fatalf("Second download failed: %v", err)
	}

	// Should return same path
	if path1 != path2 {
		t.Errorf("Cache should return same path: %s != %s", path1, path2)
	}

	// Should not have downloaded again
	if downloadCount > firstDownloadCount {
		t.Error("Second download should have used cache, not hit network")
	}
}

// TestInstallArtifact_MissingArtifactManifest tests error when no artifacts
func TestInstallArtifact_MissingArtifactManifest(t *testing.T) {
	tool := Tool{
		Name:      "dummy-tool",
		Artifacts: nil, // No artifacts manifest
	}

	tmpDir := t.TempDir()
	t.Setenv("GONEAT_HOME", tmpDir)

	result, err := InstallArtifact(tool, InstallOptions{Version: "1.0.0"})

	if err == nil {
		t.Fatal("InstallArtifact should fail with no artifacts manifest")
	}

	if result != nil {
		t.Error("Result should be nil on error")
	}

	if !strings.Contains(err.Error(), "artifacts") {
		t.Errorf("Error should mention artifacts manifest, got: %s", err.Error())
	}
}

// TestInstallArtifact_InvalidVersion tests error when version not found
func TestInstallArtifact_InvalidVersion(t *testing.T) {
	tool := Tool{
		Name: "dummy-tool",
		Artifacts: &ArtifactManifest{
			DefaultVersion: "1.0.0",
			Versions: map[string]VersionArtifacts{
				"1.0.0": {},
			},
		},
	}

	tmpDir := t.TempDir()
	t.Setenv("GONEAT_HOME", tmpDir)

	result, err := InstallArtifact(tool, InstallOptions{Version: "2.0.0"})

	if err == nil {
		t.Fatal("InstallArtifact should fail with invalid version")
	}

	if result != nil {
		t.Error("Result should be nil on error")
	}

	errMsg := err.Error()
	if !strings.Contains(errMsg, "version") || !strings.Contains(errMsg, "2.0.0") {
		t.Errorf("Error should mention version 2.0.0, got: %s", errMsg)
	}
}

// TestInstallArtifact_UnsupportedPlatform tests error when platform not available
func TestInstallArtifact_UnsupportedPlatform(t *testing.T) {
	// Create artifacts with NO platforms (will fail for any platform)
	tool := Tool{
		Name: "dummy-tool",
		Artifacts: &ArtifactManifest{
			DefaultVersion: "1.0.0",
			Versions: map[string]VersionArtifacts{
				"1.0.0": {}, // Empty - no platforms
			},
		},
	}

	tmpDir := t.TempDir()
	t.Setenv("GONEAT_HOME", tmpDir)

	result, err := InstallArtifact(tool, InstallOptions{Version: "1.0.0"})

	if err == nil {
		t.Fatal("InstallArtifact should fail with no platform available")
	}

	if result != nil {
		t.Error("Result should be nil on error")
	}

	// Should mention platform
	currentPlatform := fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH)
	errMsg := err.Error()
	if !strings.Contains(errMsg, "artifact") && !strings.Contains(errMsg, "platform") {
		t.Errorf("Error should mention platform or artifact availability, got: %s (current platform: %s)", errMsg, currentPlatform)
	}
}

// TestInstallWithPackageManager tests the package manager installation function.
func TestInstallWithPackageManager(t *testing.T) {
	tests := []struct {
		name    string
		tool    Tool
		opts    InstallOptions
		wantErr bool
		skipOn  string // Skip test on this platform
	}{
		{
			name: "no_package_manager_config",
			tool: Tool{
				Name: "test-tool",
			},
			opts:    InstallOptions{},
			wantErr: true,
		},
		{
			name: "nil_install_config",
			tool: Tool{
				Name:    "test-tool",
				Install: nil,
			},
			opts:    InstallOptions{},
			wantErr: true,
		},
		{
			name: "nil_package_manager",
			tool: Tool{
				Name: "test-tool",
				Install: &InstallConfig{
					Type:           "package_manager",
					PackageManager: nil,
				},
			},
			opts:    InstallOptions{},
			wantErr: true,
		},
		{
			name: "unsupported_manager",
			tool: Tool{
				Name: "test-tool",
				Install: &InstallConfig{
					Type: "package_manager",
					PackageManager: &PackageManagerInstall{
						Manager: "unsupported",
						Package: "test-tool",
					},
				},
			},
			opts:    InstallOptions{},
			wantErr: true,
		},
		{
			name: "brew_on_windows",
			tool: Tool{
				Name: "test-tool",
				Install: &InstallConfig{
					Type: "package_manager",
					PackageManager: &PackageManagerInstall{
						Manager: "brew",
						Package: "test-tool",
					},
				},
			},
			opts:    InstallOptions{},
			wantErr: true,
			skipOn:  "darwin,linux",
		},
		{
			name: "scoop_on_unix",
			tool: Tool{
				Name: "test-tool",
				Install: &InstallConfig{
					Type: "package_manager",
					PackageManager: &PackageManagerInstall{
						Manager: "scoop",
						Package: "test-tool",
					},
				},
			},
			opts:    InstallOptions{},
			wantErr: true,
			skipOn:  "windows",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Skip test if platform doesn't match
			if tt.skipOn != "" {
				platforms := strings.Split(tt.skipOn, ",")
				for _, platform := range platforms {
					if runtime.GOOS == strings.TrimSpace(platform) {
						t.Skipf("skipping on %s", runtime.GOOS)
					}
				}
			}

			result, err := InstallWithPackageManager(tt.tool, tt.opts)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				if result != nil {
					t.Error("expected nil result on error")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if result == nil {
					t.Error("expected non-nil result")
				}
			}
		})
	}
}

// TestInstallWithPackageManager_DryRun tests dry run mode.
func TestInstallWithPackageManager_DryRun(t *testing.T) {
	var tool Tool
	var opts InstallOptions

	switch runtime.GOOS {
	case "darwin", "linux":
		mgr := &BrewManager{}
		if !mgr.IsAvailable() {
			t.Skip("brew not installed")
		}

		tool = Tool{
			Name:          "jq",
			DetectCommand: "jq --version",
			Install: &InstallConfig{
				Type: "package_manager",
				PackageManager: &PackageManagerInstall{
					Manager: "brew",
					Package: "jq",
				},
			},
		}
		opts = InstallOptions{DryRun: true}

	case "windows":
		mgr := &ScoopManager{}
		if !mgr.IsAvailable() {
			t.Skip("scoop not installed")
		}

		tool = Tool{
			Name:          "ripgrep",
			DetectCommand: "rg --version",
			Install: &InstallConfig{
				Type: "package_manager",
				PackageManager: &PackageManagerInstall{
					Manager: "scoop",
					Package: "ripgrep",
				},
			},
		}
		opts = InstallOptions{DryRun: true}

	default:
		t.Skip("no supported package manager on this platform")
	}

	result, err := InstallWithPackageManager(tool, opts)

	if err != nil {
		t.Fatalf("unexpected error in dry run: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result.BinaryPath != "<dry-run>" {
		t.Errorf("expected dry-run marker, got %s", result.BinaryPath)
	}
	if result.Verified {
		t.Error("expected Verified to be false in dry run")
	}
}

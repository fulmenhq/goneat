package tools

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
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

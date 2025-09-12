/*
Copyright © 2025 3 Leaps <info@3leaps.net>
*/
package assess

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fulmenhq/goneat/pkg/format/finalizer"
)

func TestFormatRunner_WhitespaceConsistency(t *testing.T) {
	// Create a temporary file with trailing whitespace
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test_whitespace.md")

	content := "This line has trailing spaces   \nThis line is fine\nAnother line with spaces   "
	err := os.WriteFile(testFile, []byte(content), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Test the shared detection function
	options := finalizer.NormalizationOptions{
		TrimTrailingWhitespace: true,
		EnsureEOF:              false,
	}

	if hasIssues, issues := finalizer.DetectWhitespaceIssues([]byte(content), options); !hasIssues {
		t.Error("Expected to detect whitespace issues, but none found")
	} else if len(issues) == 0 {
		t.Error("Expected whitespace issues to be reported")
	} else if !strings.Contains(issues[0].Description, "trailing whitespace") {
		t.Errorf("Expected trailing whitespace issue, got: %v", issues)
	}
}

func TestFormatRunner_ConsistencyWithFormatCommand(t *testing.T) {
	// This test ensures that the assessment and format command detect the same issues
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test_consistency.md")

	content := "Line with trailing spaces   \nNormal line\nAnother line with spaces   \n"
	err := os.WriteFile(testFile, []byte(content), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Test assessment detection
	runner := NewFormatAssessmentRunner()
	result, err := runner.Assess(context.Background(), tmpDir, AssessmentConfig{
		IncludeFiles: []string{filepath.Base(testFile)},
	})
	if err != nil {
		t.Fatalf("Assessment failed: %v", err)
	}

	if len(result.Issues) == 0 {
		t.Error("Expected assessment to detect whitespace issues")
	}

	foundWhitespace := false
	for _, issue := range result.Issues {
		if strings.Contains(issue.Message, "trailing whitespace") {
			foundWhitespace = true
			break
		}
	}

	if !foundWhitespace {
		t.Error("Assessment should detect trailing whitespace issues")
	}

	// Test shared detection function
	options := finalizer.NormalizationOptions{
		TrimTrailingWhitespace: true,
		EnsureEOF:              false,
	}

	if hasIssues, _ := finalizer.DetectWhitespaceIssues([]byte(content), options); !hasIssues {
		t.Error("Shared detection function should detect whitespace issues")
	}
}

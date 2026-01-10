package assess

import (
	"context"
	"testing"
	"time"
)

// TestToolsRunner_Assess_WithToolsPresent tests the tools assessment execution
func TestToolsRunner_Assess_WithToolsPresent(t *testing.T) {
	runner := NewToolsRunner()

	result, err := runner.Assess(context.Background(), ".", AssessmentConfig{})
	if err != nil {
		t.Errorf("Assess should not return error: %v", err)
	}

	// Assessment may fail if tools are missing, but should still return a valid result
	if result == nil {
		t.Fatal("Assessment should return a result")
	}

	if result != nil && result.Category != CategoryTools {
		t.Errorf("Expected category %s, got %s", CategoryTools, result.Category)
	}

	// Should have metrics about tools checked
	if result.Metrics == nil {
		t.Error("Result should contain metrics")
	}

	// Should have checked some tools
	if toolsChecked, ok := result.Metrics["tools_checked"].(int); ok {
		if toolsChecked <= 0 {
			t.Error("Should have checked at least one tool")
		}
	}
}

// TestToolsRunner_Assess_WithMetrics tests that metrics are properly collected
func TestToolsRunner_Assess_WithMetrics(t *testing.T) {
	runner := NewToolsRunner()

	result, err := runner.Assess(context.Background(), ".", AssessmentConfig{})
	if err != nil {
		t.Errorf("Assess should not return error: %v", err)
	}

	if result.Metrics == nil {
		t.Error("Result should contain metrics")
	}

	// Should have standard metrics
	requiredMetrics := []string{"tools_checked", "tools_present", "tools_missing"}
	for _, metric := range requiredMetrics {
		if _, exists := result.Metrics[metric]; !exists {
			t.Errorf("Metrics should contain %s", metric)
		}
	}

	// tools_checked should be reasonable number
	// v0.4.4+: foundation scope has 11 tools; increased threshold to 15
	if toolsChecked, ok := result.Metrics["tools_checked"].(int); ok {
		if toolsChecked <= 0 {
			t.Error("tools_checked should be positive")
		}
		if toolsChecked > 15 { // Should not have too many foundation tools
			t.Errorf("tools_checked seems too high: %d", toolsChecked)
		}
	}
}

// TestToolsRunner_GetEstimatedTime_EdgeCase tests time estimation edge cases
func TestToolsRunner_GetEstimatedTime_EdgeCase(t *testing.T) {
	runner := NewToolsRunner()

	estimatedTime := runner.GetEstimatedTime(".")
	if estimatedTime <= 0 {
		t.Error("Estimated time should be positive")
	}

	// Should be reasonable for tools checking (few seconds)
	// The implementation returns 2 seconds, which is reasonable
	expectedTime := 2 * time.Second
	if estimatedTime != expectedTime {
		t.Errorf("Expected estimated time %v, got %v", expectedTime, estimatedTime)
	}
}

// TestToolsRunner_Assess_Cancellation tests context cancellation handling
func TestToolsRunner_Assess_Cancellation(t *testing.T) {
	runner := NewToolsRunner()

	// Create a cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	result, err := runner.Assess(ctx, ".", AssessmentConfig{})

	// Should handle cancellation gracefully
	if err == nil {
		t.Log("Note: Assessment completed before cancellation took effect")
		return
	}

	if result == nil {
		t.Error("Should return result even on cancellation")
		return
	}

	if result.Category != CategoryTools {
		t.Errorf("Should maintain category even on cancellation: got %s", result.Category)
	}
}

// TestToolsRunner_Assess_EmptyTarget tests with empty target directory
func TestToolsRunner_Assess_EmptyTarget(t *testing.T) {
	runner := NewToolsRunner()

	// Test with empty string target
	result, err := runner.Assess(context.Background(), "", AssessmentConfig{})
	if err != nil {
		t.Errorf("Assess with empty target should not return error: %v", err)
	}

	if result == nil {
		t.Fatal("Should return result even with empty target")
	}

	if result != nil && result.Category != CategoryTools {
		t.Errorf("Should maintain category with empty target: got %s", result.Category)
	}
}

// TestToolsRunner_Assess_ErrorHandling tests error handling scenarios
func TestToolsRunner_Assess_ErrorHandling(t *testing.T) {
	runner := NewToolsRunner()

	// Test with non-existent directory
	result, err := runner.Assess(context.Background(), "/non/existent/directory", AssessmentConfig{})

	// Should handle gracefully
	if result == nil {
		t.Error("Should return result even for non-existent directory")
		return
	}

	if result.Category != CategoryTools {
		t.Errorf("Should maintain category for non-existent directory: got %s", result.Category)
	}

	if err != nil {
		t.Logf("Error for non-existent directory (expected): %v", err)
	}
}

// TestToolsRunner_Assess_EmptyDirectory tests with empty directory
func TestToolsRunner_Assess_EmptyDirectory(t *testing.T) {
	runner := NewToolsRunner()

	// Use temp directory (should be empty)
	tempDir := t.TempDir()

	result, err := runner.Assess(context.Background(), tempDir, AssessmentConfig{})
	if err != nil {
		t.Errorf("Assess on empty directory should not return error: %v", err)
	}

	if result == nil {
		t.Fatal("Should return result for empty directory")
	}

	if result != nil && result.Category != CategoryTools {
		t.Errorf("Should maintain category for empty directory: got %s", result.Category)
	}

	// Should still check tools even in empty directory
	if result.Metrics == nil {
		t.Error("Should have metrics even for empty directory")
	}
}

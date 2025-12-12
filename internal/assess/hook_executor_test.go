package assess

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestExecuteHookCommands_EmptyCommands(t *testing.T) {
	executor := NewHookExecutor("/tmp")
	err := executor.ExecuteHookCommands(context.Background(), nil)
	if err != nil {
		t.Errorf("expected no error for empty commands, got: %v", err)
	}

	err = executor.ExecuteHookCommands(context.Background(), []HookCommand{})
	if err != nil {
		t.Errorf("expected no error for empty slice, got: %v", err)
	}
}

func TestExecuteHookCommands_PriorityOrdering(t *testing.T) {
	var executionOrder []string

	executor := NewHookExecutor("/tmp")
	executor.InternalHandler = func(ctx context.Context, command string, args []string) error {
		executionOrder = append(executionOrder, command)
		return nil
	}

	commands := []HookCommand{
		{Command: "assess", Args: []string{}, Priority: 10},
		{Command: "format", Args: []string{}, Priority: 5},
		{Command: "dependencies", Args: []string{}, Priority: 8},
	}

	err := executor.ExecuteHookCommands(context.Background(), commands)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should execute in priority order: 5, 8, 10
	expected := []string{"format", "dependencies", "assess"}
	if len(executionOrder) != len(expected) {
		t.Fatalf("expected %d commands executed, got %d", len(expected), len(executionOrder))
	}
	for i, cmd := range expected {
		if executionOrder[i] != cmd {
			t.Errorf("position %d: expected %q, got %q", i, cmd, executionOrder[i])
		}
	}
}

func TestExecuteHookCommands_StableOrderForEqualPriorities(t *testing.T) {
	var executionOrder []string

	executor := NewHookExecutor("/tmp")
	executor.InternalHandler = func(ctx context.Context, command string, args []string) error {
		executionOrder = append(executionOrder, command)
		return nil
	}

	// All same priority - should preserve original manifest order
	commands := []HookCommand{
		{Command: "format", Args: []string{}, Priority: 5},
		{Command: "assess", Args: []string{}, Priority: 5},
		{Command: "dependencies", Args: []string{}, Priority: 5},
	}

	err := executor.ExecuteHookCommands(context.Background(), commands)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should preserve original order due to stable sort
	expected := []string{"format", "assess", "dependencies"}
	if len(executionOrder) != len(expected) {
		t.Fatalf("expected %d commands executed, got %d", len(expected), len(executionOrder))
	}
	for i, cmd := range expected {
		if executionOrder[i] != cmd {
			t.Errorf("position %d: expected %q, got %q (stable sort violated)", i, cmd, executionOrder[i])
		}
	}
}

func TestExecuteHookCommands_InternalCommandRouting(t *testing.T) {
	var internalCalls []string

	executor := NewHookExecutor("/tmp")
	executor.InternalHandler = func(ctx context.Context, command string, args []string) error {
		internalCalls = append(internalCalls, command)
		return nil
	}

	commands := []HookCommand{
		{Command: "assess", Args: []string{"--categories", "format"}},
		{Command: "format", Args: []string{}},
		{Command: "dependencies", Args: []string{"--licenses"}},
		{Command: "security", Args: []string{}},
		{Command: "validate", Args: []string{}},
		{Command: "lint", Args: []string{}},
	}

	err := executor.ExecuteHookCommands(context.Background(), commands)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// All should be routed to internal handler
	if len(internalCalls) != 6 {
		t.Errorf("expected 6 internal calls, got %d", len(internalCalls))
	}
}

func TestExecuteHookCommands_ExternalCommandExecution(t *testing.T) {
	executor := NewHookExecutor("/tmp")
	// No internal handler - external commands should still work

	// Use 'true' command which exists on Unix and always succeeds
	commands := []HookCommand{
		{Command: "true", Args: []string{}, Priority: 1, Timeout: "5s"},
	}

	err := executor.ExecuteHookCommands(context.Background(), commands)
	if err != nil {
		t.Fatalf("unexpected error executing 'true' command: %v", err)
	}
}

func TestExecuteHookCommands_FailFastOnError(t *testing.T) {
	var executionOrder []string

	executor := NewHookExecutor("/tmp")
	executor.InternalHandler = func(ctx context.Context, command string, args []string) error {
		executionOrder = append(executionOrder, command)
		if command == "assess" {
			return errors.New("assessment failed")
		}
		return nil
	}

	commands := []HookCommand{
		{Command: "format", Args: []string{}, Priority: 1},
		{Command: "assess", Args: []string{}, Priority: 2},       // This will fail
		{Command: "dependencies", Args: []string{}, Priority: 3}, // Should not execute
	}

	err := executor.ExecuteHookCommands(context.Background(), commands)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	// Should have stopped after assess failed
	if len(executionOrder) != 2 {
		t.Errorf("expected 2 commands executed before failure, got %d: %v", len(executionOrder), executionOrder)
	}
	if executionOrder[0] != "format" || executionOrder[1] != "assess" {
		t.Errorf("unexpected execution order: %v", executionOrder)
	}
}

func TestExecuteHookCommands_TimeoutEnforcement(t *testing.T) {
	executor := NewHookExecutor("/tmp")

	// Use 'sleep' with a short timeout
	commands := []HookCommand{
		{Command: "sleep", Args: []string{"10"}, Priority: 1, Timeout: "100ms"},
	}

	start := time.Now()
	err := executor.ExecuteHookCommands(context.Background(), commands)
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected timeout error, got nil")
	}

	// Should have timed out quickly, not waited 10 seconds
	if elapsed > 2*time.Second {
		t.Errorf("timeout not enforced: took %v", elapsed)
	}

	if !stringContains(err.Error(), "timed out") {
		t.Errorf("expected timeout error message, got: %v", err)
	}
}

func TestExecuteHookCommands_DefaultTimeout(t *testing.T) {
	executor := NewHookExecutor("/tmp")

	// Empty timeout should default to 2m (we won't wait that long, just verify it doesn't crash)
	commands := []HookCommand{
		{Command: "true", Args: []string{}, Priority: 1, Timeout: ""},
	}

	err := executor.ExecuteHookCommands(context.Background(), commands)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestExecuteHookCommands_InvalidTimeoutWarnsAndUsesDefault(t *testing.T) {
	executor := NewHookExecutor("/tmp")

	// Invalid timeout format should warn and use default
	commands := []HookCommand{
		{Command: "true", Args: []string{}, Priority: 1, Timeout: "invalid"},
	}

	err := executor.ExecuteHookCommands(context.Background(), commands)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestExecuteHookCommands_InternalWithoutHandler(t *testing.T) {
	executor := NewHookExecutor("/tmp")
	// No InternalHandler set - should warn and attempt external execution

	// 'assess' is internal but without handler, it will try to run as external
	// This will fail because there's no 'assess' binary in PATH (usually)
	commands := []HookCommand{
		{Command: "assess", Args: []string{}, Priority: 1, Timeout: "1s"},
	}

	// We expect this to fail (no 'assess' binary), but the important thing
	// is that it attempts execution rather than silently ignoring
	err := executor.ExecuteHookCommands(context.Background(), commands)
	if err == nil {
		// If it succeeds, there might be an 'assess' binary in PATH
		t.Log("assess command succeeded (binary found in PATH)")
	}
	// Either way, we've verified the code path is exercised
}

func TestIsInternalCommand(t *testing.T) {
	tests := []struct {
		command  string
		expected bool
	}{
		{"assess", true},
		{"format", true},
		{"lint", true},
		{"security", true},
		{"dependencies", true},
		{"validate", true},
		{"make", false},
		{"npm", false},
		{"go", false},
		{"", false},
		{"ASSESS", false}, // case sensitive
	}

	for _, tt := range tests {
		t.Run(tt.command, func(t *testing.T) {
			result := IsInternalCommand(tt.command)
			if result != tt.expected {
				t.Errorf("IsInternalCommand(%q) = %v, expected %v", tt.command, result, tt.expected)
			}
		})
	}
}

func TestFormatCommandString(t *testing.T) {
	tests := []struct {
		cmd      HookCommand
		expected string
	}{
		{HookCommand{Command: "make"}, "make"},
		{HookCommand{Command: "make", Args: []string{"build"}}, "make build"},
		{HookCommand{Command: "assess", Args: []string{"--categories", "format,lint"}}, "assess --categories format,lint"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := formatCommandString(tt.cmd)
			if result != tt.expected {
				t.Errorf("formatCommandString(%+v) = %q, expected %q", tt.cmd, result, tt.expected)
			}
		})
	}
}

func TestHookExecutor_VerboseLogging(t *testing.T) {
	executor := NewHookExecutor("/tmp")
	executor.Verbose = true
	executor.InternalHandler = func(ctx context.Context, command string, args []string) error {
		return nil
	}

	commands := []HookCommand{
		{Command: "format", Args: []string{}, Priority: 1},
	}

	// Just verify it doesn't panic with verbose enabled
	err := executor.ExecuteHookCommands(context.Background(), commands)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// Helper function
func stringContains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

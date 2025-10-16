package propagation

import (
	"testing"
	"time"
)

func TestNewRegistry(t *testing.T) {
	registry := NewRegistry()
	if registry == nil {
		t.Fatal("NewRegistry() returned nil")
	}
	if registry.managers == nil {
		t.Fatal("registry.managers is nil")
	}
	if len(registry.managers) != 0 {
		t.Errorf("expected empty managers map, got %d entries", len(registry.managers))
	}
}

func TestRegistry_Register(t *testing.T) {
	registry := NewRegistry()

	// Create a mock package manager
	mockManager := &mockPackageManager{name: "test"}

	registry.Register(mockManager)

	if _, exists := registry.managers["test"]; !exists {
		t.Fatal("manager was not registered")
	}
	if registry.managers["test"] != mockManager {
		t.Errorf("expected registered manager to be %v, got %v", mockManager, registry.managers["test"])
	}
}

func TestRegistry_Get(t *testing.T) {
	registry := NewRegistry()
	mockManager := &mockPackageManager{name: "test"}
	registry.Register(mockManager)

	manager, exists := registry.Get("test")
	if !exists {
		t.Fatal("expected manager to exist")
	}
	if manager != mockManager {
		t.Errorf("expected manager %v, got %v", mockManager, manager)
	}

	_, exists = registry.Get("nonexistent")
	if exists {
		t.Error("expected nonexistent manager to not exist")
	}
}

func TestRegistry_List(t *testing.T) {
	registry := NewRegistry()

	mock1 := &mockPackageManager{name: "test1"}
	mock2 := &mockPackageManager{name: "test2"}

	registry.Register(mock1)
	registry.Register(mock2)

	managers := registry.List()
	if len(managers) != 2 {
		t.Errorf("expected 2 managers, got %d", len(managers))
	}

	found1, found2 := false, false
	for _, manager := range managers {
		if manager == mock1 {
			found1 = true
		}
		if manager == mock2 {
			found2 = true
		}
	}
	if !found1 {
		t.Error("mock1 not found in list")
	}
	if !found2 {
		t.Error("mock2 not found in list")
	}
}

func TestNewPropagator(t *testing.T) {
	registry := NewRegistry()
	propagator := NewPropagator(registry)

	if propagator == nil {
		t.Fatal("NewPropagator() returned nil")
	}
	if propagator.registry != registry {
		t.Errorf("expected registry %v, got %v", registry, propagator.registry)
	}
}

// Mock implementation for testing
type mockPackageManager struct {
	name string
}

func (m *mockPackageManager) Name() string {
	return m.name
}

func (m *mockPackageManager) Detect(root string) ([]string, error) {
	return []string{"test.json"}, nil
}

func (m *mockPackageManager) ExtractVersion(file string) (string, error) {
	return "1.0.0", nil
}

func (m *mockPackageManager) UpdateVersion(file, version string) error {
	return nil
}

func (m *mockPackageManager) ValidateVersion(file, version string) error {
	return nil
}

func TestPropagationResult(t *testing.T) {
	result := &PropagationResult{
		Success:   true,
		Processed: 5,
		Errors:    []PropagationError{},
		Changes:   []FileChange{},
		Duration:  time.Second * 2,
	}

	if !result.Success {
		t.Error("expected Success to be true")
	}
	if result.Processed != 5 {
		t.Errorf("expected Processed to be 5, got %d", result.Processed)
	}
	if len(result.Errors) != 0 {
		t.Errorf("expected empty Errors, got %d", len(result.Errors))
	}
	if len(result.Changes) != 0 {
		t.Errorf("expected empty Changes, got %d", len(result.Changes))
	}
	if result.Duration != time.Second*2 {
		t.Errorf("expected Duration to be 2s, got %v", result.Duration)
	}
}

func TestPropagationError(t *testing.T) {
	testErr := &testError{msg: "test error"}
	err := PropagationError{
		File:    "test.json",
		Error:   testErr,
		Message: "version mismatch",
	}

	if err.File != "test.json" {
		t.Errorf("expected File to be 'test.json', got %s", err.File)
	}
	if err.Error.Error() != "test error" {
		t.Errorf("expected Error message to be 'test error', got %s", err.Error.Error())
	}
	if err.Message != "version mismatch" {
		t.Errorf("expected Message to be 'version mismatch', got %s", err.Message)
	}
}

func TestFileChange(t *testing.T) {
	change := FileChange{
		File:       "test.json",
		OldVersion: "1.0.0",
		NewVersion: "2.0.0",
		BackupPath: "/tmp/backup.json",
	}

	if change.File != "test.json" {
		t.Errorf("expected File to be 'test.json', got %s", change.File)
	}
	if change.OldVersion != "1.0.0" {
		t.Errorf("expected OldVersion to be '1.0.0', got %s", change.OldVersion)
	}
	if change.NewVersion != "2.0.0" {
		t.Errorf("expected NewVersion to be '2.0.0', got %s", change.NewVersion)
	}
	if change.BackupPath != "/tmp/backup.json" {
		t.Errorf("expected BackupPath to be '/tmp/backup.json', got %s", change.BackupPath)
	}
}

func TestPropagateOptions(t *testing.T) {
	opts := PropagateOptions{
		DryRun:       true,
		Force:        false,
		Targets:      []string{"package.json"},
		Exclude:      []string{"node_modules/**"},
		Backup:       true,
		ValidateOnly: false,
		PolicyPath:   ".goneat/version-policy.yaml",
	}

	if !opts.DryRun {
		t.Error("expected DryRun to be true")
	}
	if opts.Force {
		t.Error("expected Force to be false")
	}
	if len(opts.Targets) != 1 || opts.Targets[0] != "package.json" {
		t.Errorf("expected Targets to be ['package.json'], got %v", opts.Targets)
	}
	if len(opts.Exclude) != 1 || opts.Exclude[0] != "node_modules/**" {
		t.Errorf("expected Exclude to be ['node_modules/**'], got %v", opts.Exclude)
	}
	if !opts.Backup {
		t.Error("expected Backup to be true")
	}
	if opts.ValidateOnly {
		t.Error("expected ValidateOnly to be false")
	}
	if opts.PolicyPath != ".goneat/version-policy.yaml" {
		t.Errorf("expected PolicyPath to be '.goneat/version-policy.yaml', got %s", opts.PolicyPath)
	}
}

// testError implements error for testing
type testError struct {
	msg string
}

func (e *testError) Error() string {
	return e.msg
}

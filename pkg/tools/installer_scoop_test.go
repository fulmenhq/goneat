package tools

import (
	"runtime"
	"testing"
)

// TestNewScoopInstaller tests installer creation.
func TestNewScoopInstaller(t *testing.T) {
	tool := &Tool{
		Name:          "test-tool",
		DetectCommand: "test-tool --version",
	}

	config := &PackageManagerInstall{
		Manager: "scoop",
		Package: "test-tool",
	}

	installer := NewScoopInstaller(tool, config, false)
	if installer == nil {
		t.Fatal("expected non-nil installer")
	}
	if installer.tool != tool {
		t.Error("tool not set correctly")
	}
	if installer.config != config {
		t.Error("config not set correctly")
	}
	if installer.dryRun != false {
		t.Error("dryRun not set correctly")
	}
}

// TestNewScoopInstaller_DryRun tests dry run mode.
func TestNewScoopInstaller_DryRun(t *testing.T) {
	tool := &Tool{
		Name:          "test-tool",
		DetectCommand: "test-tool --version",
	}

	config := &PackageManagerInstall{
		Manager: "scoop",
		Package: "test-tool",
	}

	installer := NewScoopInstaller(tool, config, true)
	if !installer.dryRun {
		t.Error("expected dryRun to be true")
	}
}

// TestScoopInstaller_buildInstallArgs tests command argument building.
func TestScoopInstaller_buildInstallArgs(t *testing.T) {
	tests := []struct {
		name         string
		config       *PackageManagerInstall
		expectedArgs []string
	}{
		{
			name: "simple_package",
			config: &PackageManagerInstall{
				Manager: "scoop",
				Package: "ripgrep",
			},
			expectedArgs: []string{"install", "ripgrep"},
		},
		{
			name: "with_flags",
			config: &PackageManagerInstall{
				Manager: "scoop",
				Package: "ripgrep",
				Flags:   []string{"--no-cache"},
			},
			expectedArgs: []string{"install", "--no-cache", "ripgrep"},
		},
		{
			name: "with_multiple_flags",
			config: &PackageManagerInstall{
				Manager: "scoop",
				Package: "git",
				Flags:   []string{"--global", "--skip"},
			},
			expectedArgs: []string{"install", "--global", "--skip", "git"},
		},
		{
			name: "with_bucket",
			config: &PackageManagerInstall{
				Manager: "scoop",
				Bucket:  "extras",
				Package: "vscode",
			},
			expectedArgs: []string{"install", "vscode"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tool := &Tool{
				Name:          "test-tool",
				DetectCommand: "test-tool --version",
			}

			installer := NewScoopInstaller(tool, tt.config, false)
			args := installer.buildInstallArgs()

			if len(args) != len(tt.expectedArgs) {
				t.Fatalf("expected %d args, got %d: %v", len(tt.expectedArgs), len(args), args)
			}

			for i, expected := range tt.expectedArgs {
				if args[i] != expected {
					t.Errorf("arg %d: expected '%s', got '%s'", i, expected, args[i])
				}
			}
		})
	}
}

// TestScoopInstaller_Install_NoScoopAvailable tests error when scoop is not available.
func TestScoopInstaller_Install_NoScoopAvailable(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("this test requires platform where scoop is not available")
	}

	tool := &Tool{
		Name:          "test-tool",
		DetectCommand: "test-tool --version",
	}

	config := &PackageManagerInstall{
		Manager: "scoop",
		Package: "test-tool",
	}

	installer := NewScoopInstaller(tool, config, false)
	result, err := installer.Install()

	if err == nil {
		t.Error("expected error when scoop not available")
	}
	if result != nil {
		t.Error("expected nil result on error")
	}
	if err != nil && err.Error() == "" {
		t.Error("expected non-empty error message")
	}
}

// TestScoopInstaller_Install_DryRun tests dry run mode.
func TestScoopInstaller_Install_DryRun(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("scoop not supported on this platform")
	}

	mgr := &ScoopManager{}
	if !mgr.IsAvailable() {
		t.Skip("scoop not installed")
	}

	tool := &Tool{
		Name:          "ripgrep",
		DetectCommand: "rg --version",
	}

	config := &PackageManagerInstall{
		Manager: "scoop",
		Package: "ripgrep",
	}

	installer := NewScoopInstaller(tool, config, true)
	result, err := installer.Install()

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

// TestScoopInstaller_verifyInstallation tests installation verification.
func TestScoopInstaller_verifyInstallation(t *testing.T) {
	// Test with a tool we know exists (scoop itself)
	if runtime.GOOS != "windows" {
		t.Skip("scoop not supported on this platform")
	}

	mgr := &ScoopManager{}
	if !mgr.IsAvailable() {
		t.Skip("scoop not installed")
	}

	tool := &Tool{
		Name:          "scoop",
		DetectCommand: "scoop --version",
	}

	config := &PackageManagerInstall{
		Manager: "scoop",
		Package: "scoop",
	}

	installer := NewScoopInstaller(tool, config, false)
	path, err := installer.verifyInstallation()

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if path == "" || path == "<unknown>" {
		t.Errorf("expected valid path, got %s", path)
	}
}

// TestScoopInstaller_verifyInstallation_NoDetectCommand tests error handling.
func TestScoopInstaller_verifyInstallation_NoDetectCommand(t *testing.T) {
	tool := &Tool{
		Name:          "test-tool",
		DetectCommand: "",
	}

	config := &PackageManagerInstall{
		Manager: "scoop",
		Package: "test-tool",
	}

	installer := NewScoopInstaller(tool, config, false)
	path, err := installer.verifyInstallation()

	if err == nil {
		t.Error("expected error for missing detect_command")
	}
	if path != "<unknown>" {
		t.Errorf("expected <unknown> path on error, got %s", path)
	}
}

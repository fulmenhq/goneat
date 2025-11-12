package tools

import (
	"runtime"
	"testing"
)

// TestNewBrewInstaller tests installer creation.
func TestNewBrewInstaller(t *testing.T) {
	tool := &Tool{
		Name:          "test-tool",
		DetectCommand: "test-tool --version",
	}

	config := &PackageManagerInstall{
		Manager: "brew",
		Package: "test-tool",
	}

	installer := NewBrewInstaller(tool, config, false)
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

// TestNewBrewInstaller_DryRun tests dry run mode.
func TestNewBrewInstaller_DryRun(t *testing.T) {
	tool := &Tool{
		Name:          "test-tool",
		DetectCommand: "test-tool --version",
	}

	config := &PackageManagerInstall{
		Manager: "brew",
		Package: "test-tool",
	}

	installer := NewBrewInstaller(tool, config, true)
	if !installer.dryRun {
		t.Error("expected dryRun to be true")
	}
}

// TestBrewInstaller_buildInstallArgs tests command argument building.
func TestBrewInstaller_buildInstallArgs(t *testing.T) {
	tests := []struct {
		name         string
		config       *PackageManagerInstall
		expectedArgs []string
	}{
		{
			name: "formula_default",
			config: &PackageManagerInstall{
				Manager:     "brew",
				Package:     "jq",
				PackageType: "",
			},
			expectedArgs: []string{"install", "--formula", "jq"},
		},
		{
			name: "formula_explicit",
			config: &PackageManagerInstall{
				Manager:     "brew",
				Package:     "jq",
				PackageType: "formula",
			},
			expectedArgs: []string{"install", "--formula", "jq"},
		},
		{
			name: "cask",
			config: &PackageManagerInstall{
				Manager:     "brew",
				Package:     "docker",
				PackageType: "cask",
			},
			expectedArgs: []string{"install", "--cask", "docker"},
		},
		{
			name: "with_flags",
			config: &PackageManagerInstall{
				Manager:     "brew",
				Package:     "jq",
				PackageType: "formula",
				Flags:       []string{"--quiet", "--force"},
			},
			expectedArgs: []string{"install", "--formula", "--quiet", "--force", "jq"},
		},
		{
			name: "with_tap_package",
			config: &PackageManagerInstall{
				Manager:     "brew",
				Tap:         "fulmenhq/homebrew-tap",
				Package:     "fulmenhq/tap/goneat",
				PackageType: "formula",
			},
			expectedArgs: []string{"install", "--formula", "fulmenhq/tap/goneat"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tool := &Tool{
				Name:          "test-tool",
				DetectCommand: "test-tool --version",
			}

			installer := NewBrewInstaller(tool, tt.config, false)
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

// TestBrewInstaller_Install_NoBrewAvailable tests error when brew is not available.
func TestBrewInstaller_Install_NoBrewAvailable(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("this test requires platform where brew is not available")
	}

	tool := &Tool{
		Name:          "test-tool",
		DetectCommand: "test-tool --version",
	}

	config := &PackageManagerInstall{
		Manager: "brew",
		Package: "test-tool",
	}

	installer := NewBrewInstaller(tool, config, false)
	result, err := installer.Install()

	if err == nil {
		t.Error("expected error when brew not available")
	}
	if result != nil {
		t.Error("expected nil result on error")
	}
	if err != nil && err.Error() == "" {
		t.Error("expected non-empty error message")
	}
}

// TestBrewInstaller_Install_DryRun tests dry run mode.
func TestBrewInstaller_Install_DryRun(t *testing.T) {
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		t.Skip("brew not supported on this platform")
	}

	mgr := &BrewManager{}
	if !mgr.IsAvailable() {
		t.Skip("brew not installed")
	}

	tool := &Tool{
		Name:          "jq",
		DetectCommand: "jq --version",
	}

	config := &PackageManagerInstall{
		Manager: "brew",
		Package: "jq",
	}

	installer := NewBrewInstaller(tool, config, true)
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

// TestBrewInstaller_verifyInstallation tests installation verification.
func TestBrewInstaller_verifyInstallation(t *testing.T) {
	// Test with a tool we know exists (brew itself)
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		t.Skip("brew not supported on this platform")
	}

	mgr := &BrewManager{}
	if !mgr.IsAvailable() {
		t.Skip("brew not installed")
	}

	tool := &Tool{
		Name:          "brew",
		DetectCommand: "brew --version",
	}

	config := &PackageManagerInstall{
		Manager: "brew",
		Package: "brew",
	}

	installer := NewBrewInstaller(tool, config, false)
	path, err := installer.verifyInstallation()

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if path == "" || path == "<unknown>" {
		t.Errorf("expected valid path, got %s", path)
	}
}

// TestBrewInstaller_verifyInstallation_NoDetectCommand tests error handling.
func TestBrewInstaller_verifyInstallation_NoDetectCommand(t *testing.T) {
	tool := &Tool{
		Name:          "test-tool",
		DetectCommand: "",
	}

	config := &PackageManagerInstall{
		Manager: "brew",
		Package: "test-tool",
	}

	installer := NewBrewInstaller(tool, config, false)
	path, err := installer.verifyInstallation()

	if err == nil {
		t.Error("expected error for missing detect_command")
	}
	if path != "<unknown>" {
		t.Errorf("expected <unknown> path on error, got %s", path)
	}
}

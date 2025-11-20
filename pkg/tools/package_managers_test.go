package tools

import (
	"runtime"
	"strings"
	"testing"
)

// TestBrewManager_Name tests the Name method.
func TestBrewManager_Name(t *testing.T) {
	mgr := &BrewManager{}
	if mgr.Name() != "brew" {
		t.Errorf("expected name 'brew', got '%s'", mgr.Name())
	}
}

// TestBrewManager_InstallationURL tests the InstallationURL method.
func TestBrewManager_InstallationURL(t *testing.T) {
	mgr := &BrewManager{}
	expected := "https://brew.sh"
	if mgr.InstallationURL() != expected {
		t.Errorf("expected URL '%s', got '%s'", expected, mgr.InstallationURL())
	}
}

// TestBrewManager_SupportedPlatforms tests the SupportedPlatforms method.
func TestBrewManager_SupportedPlatforms(t *testing.T) {
	mgr := &BrewManager{}
	platforms := mgr.SupportedPlatforms()

	expected := map[string]bool{"darwin": true, "linux": true}
	if len(platforms) != len(expected) {
		t.Fatalf("expected %d platforms, got %d", len(expected), len(platforms))
	}

	for _, platform := range platforms {
		if !expected[platform] {
			t.Errorf("unexpected platform: %s", platform)
		}
	}
}

// TestBrewManager_IsSupportedOnCurrentPlatform tests platform support checking.
func TestBrewManager_IsSupportedOnCurrentPlatform(t *testing.T) {
	mgr := &BrewManager{}
	supported := mgr.IsSupportedOnCurrentPlatform()

	goos := runtime.GOOS
	expectedSupport := (goos == "darwin" || goos == "linux")

	if supported != expectedSupport {
		t.Errorf("expected platform support %v for %s, got %v", expectedSupport, goos, supported)
	}
}

// TestParseBrewVersion tests brew version parsing.
func TestParseBrewVersion(t *testing.T) {
	tests := []struct {
		name     string
		output   []byte
		expected string
	}{
		{
			name:     "standard_format",
			output:   []byte("Homebrew 4.1.20\nHomebrew/homebrew-core (git revision abc123)\n"),
			expected: "4.1.20",
		},
		{
			name:     "simple_format",
			output:   []byte("Homebrew 3.6.0"),
			expected: "3.6.0",
		},
		{
			name:     "with_trailing_newline",
			output:   []byte("Homebrew 4.0.0\n"),
			expected: "4.0.0",
		},
		{
			name:     "empty_output",
			output:   []byte(""),
			expected: "",
		},
		{
			name:     "malformed_output",
			output:   []byte("Some other text"),
			expected: "",
		},
		{
			name:     "version_with_patch",
			output:   []byte("Homebrew 4.1.20-12-g1234567\n"),
			expected: "4.1.20-12-g1234567",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseBrewVersion(tt.output)
			if result != tt.expected {
				t.Errorf("expected '%s', got '%s'", tt.expected, result)
			}
		})
	}
}

// TestScoopManager_Name tests the Name method.
func TestScoopManager_Name(t *testing.T) {
	mgr := &ScoopManager{}
	if mgr.Name() != "scoop" {
		t.Errorf("expected name 'scoop', got '%s'", mgr.Name())
	}
}

// TestScoopManager_InstallationURL tests the InstallationURL method.
func TestScoopManager_InstallationURL(t *testing.T) {
	mgr := &ScoopManager{}
	expected := "https://scoop.sh"
	if mgr.InstallationURL() != expected {
		t.Errorf("expected URL '%s', got '%s'", expected, mgr.InstallationURL())
	}
}

// TestScoopManager_SupportedPlatforms tests the SupportedPlatforms method.
func TestScoopManager_SupportedPlatforms(t *testing.T) {
	mgr := &ScoopManager{}
	platforms := mgr.SupportedPlatforms()

	if len(platforms) != 1 || platforms[0] != "windows" {
		t.Errorf("expected ['windows'], got %v", platforms)
	}
}

// TestScoopManager_IsSupportedOnCurrentPlatform tests platform support checking.
func TestScoopManager_IsSupportedOnCurrentPlatform(t *testing.T) {
	mgr := &ScoopManager{}
	supported := mgr.IsSupportedOnCurrentPlatform()

	expectedSupport := runtime.GOOS == "windows"

	if supported != expectedSupport {
		t.Errorf("expected platform support %v for %s, got %v",
			expectedSupport, runtime.GOOS, supported)
	}
}

// TestParseScoopVersion tests scoop version parsing.
func TestParseScoopVersion(t *testing.T) {
	tests := []struct {
		name     string
		output   []byte
		expected string
	}{
		{
			name:     "simple_version",
			output:   []byte("v0.3.1"),
			expected: "v0.3.1",
		},
		{
			name:     "with_prefix_text",
			output:   []byte("Current Scoop version:\nv0.3.1"),
			expected: "v0.3.1",
		},
		{
			name:     "multiline_with_info",
			output:   []byte("Some info\nv0.2.4\nMore info"),
			expected: "v0.2.4",
		},
		{
			name:     "version_with_metadata",
			output:   []byte("v0.3.1-dev"),
			expected: "v0.3.1-dev",
		},
		{
			name:     "empty_output",
			output:   []byte(""),
			expected: "",
		},
		{
			name:     "no_version_marker",
			output:   []byte("Scoop is installed"),
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseScoopVersion(tt.output)
			if result != tt.expected {
				t.Errorf("expected '%s', got '%s'", tt.expected, result)
			}
		})
	}
}

// TestGetManager tests the GetManager factory function.
func TestGetManager(t *testing.T) {
	tests := []struct {
		name        string
		managerName string
		wantErr     bool
		wantType    string
	}{
		{
			name:        "brew",
			managerName: "brew",
			wantErr:     runtime.GOOS != "darwin" && runtime.GOOS != "linux",
			wantType:    "*tools.BrewManager",
		},
		{
			name:        "scoop",
			managerName: "scoop",
			wantErr:     runtime.GOOS != "windows",
			wantType:    "*tools.ScoopManager",
		},
		{
			name:        "unknown_manager",
			managerName: "unknown",
			wantErr:     true,
		},
		{
			name:        "empty_name",
			managerName: "",
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mgr, err := GetManager(tt.managerName)

			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error, got nil")
				}
				if mgr != nil {
					t.Errorf("expected nil manager on error, got %v", mgr)
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if mgr == nil {
				t.Error("expected non-nil manager, got nil")
				return
			}

			if mgr.Name() != tt.managerName {
				t.Errorf("expected manager name '%s', got '%s'", tt.managerName, mgr.Name())
			}
		})
	}
}

// TestGetManager_PlatformValidation tests that GetManager enforces platform requirements.
func TestGetManager_PlatformValidation(t *testing.T) {
	// Test that getting a manager for an unsupported platform returns an error
	var testManager string
	var supportedPlatforms []string

	switch runtime.GOOS {
	case "darwin", "linux":
		// On macOS/Linux, scoop should fail
		testManager = "scoop"
		supportedPlatforms = []string{"windows"}
	case "windows":
		// On Windows, brew should fail
		testManager = "brew"
		supportedPlatforms = []string{"darwin", "linux"}
	default:
		t.Skip("unknown platform")
	}

	mgr, err := GetManager(testManager)
	if err == nil {
		t.Errorf("expected error for unsupported manager '%s' on %s, got nil",
			testManager, runtime.GOOS)
	}
	if mgr != nil {
		t.Errorf("expected nil manager for unsupported platform, got %v", mgr)
	}

	// Error message should mention platform
	if err != nil {
		errMsg := err.Error()
		if !strings.Contains(errMsg, runtime.GOOS) && !strings.Contains(errMsg, "supported") {
			t.Errorf("error message should mention platform or support: %s", errMsg)
		}
	}

	_ = supportedPlatforms // Used in test logic above
}

// TestGetAllManagers tests platform-specific manager listing.
func TestGetAllManagers(t *testing.T) {
	managers := GetAllManagers()

	switch runtime.GOOS {
	case "darwin", "linux":
		if len(managers) != 1 {
			t.Errorf("expected 1 manager on %s, got %d", runtime.GOOS, len(managers))
		}
		if len(managers) > 0 && managers[0].Name() != "brew" {
			t.Errorf("expected brew on %s, got %s", runtime.GOOS, managers[0].Name())
		}
	case "windows":
		if len(managers) != 1 {
			t.Errorf("expected 1 manager on windows, got %d", len(managers))
		}
		if len(managers) > 0 && managers[0].Name() != "scoop" {
			t.Errorf("expected scoop on windows, got %s", managers[0].Name())
		}
	default:
		if len(managers) != 0 {
			t.Errorf("expected 0 managers on unknown platform %s, got %d",
				runtime.GOOS, len(managers))
		}
	}
}

// TestGetPackageManagerStatus tests status retrieval.
func TestGetPackageManagerStatus(t *testing.T) {
	var validManager string
	var invalidManager string

	switch runtime.GOOS {
	case "darwin", "linux":
		validManager = "brew"
		invalidManager = "scoop"
	case "windows":
		validManager = "scoop"
		invalidManager = "brew"
	default:
		t.Skip("unknown platform")
	}

	// Test valid manager
	t.Run("valid_manager", func(t *testing.T) {
		status, err := GetPackageManagerStatus(validManager)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if status == nil {
			t.Fatal("expected non-nil status")
		}
		if status.Name != validManager {
			t.Errorf("expected name '%s', got '%s'", validManager, status.Name)
		}
		if !status.SupportedHere {
			t.Error("expected SupportedHere to be true")
		}
		if status.InstallationURL == "" {
			t.Error("expected non-empty installation URL")
		}
		if len(status.PlatformSupport) == 0 {
			t.Error("expected non-empty platform support list")
		}
		// Available status depends on whether the manager is actually installed
		// We don't assert on that since it's environment-dependent
	})

	// Test invalid manager for current platform
	t.Run("invalid_manager_for_platform", func(t *testing.T) {
		status, err := GetPackageManagerStatus(invalidManager)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if status == nil {
			t.Fatal("expected non-nil status")
		}
		if status.SupportedHere {
			t.Error("expected SupportedHere to be false for unsupported platform")
		}
		if status.Available {
			t.Error("expected Available to be false for unsupported platform")
		}
	})

	// Test unknown manager
	t.Run("unknown_manager", func(t *testing.T) {
		status, err := GetPackageManagerStatus("nonexistent")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if status == nil {
			t.Fatal("expected non-nil status")
		}
		if status.SupportedHere {
			t.Error("expected SupportedHere to be false for unknown manager")
		}
		if status.DetectionError == nil {
			t.Error("expected DetectionError to be set for unknown manager")
		}
	})
}

// TestGetAllPackageManagerStatuses tests retrieving all statuses.
func TestGetAllPackageManagerStatuses(t *testing.T) {
	statuses := GetAllPackageManagerStatuses()

	switch runtime.GOOS {
	case "darwin", "linux":
		if len(statuses) != 1 {
			t.Errorf("expected 1 status on %s, got %d", runtime.GOOS, len(statuses))
		}
		if len(statuses) > 0 {
			if statuses[0].Name != "brew" {
				t.Errorf("expected brew status, got %s", statuses[0].Name)
			}
			if !statuses[0].SupportedHere {
				t.Error("expected brew to be supported on current platform")
			}
		}
	case "windows":
		if len(statuses) != 1 {
			t.Errorf("expected 1 status on windows, got %d", len(statuses))
		}
		if len(statuses) > 0 {
			if statuses[0].Name != "scoop" {
				t.Errorf("expected scoop status, got %s", statuses[0].Name)
			}
			if !statuses[0].SupportedHere {
				t.Error("expected scoop to be supported on windows")
			}
		}
	default:
		if len(statuses) != 0 {
			t.Errorf("expected 0 statuses on unknown platform, got %d", len(statuses))
		}
	}
}

// TestBrewManager_IsAvailable tests availability detection (environment-dependent).
func TestBrewManager_IsAvailable(t *testing.T) {
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		t.Skip("brew not supported on this platform")
	}

	mgr := &BrewManager{}
	available := mgr.IsAvailable()

	// We can't assert the specific value since it depends on the environment,
	// but we can verify the method doesn't panic and returns a bool
	t.Logf("brew available: %v", available)
}

// TestScoopManager_IsAvailable tests availability detection (environment-dependent).
func TestScoopManager_IsAvailable(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("scoop not supported on this platform")
	}

	mgr := &ScoopManager{}
	available := mgr.IsAvailable()

	// We can't assert the specific value since it depends on the environment,
	// but we can verify the method doesn't panic and returns a bool
	t.Logf("scoop available: %v", available)
}

// TestBrewManager_Version tests version retrieval (environment-dependent).
func TestBrewManager_Version(t *testing.T) {
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		t.Skip("brew not supported on this platform")
	}

	mgr := &BrewManager{}
	if !mgr.IsAvailable() {
		t.Skip("brew not installed")
	}

	version, err := mgr.Version()
	if err != nil {
		t.Errorf("unexpected error getting brew version: %v", err)
	}
	if version == "" {
		t.Error("expected non-empty version string")
	}
	t.Logf("brew version: %s", version)
}

// TestScoopManager_Version tests version retrieval (environment-dependent).
func TestScoopManager_Version(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("scoop not supported on this platform")
	}

	mgr := &ScoopManager{}
	if !mgr.IsAvailable() {
		t.Skip("scoop not installed")
	}

	version, err := mgr.Version()
	if err != nil {
		t.Errorf("unexpected error getting scoop version: %v", err)
	}
	if version == "" {
		t.Error("expected non-empty version string")
	}
	t.Logf("scoop version: %s", version)
}

// TestBrewLocation_String tests BrewLocation string representation.
func TestBrewLocation_String(t *testing.T) {
	tests := []struct {
		loc      BrewLocation
		expected string
	}{
		{BrewNotFound, "not_found"},
		{BrewSystemAppleSilicon, "system_apple_silicon"},
		{BrewSystemIntel, "system_intel"},
		{BrewSystemLinux, "system_linux"},
		{BrewUserLocal, "user_local"},
		{BrewCustom, "custom"},
		{BrewLocation(999), "unknown"}, // Invalid value
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := tt.loc.String()
			if result != tt.expected {
				t.Errorf("expected '%s', got '%s'", tt.expected, result)
			}
		})
	}
}

// TestClassifyBrewPath tests brew path classification logic.
func TestClassifyBrewPath(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		expected BrewLocation
	}{
		{
			name:     "apple_silicon",
			path:     "/opt/homebrew/bin/brew",
			expected: BrewSystemAppleSilicon,
		},
		{
			name:     "intel_mac",
			path:     "/usr/local/bin/brew",
			expected: BrewSystemIntel,
		},
		{
			name:     "linux_standard",
			path:     "/home/linuxbrew/.linuxbrew/bin/brew",
			expected: BrewSystemLinux,
		},
		{
			name:     "user_local",
			path:     "/Users/dave/homebrew-local/bin/brew",
			expected: BrewUserLocal,
		},
		{
			name:     "user_local_linux",
			path:     "/home/dave/homebrew-local/bin/brew",
			expected: BrewUserLocal,
		},
		{
			name:     "custom_path",
			path:     "/custom/location/brew",
			expected: BrewCustom,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := classifyBrewPath(tt.path)
			if result != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected.String(), result.String())
			}
		})
	}
}

// TestGetBrewPrefix tests prefix generation for different brew locations.
func TestGetBrewPrefix(t *testing.T) {
	tests := []struct {
		name     string
		loc      BrewLocation
		expected string
	}{
		{
			name:     "apple_silicon",
			loc:      BrewSystemAppleSilicon,
			expected: "/opt/homebrew",
		},
		{
			name:     "intel",
			loc:      BrewSystemIntel,
			expected: "/usr/local",
		},
		{
			name:     "linux",
			loc:      BrewSystemLinux,
			expected: "/home/linuxbrew/.linuxbrew",
		},
		{
			name:     "user_local",
			loc:      BrewUserLocal,
			expected: "", // Will be $HOME/homebrew-local, but we can't predict $HOME in tests
		},
		{
			name:     "not_found",
			loc:      BrewNotFound,
			expected: "",
		},
		{
			name:     "custom",
			loc:      BrewCustom,
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetBrewPrefix(tt.loc)
			if tt.loc == BrewUserLocal {
				// For user-local, just verify it contains homebrew-local
				if !strings.Contains(result, "homebrew-local") {
					t.Errorf("expected path containing 'homebrew-local', got '%s'", result)
				}
			} else {
				if result != tt.expected {
					t.Errorf("expected '%s', got '%s'", tt.expected, result)
				}
			}
		})
	}
}

// TestIsUserLocalBrew tests user-local brew detection.
func TestIsUserLocalBrew(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		expected bool
	}{
		{
			name:     "user_local_mac",
			path:     "/Users/dave/homebrew-local/bin/brew",
			expected: true,
		},
		{
			name:     "user_local_linux",
			path:     "/home/dave/homebrew-local/bin/brew",
			expected: true,
		},
		{
			name:     "system_apple_silicon",
			path:     "/opt/homebrew/bin/brew",
			expected: false,
		},
		{
			name:     "system_intel",
			path:     "/usr/local/bin/brew",
			expected: false,
		},
		{
			name:     "system_linux",
			path:     "/home/linuxbrew/.linuxbrew/bin/brew",
			expected: false,
		},
		{
			name:     "empty_path",
			path:     "",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsUserLocalBrew(tt.path)
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

// TestFileExists tests the fileExists helper function.
func TestFileExists(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		expected bool
	}{
		{
			name:     "empty_path",
			path:     "",
			expected: false,
		},
		{
			name:     "nonexistent_file",
			path:     "/nonexistent/path/to/nowhere",
			expected: false,
		},
		// Note: Testing existing files would be environment-specific
		// so we only test the failure cases here
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := fileExists(tt.path)
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

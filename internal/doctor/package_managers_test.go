package doctor

import (
	"runtime"
	"testing"
)

func TestLoadPackageManagersConfig(t *testing.T) {
	config, err := LoadPackageManagersConfig()
	if err != nil {
		t.Fatalf("Failed to load package managers config: %v", err)
	}

	if config.Version == "" {
		t.Error("Config version is empty")
	}

	if len(config.PackageManagers) == 0 {
		t.Error("No package managers defined")
	}

	// Verify expected package managers are present
	expectedPMs := []string{"bun", "mise", "go-install", "uv", "npm", "cargo"}
	foundPMs := make(map[string]bool)
	for _, pm := range config.PackageManagers {
		foundPMs[pm.Name] = true
	}

	for _, expected := range expectedPMs {
		if !foundPMs[expected] {
			t.Errorf("Expected package manager %s not found", expected)
		}
	}
}

func TestGetBoolForPlatform(t *testing.T) {
	tests := []struct {
		name     string
		value    interface{}
		platform string
		expected bool
	}{
		{
			name:     "simple bool true",
			value:    true,
			platform: "darwin",
			expected: true,
		},
		{
			name:     "simple bool false",
			value:    false,
			platform: "darwin",
			expected: false,
		},
		{
			name: "platform map darwin true",
			value: map[string]interface{}{
				"darwin": true,
				"linux":  false,
			},
			platform: "darwin",
			expected: true,
		},
		{
			name: "platform map darwin false",
			value: map[string]interface{}{
				"darwin": false,
				"linux":  true,
			},
			platform: "darwin",
			expected: false,
		},
		{
			name: "platform not in map",
			value: map[string]interface{}{
				"linux": true,
			},
			platform: "darwin",
			expected: false,
		},
		{
			name:     "nil value",
			value:    nil,
			platform: "darwin",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetBoolForPlatform(tt.value, tt.platform)
			if result != tt.expected {
				t.Errorf("GetBoolForPlatform() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestGetStringForPlatform(t *testing.T) {
	tests := []struct {
		name     string
		value    interface{}
		platform string
		expected string
	}{
		{
			name:     "simple string",
			value:    "test-command",
			platform: "darwin",
			expected: "test-command",
		},
		{
			name: "platform map darwin",
			value: map[string]interface{}{
				"darwin": "darwin-command",
				"linux":  "linux-command",
			},
			platform: "darwin",
			expected: "darwin-command",
		},
		{
			name: "platform not in map",
			value: map[string]interface{}{
				"linux": "linux-command",
			},
			platform: "darwin",
			expected: "",
		},
		{
			name:     "nil value",
			value:    nil,
			platform: "darwin",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetStringForPlatform(tt.value, tt.platform)
			if result != tt.expected {
				t.Errorf("GetStringForPlatform() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestPackageManager_RequiresSudoOnPlatform(t *testing.T) {
	tests := []struct {
		name         string
		pm           PackageManager
		platform     string
		expectedSudo bool
	}{
		{
			name: "brew requires sudo on darwin",
			pm: PackageManager{
				Name: "brew",
				RequiresSudo: map[string]interface{}{
					"darwin": true,
					"linux":  true,
				},
			},
			platform:     "darwin",
			expectedSudo: true,
		},
		{
			name: "bun does not require sudo",
			pm: PackageManager{
				Name:         "bun",
				RequiresSudo: false,
			},
			platform:     "darwin",
			expectedSudo: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.pm.RequiresSudoOnPlatform(tt.platform)
			if result != tt.expectedSudo {
				t.Errorf("RequiresSudoOnPlatform() = %v, want %v", result, tt.expectedSudo)
			}
		})
	}
}

func TestPackageManager_SupportsPlatform(t *testing.T) {
	tests := []struct {
		name     string
		pm       PackageManager
		platform string
		expected bool
	}{
		{
			name: "supports darwin",
			pm: PackageManager{
				Name:      "bun",
				Platforms: []string{"darwin", "linux", "windows"},
			},
			platform: "darwin",
			expected: true,
		},
		{
			name: "does not support windows",
			pm: PackageManager{
				Name:      "mise",
				Platforms: []string{"darwin", "linux"},
			},
			platform: "windows",
			expected: false,
		},
		{
			name: "no platform restriction",
			pm: PackageManager{
				Name:      "universal",
				Platforms: []string{},
			},
			platform: "darwin",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.pm.SupportsPlatform(tt.platform)
			if result != tt.expected {
				t.Errorf("SupportsPlatform() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestPackageManager_SupportsLanguage(t *testing.T) {
	tests := []struct {
		name     string
		pm       PackageManager
		language string
		expected bool
	}{
		{
			name: "supports go",
			pm: PackageManager{
				Name:             "go-install",
				SafeForLanguages: []string{"go"},
			},
			language: "go",
			expected: true,
		},
		{
			name: "supports all languages",
			pm: PackageManager{
				Name:             "bun",
				SafeForLanguages: []string{"all"},
			},
			language: "python",
			expected: true,
		},
		{
			name: "does not support rust",
			pm: PackageManager{
				Name:             "uv",
				SafeForLanguages: []string{"python"},
			},
			language: "rust",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.pm.SupportsLanguage(tt.language)
			if result != tt.expected {
				t.Errorf("SupportsLanguage() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestDetectPackageManager(t *testing.T) {
	// Test with a command that should exist (go)
	pm := PackageManager{
		Name:             "go",
		DetectionCommand: "go version",
	}

	installed, version := DetectPackageManager(&pm)

	// This test is environment-dependent - just verify it doesn't crash
	if installed && version == "" {
		t.Log("go is installed but version detection returned empty")
	}

	// Test with a command that definitely doesn't exist
	fakePM := PackageManager{
		Name:             "nonexistent",
		DetectionCommand: "nonexistent-command-12345 --version",
	}

	installed, version = DetectPackageManager(&fakePM)
	if installed {
		t.Error("Expected nonexistent command to not be detected")
	}
	if version != "" {
		t.Error("Expected empty version for nonexistent command")
	}
}

func TestDetectAllPackageManagers(t *testing.T) {
	config, err := LoadPackageManagersConfig()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	detected := DetectAllPackageManagers(config)

	// Should only include package managers for current platform
	platform := runtime.GOOS
	for _, pm := range detected {
		if !pm.SupportsPlatform(platform) {
			t.Errorf("Package manager %s does not support platform %s", pm.Name, platform)
		}
	}

	// At least some package managers should be detected
	// (This is environment-dependent, so we just log results)
	installedCount := 0
	for _, pm := range detected {
		if pm.Installed {
			installedCount++
			t.Logf("Detected %s: %s", pm.Name, pm.Version)
		}
	}
	t.Logf("Found %d/%d package managers installed", installedCount, len(detected))
}

func TestGetRecommendedPackageManagers(t *testing.T) {
	config, err := LoadPackageManagersConfig()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	recommended := GetRecommendedPackageManagers(config)

	// Should only include recommended package managers for current platform
	platform := runtime.GOOS
	for _, pm := range recommended {
		if !pm.IsRecommendedOnPlatform(platform) {
			t.Errorf("Package manager %s is not recommended on platform %s", pm.Name, platform)
		}
	}

	// Should have at least some recommendations
	if len(recommended) == 0 {
		t.Error("Expected at least some recommended package managers")
	}

	t.Logf("Recommended package managers for %s: %d", platform, len(recommended))
	for _, pm := range recommended {
		t.Logf("  - %s (installed: %v)", pm.Name, pm.Installed)
	}
}

func TestGetSafePackageManagersForLanguage(t *testing.T) {
	config, err := LoadPackageManagersConfig()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	tests := []struct {
		language string
		minCount int // Minimum expected safe package managers
	}{
		{"go", 1},     // At least go-install
		{"python", 1}, // At least uv or pip
		{"all", 1},    // Universal package managers
	}

	for _, tt := range tests {
		t.Run(tt.language, func(t *testing.T) {
			safe := GetSafePackageManagersForLanguage(config, tt.language)

			if len(safe) < tt.minCount {
				t.Errorf("Expected at least %d safe package managers for %s, got %d",
					tt.minCount, tt.language, len(safe))
			}

			// Verify all returned package managers support the language
			platform := runtime.GOOS
			for _, pm := range safe {
				if !pm.SupportsLanguage(tt.language) {
					t.Errorf("Package manager %s does not support language %s", pm.Name, tt.language)
				}

				// Verify they don't require sudo
				if pm.RequiresSudoOnPlatform(platform) {
					t.Errorf("Package manager %s requires sudo on %s (should be filtered out)",
						pm.Name, platform)
				}
			}

			t.Logf("Safe package managers for %s: %d", tt.language, len(safe))
			for _, pm := range safe {
				t.Logf("  - %s (installed: %v)", pm.Name, pm.Installed)
			}
		})
	}
}

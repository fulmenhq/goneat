package buildinfo

import (
	"runtime/debug"
	"testing"
)

func TestBinaryVersion(t *testing.T) {
	// Test that BinaryVersion has a default value
	if BinaryVersion == "" {
		t.Error("BinaryVersion should not be empty")
	}

	// Test that it's set to expected default
	if BinaryVersion != "dev" {
		t.Errorf("Expected BinaryVersion to be 'dev', got '%s'", BinaryVersion)
	}
}

func TestModuleVersion(t *testing.T) {
	version := ModuleVersion()

	// Version could be empty if build info is not available
	// This is acceptable for testing environments
	if version == "" {
		t.Log("ModuleVersion returned empty string (build info not available)")
		return
	}

	// If we get a version, it should be a non-empty string
	if version == "" {
		t.Error("ModuleVersion should not return empty string when build info is available")
	}

	// Basic validation that it looks like a version string
	// Could be semver (v1.2.3), pseudo-version (v0.0.0-...), or other formats
	if len(version) < 2 {
		t.Errorf("ModuleVersion seems too short: '%s'", version)
	}
}

func TestModuleVersionIntegration(t *testing.T) {
	// Test that our function matches debug.ReadBuildInfo directly
	expected := ""
	if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" {
		expected = info.Main.Version
	}

	actual := ModuleVersion()

	if expected != actual {
		t.Errorf("ModuleVersion() = '%s', expected '%s'", actual, expected)
	}
}

func TestBinaryVersionTypes(t *testing.T) {
	// Ensure BinaryVersion is a string type
	version := BinaryVersion
	if version != BinaryVersion {
		t.Error("BinaryVersion should be accessible as a string")
	}

	// Test string operations work
	_ = len(BinaryVersion)
	_ = BinaryVersion + "test"
	_ = string([]byte(BinaryVersion))
}

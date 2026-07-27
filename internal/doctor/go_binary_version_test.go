package doctor

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"
	"testing"

	"github.com/fulmenhq/goneat/pkg/versioning"
)

func TestGoBinaryVersion(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		info *debug.BuildInfo
		want string
	}{
		{
			name: "module version",
			info: &debug.BuildInfo{
				Main: debug.Module{
					Path:    "github.com/google/go-licenses/v2",
					Version: "v2.0.1",
				},
			},
			want: "v2.0.1",
		},
		{
			name: "trim whitespace",
			info: &debug.BuildInfo{
				Main: debug.Module{
					Path:    "github.com/google/go-licenses",
					Version: " v1.6.0 ",
				},
			},
			want: "v1.6.0",
		},
		{
			name: "development build",
			info: &debug.BuildInfo{
				Main: debug.Module{
					Path:    "github.com/example/tool",
					Version: "(devel)",
				},
			},
		},
		{
			name: "missing build info",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := goBinaryVersion(tt.info); got != tt.want {
				t.Fatalf("goBinaryVersion() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestDetectGoBinaryVersionRejectsNonGoBinary(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "not-a-go-binary")
	if err := os.WriteFile(path, []byte("not a Go executable"), 0o600); err != nil {
		t.Fatalf("write non-Go fixture: %v", err)
	}
	if got := detectGoBinaryVersion(path); got != "" {
		t.Fatalf("detectGoBinaryVersion() = %q, want empty for a non-Go binary", got)
	}
}

func TestGoVersionProbeHelper(t *testing.T) {
	output := os.Getenv("GONEAT_TEST_GO_VERSION_PROBE_OUTPUT")
	if output == "" {
		return
	}
	fmt.Fprintln(os.Stdout, output) //nolint:errcheck // subprocess test fixture
}

func TestDetectGoToolVersionAtPathPrefersConfiguredCLI(t *testing.T) {
	executable, err := os.Executable()
	if err != nil {
		t.Fatalf("resolve test executable: %v", err)
	}
	t.Setenv("GONEAT_TEST_GO_VERSION_PROBE_OUTPUT", "fixture version 9.8.7")

	buildInfoRead := false
	tool := Tool{
		Name:          "fixture",
		Kind:          "go",
		DetectCommand: "fixture -test.run=^TestGoVersionProbeHelper$",
	}

	got := detectGoToolVersionAtPathWith(tool, executable, func(string) string {
		buildInfoRead = true
		return "v2.0.1"
	})
	if got != "9.8.7" {
		t.Fatalf("detectGoToolVersionAtPathWith() = %q, want CLI version 9.8.7", got)
	}
	if buildInfoRead {
		t.Fatal("build metadata reader called despite valid configured CLI version")
	}
}

func TestDetectGoToolVersionAtPathFallsBackFromInvalidCLI(t *testing.T) {
	executable, err := os.Executable()
	if err != nil {
		t.Fatalf("resolve test executable: %v", err)
	}
	t.Setenv("GONEAT_TEST_GO_VERSION_PROBE_OUTPUT", "Usage: fixture [command]")

	buildInfoRead := false
	tool := Tool{
		Name:          "fixture",
		Kind:          "go",
		DetectCommand: "fixture -test.run=^TestGoVersionProbeHelper$",
	}

	got := detectGoToolVersionAtPathWith(tool, executable, func(path string) string {
		buildInfoRead = true
		if path != executable {
			t.Fatalf("build metadata path = %q, want %q", path, executable)
		}
		return "v2.0.1"
	})
	if got != "v2.0.1" {
		t.Fatalf("detectGoToolVersionAtPathWith() = %q, want build metadata version v2.0.1", got)
	}
	if !buildInfoRead {
		t.Fatal("build metadata reader was not called after invalid CLI output")
	}
}

func TestCheckGoToolInstallationOutsidePathIncludesVersion(t *testing.T) {
	goBin := t.TempDir()
	toolPath := filepath.Join(goBin, "goneat-version-fixture")
	if err := os.WriteFile(toolPath, []byte("fixture"), 0o700); err != nil {
		t.Fatalf("write tool fixture: %v", err)
	}
	t.Setenv("GOBIN", goBin)

	status := checkGoToolInstallationWith(Tool{
		Name: "goneat-version-fixture",
		Kind: "go",
	}, func(_ Tool, path string) string {
		if path != toolPath {
			t.Fatalf("detector path = %q, want %q", path, toolPath)
		}
		return "v2.0.1"
	})

	if status.Present {
		t.Fatal("outside-PATH tool reported present")
	}
	if status.Version != "v2.0.1" {
		t.Fatalf("outside-PATH version = %q, want v2.0.1", status.Version)
	}
	if !strings.Contains(status.Instructions, toolPath) || !strings.Contains(status.Instructions, "PATH") {
		t.Fatalf("outside-PATH instructions are not actionable: %q", status.Instructions)
	}
}

func TestVerifyGoInstallResultRejectsShadowedActiveBinary(t *testing.T) {
	activePath := filepath.Join(t.TempDir(), "go-licenses")
	installedPath := filepath.Join(t.TempDir(), "go-licenses")
	for _, path := range []string{activePath, installedPath} {
		if err := os.WriteFile(path, []byte("fixture"), 0o700); err != nil {
			t.Fatalf("write binary fixture %s: %v", path, err)
		}
	}

	tool := Tool{
		Name: "go-licenses",
		Kind: "go",
		VersionPolicy: versioning.Policy{
			Scheme:             versioning.SchemeSemverLegacy,
			MinimumVersion:     "2.0.1",
			RecommendedVersion: "2.0.1",
		},
	}
	status := Status{
		Name:      tool.Name,
		Present:   true,
		Installed: true,
		Version:   "v1.6.0",
	}
	applyVersionPolicy(tool, &status)

	verifyGoInstallResult(tool, &status, activePath, installedPath)

	if status.Error == nil {
		t.Fatal("shadowed active binary accepted after install")
	}
	message := status.Error.Error()
	for _, want := range []string{installedPath, activePath, "v1.6.0", "below minimum"} {
		if !strings.Contains(message, want) {
			t.Fatalf("error %q does not contain %q", message, want)
		}
	}
	for _, want := range []string{filepath.Dir(installedPath), filepath.Dir(activePath), "PATH"} {
		if !strings.Contains(status.Instructions, want) {
			t.Fatalf("instructions %q do not contain %q", status.Instructions, want)
		}
	}
}

func TestValidateUpgradeResult(t *testing.T) {
	tool := Tool{
		Name: "fixture",
		VersionPolicy: versioning.Policy{
			Scheme:             versioning.SchemeSemverLegacy,
			MinimumVersion:     "2.0.0",
			RecommendedVersion: "2.1.0",
		},
	}

	tests := []struct {
		name    string
		present bool
		version string
		wantErr string
	}{
		{name: "missing active binary", version: "v2.1.0", wantErr: "not active in PATH"},
		{name: "below minimum", present: true, version: "v1.9.0", wantErr: "below minimum"},
		{name: "below recommended", present: true, version: "v2.0.0", wantErr: "below recommended"},
		{name: "meets intended policy", present: true, version: "v2.1.0"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status := Status{Name: tool.Name, Present: tt.present, Version: tt.version}
			applyVersionPolicy(tool, &status)
			err := ValidateUpgradeResult(tool, status)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("ValidateUpgradeResult() unexpected error: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("ValidateUpgradeResult() error = %v, want substring %q", err, tt.wantErr)
			}
		})
	}
}

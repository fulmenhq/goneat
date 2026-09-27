package assess

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fulmenhq/goneat/internal/schema"
)

// An invalid known value next to valid siblings.
const invalidKnownAssessConfig = `version: 1
lint:
  yamllint:
    enabled: false
  shell:
    shfmt:
      enabled: "sometimes"
`

func requireAssessConfigError(t *testing.T, err error, wantSubstrings ...string) *AssessConfigError {
	t.Helper()
	var cfgErr *AssessConfigError
	if !errors.As(err, &cfgErr) {
		t.Fatalf("expected *AssessConfigError, got %T: %v", err, err)
	}
	if !strings.HasSuffix(cfgErr.Path, filepath.Join(".goneat", "assess.yaml")) {
		t.Fatalf("error must name the config path, got %q", cfgErr.Path)
	}
	for _, want := range wantSubstrings {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("error %q must mention %q", err.Error(), want)
		}
	}
	return cfgErr
}

func TestLoadAssessOverrides_AbsentUsesDefaults(t *testing.T) {
	overrides, err := loadAssessOverrides(t.TempDir())
	if overrides != nil || err != nil {
		t.Fatalf("absent config must return (nil, nil), got %+v, %v", overrides, err)
	}
}

func TestLoadAssessOverrides_InvalidKnownValueFailsAndStaysCached(t *testing.T) {
	repo := t.TempDir()
	writeAssessYAML(t, repo, invalidKnownAssessConfig)

	_, err := loadAssessOverrides(repo)
	cfgErr := requireAssessConfigError(t, err, "shfmt", "enabled")
	if len(cfgErr.Problems) == 0 {
		t.Fatalf("validation failure must carry key-level problems")
	}

	// Repeated loads (other runners in the same run) must not turn green.
	for i := 0; i < 3; i++ {
		overrides, again := loadAssessOverrides(repo)
		if overrides != nil || again == nil || again.Error() != err.Error() {
			t.Fatalf("load %d: cached error must persist, got %+v, %v", i, overrides, again)
		}
	}
}

func TestLoadAssessOverrides_MalformedYAMLFails(t *testing.T) {
	repo := t.TempDir()
	writeAssessYAML(t, repo, "lint:\n  yamllint: [unterminated\n")
	_, err := loadAssessOverrides(repo)
	_ = requireAssessConfigError(t, err, "parse")
}

func TestLoadAssessOverrides_UnreadableFileFails(t *testing.T) {
	repo := t.TempDir()
	// A directory where the file should be is present but cannot be read as
	// a file, on every platform and regardless of privileges.
	if err := os.MkdirAll(filepath.Join(repo, ".goneat", "assess.yaml"), 0o755); err != nil {
		t.Fatal(err)
	}
	_, err := loadAssessOverrides(repo)
	_ = requireAssessConfigError(t, err, "read")
}

// A dangling .goneat/assess.yaml symlink is a present, unreadable config,
// not an absent one.
func TestLoadAssessOverrides_DanglingSymlinkFails(t *testing.T) {
	repo := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repo, ".goneat"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("nonexistent.yaml", filepath.Join(repo, ".goneat", "assess.yaml")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	_, err := loadAssessOverrides(repo)
	_ = requireAssessConfigError(t, err, "read")
}

func TestLoadAssessOverrides_SchemaEngineErrorFails(t *testing.T) {
	orig := validateAssessConfig
	validateAssessConfig = func(map[string]any) (*schema.Result, error) {
		return nil, errors.New("engine unavailable")
	}
	t.Cleanup(func() { validateAssessConfig = orig })

	repo := t.TempDir()
	writeAssessYAML(t, repo, "lint:\n  yamllint:\n    enabled: false\n")
	_, err := loadAssessOverrides(repo)
	_ = requireAssessConfigError(t, err, "schema validation", "engine unavailable")
}

// The consuming runners report the config error as a failed category; they
// do not run under defaults.
func TestAssessRunners_InvalidConfigFailsConsumingCategories(t *testing.T) {
	repo := t.TempDir()
	writeAssessYAML(t, repo, `version: 1
lint:
  yamllint:
    enabled: false
typecheck:
  enabled: "yes please"
`)
	cfg := DefaultAssessmentConfig()
	runners := map[string]AssessmentRunner{
		"lint":      NewLintAssessmentRunner(),
		"typecheck": NewTypecheckAssessmentRunner(),
	}
	for name, runner := range runners {
		res, err := runner.Assess(context.Background(), repo, cfg)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if res.Success || !strings.Contains(res.Error, "invalid assess configuration") || !strings.Contains(res.Error, "typecheck") {
			t.Fatalf("%s: expected a failed result naming the config key, got success=%v error=%q", name, res.Success, res.Error)
		}
	}

	issues, err := runCargoClippyLint(repo, cfg)
	if issues != nil || err == nil {
		t.Fatalf("clippy settings come from assess.yaml and must fail too, got %v, %v", issues, err)
	}
}

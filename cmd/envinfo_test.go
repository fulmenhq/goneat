package cmd

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

// helper to run root with args and capture stdout/stderr
func execRoot(t *testing.T, args []string) (string, error) {
	t.Helper()

	// Reset global assess flags to avoid cross-test bleed
	assessMode, assessNoOp, assessCheck, assessFix = "check", false, false, false
	contentRoot = "docs"
	contentManifest = "docs/embed-manifest.yaml"
	contentTarget = ""
	contentJSON = false
	contentFormat = "pretty"
	contentPrintPaths = false
	contentNoDelete = false
	contentAllManifests = false
	contentAssetTypeOverride = ""
	contentContentTypesOverride = nil
	contentExcludePatternsOverride = nil
	contentManifestsValidate = false
	contentMigrateOutput = ""
	contentMigrateForce = false
	contentDryRun = false
	contentInitAssetType = ""
	contentInitRoot = ""
	contentInitTarget = ""
	contentInitTopic = ""
	contentInitOutput = ""
	contentInitInclude = nil
	contentInitExclude = nil
	contentInitOverwrite = false

	// Reset validate suite flags to avoid cross-test bleed
	validateSuiteDataRoot = ""
	validateSuiteSchemasRoot = ""
	validateSuiteManifestPath = ".goneat/schema-mappings.yaml"
	validateSuiteRefDirs = nil
	validateSuiteNoIgnore = false
	validateSuiteForceInclude = nil
	validateSuiteExclude = nil
	validateSuiteSkip = nil
	validateSuiteExpectFail = nil
	validateSuiteStrict = false
	validateSuiteEnableMeta = false
	validateSuiteMaxWorkers = runtime.NumCPU()
	validateSuiteTimeout = 3 * time.Minute
	validateSuiteFormat = "markdown"
	validateSuiteFailOnUnmapped = true

	// Create a fresh root command instance per test to prevent command tree pollution
	cmd := newRootCommand()
	registerSubcommands(cmd)

	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	// Reduce log noise to capture clean command output for JSON parsing
	full := append([]string{"--log-level", "error"}, args...)
	cmd.SetArgs(full)

	// Run from repository root so relative defaults (docs/, schemas/) resolve consistently
	cwd, _ := os.Getwd()
	if repo := findRepoRootFS(cwd); repo != "" {
		_ = os.Chdir(repo)
		defer func() { _ = os.Chdir(cwd) }()
	}

	t.Setenv("GONEAT_OFFLINE_SCHEMA_VALIDATION", "true")
	err := cmd.Execute()
	return buf.String(), err
}

// findRepoRootFS finds a parent directory containing .git
func findRepoRootFS(start string) string {
	dir := start
	for {
		if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}

func TestEnvinfo_JSON(t *testing.T) {
	out, err := execRoot(t, []string{"envinfo", "--json"})
	if err != nil {
		t.Fatalf("envinfo --json failed: %v\n%s", err, out)
	}
	// basic JSON validation
	var v map[string]interface{}
	if json.Unmarshal([]byte(out), &v) != nil {
		t.Fatalf("envinfo output is not valid JSON: %s", out)
	}
	if _, ok := v["system"]; !ok {
		t.Errorf("expected system key in envinfo JSON")
	}
}

func TestEnvinfo_DefaultConsole(t *testing.T) {
	out, err := execRoot(t, []string{"envinfo", "--no-color"})
	if err != nil {
		t.Fatalf("envinfo failed: %v\n%s", err, out)
	}
	if out == "" {
		t.Errorf("expected some console output for envinfo")
	}
}

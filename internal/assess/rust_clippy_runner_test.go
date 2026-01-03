package assess

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunCargoClippyLint_WorkspaceAddsFlag(t *testing.T) {
	repo := t.TempDir()
	cargoContent := `[workspace]
members = ["crates/*"]
resolver = "2"
`
	if err := os.WriteFile(filepath.Join(repo, "Cargo.toml"), []byte(cargoContent), 0o644); err != nil {
		t.Fatalf("write Cargo.toml: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(repo, "src"), 0o755); err != nil {
		t.Fatalf("mkdir src: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repo, "src", "lib.rs"), []byte("pub fn hi() {}"), 0o644); err != nil {
		t.Fatalf("write lib.rs: %v", err)
	}

	argsOut := filepath.Join(repo, "clippy-args.txt")
	script := "#!/usr/bin/env bash\n" +
		"set -euo pipefail\n" +
		"if [[ \"$1\" == \"clippy\" && \"${2:-}\" == \"--version\" ]]; then echo 'clippy 0.1.85'; exit 0; fi\n" +
		"if [[ \"$1\" == \"clippy\" ]]; then\n" +
		"  if [[ -n \"${CARGO_ARGS_OUT:-}\" ]]; then printf '%s\n' \"$@\" > \"$CARGO_ARGS_OUT\"; fi\n" +
		"  echo '{\"reason\":\"compiler-message\",\"message\":{\"message\":\"lint warning\",\"level\":\"warning\",\"spans\":[{\"file_name\":\"src/lib.rs\",\"line_start\":1,\"column_start\":1,\"is_primary\":true}]}}'\n" +
		"  exit 1\n" +
		"fi\n" +
		"if [[ \"$1\" == \"--version\" ]]; then echo 'cargo 1.75.0'; exit 0; fi\n" +
		"exit 0\n"

	writeFakeCargo(t, repo, script)
	t.Setenv("CARGO_ARGS_OUT", argsOut)

	cfg := DefaultAssessmentConfig()
	cfg.Mode = AssessmentModeCheck

	issues, err := runCargoClippyLint(repo, cfg)
	if err != nil {
		t.Fatalf("runCargoClippyLint error: %v", err)
	}
	if len(issues) != 1 {
		t.Fatalf("expected 1 issue, got %d", len(issues))
	}

	argsBytes, err := os.ReadFile(argsOut)
	if err != nil {
		t.Fatalf("read args out: %v", err)
	}
	args := strings.Fields(string(argsBytes))

	foundWorkspace := false
	foundFormat := false
	for _, a := range args {
		if a == "--workspace" {
			foundWorkspace = true
		}
		if a == "--message-format=json" {
			foundFormat = true
		}
	}
	if !foundWorkspace {
		t.Fatalf("expected --workspace in args, got %v", args)
	}
	if !foundFormat {
		t.Fatalf("expected --message-format=json in args, got %v", args)
	}
}

package assess

import (
	"os"
	"path/filepath"
	"testing"
)

// Go scanners need a Go module. Repositories in other languages must report
// them inapplicable rather than run them and fail.
func TestSecurityGoScannerApplicabilityByProject(t *testing.T) {
	for _, tt := range []struct {
		name  string
		files map[string]string
		goApp bool
	}{
		{"rust", map[string]string{"Cargo.toml": "[package]\nname = \"fixture\"\n", "src/lib.rs": ""}, false},
		{"typescript", map[string]string{"package.json": "{}", "tsconfig.json": "{}", "src/index.ts": ""}, false},
		{"python", map[string]string{"pyproject.toml": "[project]\nname = \"fixture\"\n", "fixture/__init__.py": ""}, false},
		{"docs_only", map[string]string{"README.md": ""}, false},
		{"go_root", map[string]string{"go.mod": "module fixture\n", "main.go": "package main\n"}, true},
		{"go_nested", map[string]string{"Cargo.toml": "", "tools/gen/go.mod": "module gen\n"}, true},
		{"go_only_in_node_modules", map[string]string{"package.json": "{}", "node_modules/dep/go.mod": "module dep\n"}, false},
		{"go_only_in_gitignored_dir", map[string]string{".gitignore": "/third_party/\n", "pyproject.toml": "", "third_party/x/go.mod": "module x\n"}, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			for name, content := range tt.files {
				path := filepath.Join(root, filepath.FromSlash(name))
				if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			_, admissions, errs := securityRegistry.selectAdmissions(DefaultAssessmentConfig(), NewSecurityAssessmentRunner(), root)
			if len(errs) != 0 {
				t.Fatalf("admission errors: %v", errs)
			}
			states := map[string]securityToolAdmission{}
			for _, a := range admissions {
				states[a.Tool] = a
			}
			for _, tool := range []string{"gosec", "govulncheck"} {
				a, ok := states[tool]
				if !ok {
					t.Fatalf("%s not evaluated: %+v", tool, admissions)
				}
				if inapplicable := a.State == "inapplicable"; inapplicable == tt.goApp {
					t.Fatalf("%s state %q (%s); want applicable=%v", tool, a.State, a.Reason, tt.goApp)
				}
			}
		})
	}
}

func TestSecurityGoScannerExplicitInNonGoProjectSkips(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "Cargo.toml"), []byte(""), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg := DefaultAssessmentConfig()
	cfg.SecurityTools = []string{"gosec", "govulncheck"}
	adapters, admissions, errs := securityRegistry.selectAdmissions(cfg, NewSecurityAssessmentRunner(), root)
	if len(adapters) != 0 || len(errs) != 0 {
		t.Fatalf("explicit Go scanners in a non-Go project must be a labeled skip: adapters=%d errs=%v", len(adapters), errs)
	}
	for _, a := range admissions {
		if (a.Tool == "gosec" || a.Tool == "govulncheck") && (a.State != "inapplicable" || !a.Explicit) {
			t.Fatalf("explicit inapplicable state lost: %+v", a)
		}
	}
}

func TestSecurityHasGoModuleWalkErrorIsApplicable(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing")
	if !NewSecurityAssessmentRunner().hasGoModule(missing, DefaultAssessmentConfig()) {
		t.Fatal("an unreadable root must not be treated as a non-Go project")
	}
}

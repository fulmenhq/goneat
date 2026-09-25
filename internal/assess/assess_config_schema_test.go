package assess

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	internalschema "github.com/fulmenhq/goneat/internal/schema"
	"github.com/fulmenhq/goneat/pkg/logger"
	pkgschema "github.com/fulmenhq/goneat/pkg/schema"
	"gopkg.in/yaml.v3"
)

const assessConfigSourceSchema = "../../schemas/config/v1.0.0/assess-config.yaml"

func decodeYAMLDoc(t *testing.T, doc string) map[string]any {
	t.Helper()
	var raw map[string]any
	if err := yaml.Unmarshal([]byte(doc), &raw); err != nil {
		t.Fatalf("decode fixture: %v", err)
	}
	return raw
}

// The schema itself must close the root: this validates raw, unstripped
// documents directly (no loader), against both the source schema and the
// embedded copy the binary uses.
func TestAssessConfigSchema_RootRejectsUnknownKeys(t *testing.T) {
	valid := `version: 1
lint:
  yamllint:
    enabled: false
  rust:
    clippy:
      all_targets: true
      targets: [x86_64-unknown-linux-gnu]
`
	withLegacyRust := valid + "rust: {}\n"

	sourceBytes, err := os.ReadFile(assessConfigSourceSchema)
	if err != nil {
		t.Fatalf("read source schema: %v", err)
	}

	validators := map[string]func(map[string]any) bool{
		"source": func(doc map[string]any) bool {
			res, err := pkgschema.ValidateFromBytes(sourceBytes, doc)
			if err != nil {
				t.Fatalf("validate against source schema: %v", err)
			}
			return res.Valid
		},
		"embedded": func(doc map[string]any) bool {
			res, err := internalschema.Validate(doc, "assess-config-v1.0.0")
			if err != nil {
				t.Fatalf("validate against embedded schema: %v", err)
			}
			return res.Valid
		},
	}
	for name, validate := range validators {
		t.Run(name, func(t *testing.T) {
			if !validate(decodeYAMLDoc(t, valid)) {
				t.Fatalf("supported config (including lint.rust.clippy) must validate")
			}
			if validate(decodeYAMLDoc(t, withLegacyRust)) {
				t.Fatalf("unknown root key rust: must be rejected by the schema")
			}
		})
	}
}

// Legacy top-level rust: must warn and be ignored without discarding the
// valid sections beside it.
func TestLoadAssessOverrides_LegacyRustKeepsSiblings(t *testing.T) {
	repo := t.TempDir()
	writeAssessYAML(t, repo, `version: 1
rust:
  clippy:
    enabled: true
lint:
  yamllint:
    enabled: false
`)
	if err := logger.Initialize(logger.Config{Level: logger.InfoLevel, Component: "goneat"}); err != nil {
		t.Fatalf("init logger: %v", err)
	}
	var logBuf bytes.Buffer
	logger.SetOutput(&logBuf)
	defer logger.SetOutput(io.Discard)

	overrides := loadAssessOverrides(repo)
	if overrides == nil || overrides.Lint == nil {
		t.Fatalf("valid sections must survive a legacy rust: block, got %+v", overrides)
	}
	if overrides.Lint.yamllintConfig().enabled() {
		t.Fatalf("lint.yamllint.enabled: false must be honored")
	}
	if !strings.Contains(logBuf.String(), "top-level rust: is not read from this file and is ignored; use format.rust in .goneat.yaml and lint.rust.clippy") {
		t.Fatalf("expected legacy rust: warning, got logs:\n%s", logBuf.String())
	}
}

func TestAssessConfigTopLevelKeysMatchSchema(t *testing.T) {
	data, err := os.ReadFile(filepath.Clean(assessConfigSourceSchema))
	if err != nil {
		t.Fatalf("read source schema: %v", err)
	}
	var doc struct {
		Properties map[string]any `yaml:"properties"`
	}
	if err := yaml.Unmarshal(data, &doc); err != nil {
		t.Fatalf("decode schema: %v", err)
	}
	var fromSchema, fromLoader []string
	for k := range doc.Properties {
		fromSchema = append(fromSchema, k)
	}
	for k := range assessConfigTopLevelKeys {
		fromLoader = append(fromLoader, k)
	}
	sort.Strings(fromSchema)
	sort.Strings(fromLoader)
	if strings.Join(fromSchema, ",") != strings.Join(fromLoader, ",") {
		t.Fatalf("loader top-level keys %v must match schema root properties %v", fromLoader, fromSchema)
	}
}

// Downstream repos carry an inert top-level format: block; it must warn once
// with its real home, keep sibling sections, and not repeat per runner.
func TestLoadAssessOverrides_LegacyFormatWarnsOncePerRun(t *testing.T) {
	repo := t.TempDir()
	writeAssessYAML(t, repo, `version: 1
format:
  json:
    ignore: ["**/dist/**"]
lint:
  yamllint:
    enabled: false
`)
	if err := logger.Initialize(logger.Config{Level: logger.InfoLevel, Component: "goneat"}); err != nil {
		t.Fatalf("init logger: %v", err)
	}
	var logBuf bytes.Buffer
	logger.SetOutput(&logBuf)
	defer logger.SetOutput(io.Discard)

	for i := 0; i < 3; i++ {
		absRepo, _ := filepath.Abs(repo)
		assessConfigCache.Delete(absRepo) // simulate separate loads by different runners
		overrides := loadAssessOverrides(repo)
		if overrides == nil || overrides.Lint.yamllintConfig().enabled() {
			t.Fatalf("sibling lint settings must survive a top-level format: block")
		}
	}
	logs := logBuf.String()
	if n := strings.Count(logs, "top-level format:"); n != 1 {
		t.Fatalf("expected exactly one format: warning, got %d:\n%s", n, logs)
	}
	if !strings.Contains(logs, ".goneatignore") || !strings.Contains(logs, "project .goneat.yaml") {
		t.Fatalf("format: warning must point to .goneat.yaml and .goneatignore, got:\n%s", logs)
	}
}

package schema

import (
	"os"
	"path/filepath"
	"testing"
)

// Config files are validated as written: JSON text keeps exact numbers and
// refuses invalid Unicode instead of being rounded or repaired by a YAML
// decode.
func TestValidateConfigFileRawJSON(t *testing.T) {
	dir := t.TempDir()
	for name, tt := range map[string]struct {
		body string
		ok   bool
	}{
		"rounded integer":  {`{"format":{"yaml":{"indent":2.0000000000000001}}}`, false},
		"exact integer":    {`{"format":{"yaml":{"indent":2}}}`, true},
		"lone surrogate":   {`{"format":{"yaml":{"line_ending":"\ud800"}}}`, false},
		"yaml still works": {"format:\n  yaml:\n    indent: 2\n", true},
	} {
		for _, ext := range []string{".yaml", ".json"} {
			if name == "yaml still works" && ext == ".json" {
				continue
			}
			p := filepath.Join(dir, "cfg"+ext)
			if err := os.WriteFile(p, []byte(tt.body), 0o600); err != nil {
				t.Fatal(err)
			}
			if err := ValidateConfigFile(p, "goneat-config-v1.0.0"); (err == nil) != tt.ok {
				t.Errorf("%s%s: err=%v, want ok=%v", name, ext, err, tt.ok)
			}
		}
	}
}

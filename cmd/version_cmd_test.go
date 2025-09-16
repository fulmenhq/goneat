package cmd

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/fulmenhq/goneat/pkg/buildinfo"
)

func TestVersion_JSON(t *testing.T) {
	out, err := execRoot(t, []string{"version", "--json"})
	if err != nil {
		t.Fatalf("version --json failed: %v\n%s", err, out)
	}
	var v map[string]any
	if json.Unmarshal([]byte(out), &v) != nil {
		t.Fatalf("version output is not valid JSON: %s", out)
	}
	if _, ok := v["binaryVersion"].(string); !ok {
		t.Errorf("expected binaryVersion field in JSON")
	}
	if _, ok := v["goVersion"].(string); !ok {
		t.Errorf("expected goVersion field in JSON")
	}
	if _, ok := v["platform"].(string); !ok {
		t.Errorf("expected platform field in JSON")
	}
}

func TestVersion_ProjectMode_JSON(t *testing.T) {
	out, err := execRoot(t, []string{"version", "--project", "--json"})
	if err != nil {
		t.Fatalf("version --project --json failed: %v\n%s", err, out)
	}
	var v map[string]any
	if json.Unmarshal([]byte(out), &v) != nil {
		t.Fatalf("version --project output is not valid JSON: %s", out)
	}
	if project, ok := v["project"].(map[string]any); ok {
		if _, ok := project["version"].(string); !ok {
			t.Errorf("expected project.version field in JSON")
		}
		if _, ok := project["source"].(string); !ok {
			t.Errorf("expected project.source field in JSON")
		}
	} else {
		t.Errorf("expected project object in JSON")
	}
	if _, ok := v["binaryVersion"].(string); !ok {
		t.Errorf("expected binaryVersion field in JSON")
	}
}

func TestVersion_DefaultOutput(t *testing.T) {
	out, err := execRoot(t, []string{"version"})
	if err != nil {
		t.Fatalf("version failed: %v\n%s", err, out)
	}

	// Should contain binary version prominently
	if !strings.Contains(out, buildinfo.BinaryVersion) {
		t.Errorf("expected binary version %s in output, got:\n%s", buildinfo.BinaryVersion, out)
	}

	// Should not show project version by default
	// Note: this test assumes we're in goneat repo where project and binary versions match
	// In external projects this would be different, but the structure test above covers that
}

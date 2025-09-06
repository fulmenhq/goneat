package cmd

import (
    "encoding/json"
    "testing"
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
    if _, ok := v["version"].(string); !ok {
        t.Errorf("expected version field in JSON")
    }
    if _, ok := v["goVersion"].(string); !ok {
        t.Errorf("expected goVersion field in JSON")
    }
    if _, ok := v["platform"].(string); !ok {
        t.Errorf("expected platform field in JSON")
    }
}


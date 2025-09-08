package cmd

import (
    "encoding/json"
    "testing"
)

func TestDocsList_JSON(t *testing.T) {
    out, err := execRoot(t, []string{"docs", "list", "--format", "json"})
    if err != nil {
        t.Fatalf("docs list failed: %v\n%s", err, out)
    }
    var items []map[string]any
    if json.Unmarshal([]byte(out), &items) != nil {
        t.Fatalf("docs list output is not valid JSON: %s", out)
    }
    if len(items) == 0 {
        t.Fatalf("expected at least one embedded doc; got 0")
    }
}

func TestDocsShow_JSON(t *testing.T) {
    // pick a known slug from manifest
    out, err := execRoot(t, []string{"docs", "show", "user-guide/install", "--format", "json"})
    if err != nil {
        t.Fatalf("docs show failed: %v\n%s", err, out)
    }
    var v map[string]any
    if json.Unmarshal([]byte(out), &v) != nil {
        t.Fatalf("docs show output is not valid JSON: %s", out)
    }
    if _, ok := v["content"]; !ok {
        t.Fatalf("expected content field in docs show JSON")
    }
}


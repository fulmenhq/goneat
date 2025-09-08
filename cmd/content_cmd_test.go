package cmd

import (
    "encoding/json"
    "testing"
)

func TestContentFind_JSON(t *testing.T) {
    out, err := execRoot(t, []string{"content", "find", "--format", "json"})
    if err != nil {
        t.Fatalf("content find failed: %v\n%s", err, out)
    }
    var v struct{
        Version string `json:"version"`
        Root    string `json:"root"`
        Manifest string `json:"manifest"`
        Count   int    `json:"count"`
        Items   []map[string]any `json:"items"`
    }
    if json.Unmarshal([]byte(out), &v) != nil {
        t.Fatalf("content find output is not valid JSON: %s", out)
    }
    if v.Count == 0 || len(v.Items) == 0 {
        t.Fatalf("expected at least one curated doc in find output: %s", out)
    }
}

func TestContentVerify_OK(t *testing.T) {
    // Ensure mirror is populated
    if _, err := execRoot(t, []string{"content", "embed"}); err != nil {
        t.Fatalf("content embed failed: %v", err)
    }
    if _, err := execRoot(t, []string{"content", "verify", "--format", "json"}); err != nil {
        t.Fatalf("content verify failed: %v", err)
    }
}


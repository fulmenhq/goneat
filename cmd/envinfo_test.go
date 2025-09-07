package cmd

import (
	"bytes"
	"encoding/json"
	"testing"
)

// helper to run root with args and capture stdout/stderr
func execRoot(t *testing.T, args []string) (string, error) {
	t.Helper()
	var buf bytes.Buffer
	rootCmd.SetOut(&buf)
	rootCmd.SetErr(&buf)
	// Reduce log noise to capture clean command output for JSON parsing
	full := append([]string{"--log-level", "error"}, args...)
	rootCmd.SetArgs(full)
	err := rootCmd.Execute()
	return buf.String(), err
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

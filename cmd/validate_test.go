package cmd

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestValidateData_Valid(t *testing.T) {
	// Create temp valid data
	tmpData := createTempFile(t, `format:
  go:
    simplify: true
security:
  timeout: 5m
`)
	defer func() { _ = os.Remove(tmpData) }()

	validateDataFile = tmpData
	validateDataSchema = "goneat-config-v1.0.0"
	validateFormat = "markdown"

	var buf bytes.Buffer
	cmd := &cobra.Command{} // Mock
	cmd.SetOut(&buf)
	if err := runValidateData(cmd, []string{}); err != nil {
		t.Errorf("expected success, got %v", err)
	}
	if !bytes.Contains(buf.Bytes(), []byte("✅ Validation passed")) {
		t.Errorf("expected passed message")
	}
}

func TestValidateData_Invalid(t *testing.T) {
	// Invalid data - indent below minimum
	tmpData := createTempFile(t, `format:
  yaml:
    indent: 1
security:
  timeout: 5m
`)
	defer func() { _ = os.Remove(tmpData) }()

	validateDataFile = tmpData
	validateDataSchema = "goneat-config-v1.0.0"
	validateFormat = "markdown"

	var buf bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&buf)
	err := runValidateData(cmd, []string{})
	if err == nil {
		t.Error("expected error")
	}
	if !bytes.Contains(buf.Bytes(), []byte("❌ Validation failed")) {
		t.Errorf("expected failed message")
	}
}

func TestValidateData_JSONOutput(t *testing.T) {
	tmpData := createTempFile(t, `format:
  yaml:
    indent: 1
security:
  timeout: 5m
`)
	defer func() { _ = os.Remove(tmpData) }()

	validateDataFile = tmpData
	validateDataSchema = "goneat-config-v1.0.0"
	validateFormat = "json"

	var buf bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&buf)
	_ = runValidateData(cmd, []string{}) // Ignore err for output test

	if !bytes.Contains(buf.Bytes(), []byte("\"valid\": false")) {
		t.Errorf("expected json invalid")
	}
}

func TestValidateData_MissingFlags(t *testing.T) {
	validateDataSchema = ""
	err := runValidateData(&cobra.Command{}, []string{})
	if err == nil || !strings.Contains(err.Error(), "required") {
		t.Errorf("expected flag error, got %v", err)
	}
}

func createTempFile(t *testing.T, content string) string {
	tmpf, err := os.CreateTemp("", "test-config-*.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tmpf.WriteString(content); err != nil {
		t.Fatal(err)
	}
	if err := tmpf.Close(); err != nil {
		t.Fatal(err)
	}
	return tmpf.Name()
}

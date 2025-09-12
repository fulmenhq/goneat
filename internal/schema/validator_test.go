package schema

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestValidate(t *testing.T) {
	// Valid config example
	validYAML := `
format:
  go:
    simplify: true
security:
  timeout: 5m
`
	var validDoc interface{}
	if err := yaml.Unmarshal([]byte(validYAML), &validDoc); err != nil {
		t.Fatal(err)
	}

	res, err := Validate(validDoc, "goneat-config-v1.0.0")
	if err != nil {
		t.Fatal(err)
	}
	if !res.Valid {
		t.Errorf("expected valid config, got errors: %v", res.Errors)
	}

	// Invalid: violates schema constraints
	invalidYAML := `
format:
  yaml:
    indent: 1  # Invalid: minimum is 2
    line_length: 50  # Invalid: minimum is 60
  json:
    indent: "invalid"  # Invalid: must match pattern ^(\\s+|\\t)$
security:
  timeout: 5m
`
	var invalidDoc interface{}
	if err := yaml.Unmarshal([]byte(invalidYAML), &invalidDoc); err != nil {
		t.Fatal(err)
	}

	res, err = Validate(invalidDoc, "goneat-config-v1.0.0")
	if err != nil {
		t.Fatal(err)
	}
	if res.Valid {
		t.Error("expected invalid config")
	}
	if len(res.Errors) == 0 {
		t.Error("expected validation errors")
	}

	// Non-existent schema
	_, err = Validate(validDoc, "nonexistent")
	if err == nil || !strings.Contains(err.Error(), "not found in registry") {
		t.Errorf("expected schema not found error, got %v", err)
	}
}

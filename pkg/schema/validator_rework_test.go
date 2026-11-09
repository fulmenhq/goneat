package schema

import (
	"encoding/json"
	"errors"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestValidateFromBytes_YAMLSchema(t *testing.T) {
	t.Parallel()
	schemaYAML := []byte(`
$schema: https://json-schema.org/draft/2020-12/schema
type: object
properties:
  name:
    type: string
required:
  - name
`)
	var data interface{}
	if err := yaml.Unmarshal([]byte(`name: Alice`), &data); err != nil {
		t.Fatalf("failed to parse data yaml: %v", err)
	}
	res, err := ValidateFromBytes(schemaYAML, data)
	if err != nil {
		t.Fatalf("ValidateFromBytes error: %v", err)
	}
	if !res.Valid {
		t.Fatalf("expected valid, got errors: %+v", res.Errors)
	}
}

func TestValidateFromBytes_JSONSchema(t *testing.T) {
	t.Parallel()
	schemaJSON := []byte(`{
  "$schema": "http://json-schema.org/draft-07/schema#",
  "type": "object",
  "properties": {
    "name": {"type": "string"},
    "age": {"type": "number"}
  },
  "required": ["name"]
}`)

	// Test 1: Valid data with required field
	var data1 map[string]interface{}
	if err := json.Unmarshal([]byte(`{"name":"Bob","age":30}`), &data1); err != nil {
		t.Fatalf("failed to parse json: %v", err)
	}
	res, err := ValidateFromBytes(schemaJSON, data1)
	if err != nil {
		t.Fatalf("ValidateFromBytes error: %v", err)
	}
	if !res.Valid {
		t.Fatalf("expected valid, got errors: %+v", res.Errors)
	}

	// Test 2: Invalid data missing required field
	var data2 map[string]interface{}
	if err := json.Unmarshal([]byte(`{"age":30}`), &data2); err != nil {
		t.Fatalf("failed to parse json: %v", err)
	}
	res, err = ValidateFromBytes(schemaJSON, data2)
	if err != nil {
		t.Fatalf("ValidateFromBytes error: %v", err)
	}
	if res.Valid {
		t.Fatalf("expected invalid, got valid")
	}
}

func TestValidateFromBytes_InvalidFormat(t *testing.T) {
	t.Parallel()
	_, err := ValidateFromBytes([]byte("not yaml and not json: \x00\x01"), map[string]any{"x": 1})
	if err == nil {
		t.Fatalf("expected error for invalid schema format")
	}
}

func TestValidateFromBytes_UnsupportedDraft(t *testing.T) {
	t.Parallel()
	schemaJSON := []byte(`{
  "$schema": "http://json-schema.org/draft-03/schema#",
  "type": "object"
}`)
	_, err := ValidateFromBytes(schemaJSON, map[string]any{})
	var ae *AdmissionError
	if !errors.As(err, &ae) || ae.Reason != "unsupported-dialect" {
		t.Fatalf("expected unsupported-dialect admission error, got %v", err)
	}
}

func TestValidateFromBytes_AllSupportedDrafts(t *testing.T) {
	t.Parallel()
	drafts := []struct {
		name   string
		schema string
	}{
		{"Draft-04", `{"$schema":"http://json-schema.org/draft-04/schema#","type":"object","required":["name"],"properties":{"name":{"type":"string"}}}`},
		{"Draft-06", `{"$schema":"http://json-schema.org/draft-06/schema#","type":"object","required":["name"],"properties":{"name":{"type":"string"}}}`},
		{"Draft-07", `{"$schema":"http://json-schema.org/draft-07/schema#","type":"object","required":["name"],"properties":{"name":{"type":"string"}}}`},
		{"2019-09", `{"$schema":"https://json-schema.org/draft/2019-09/schema","type":"object","required":["name"],"properties":{"name":{"type":"string"}}}`},
		{"2020-12", `{"$schema":"https://json-schema.org/draft/2020-12/schema","type":"object","required":["name"],"properties":{"name":{"type":"string"}}}`},
	}
	for _, d := range drafts {
		t.Run(d.name+"_valid", func(t *testing.T) {
			t.Parallel()
			res, err := ValidateFromBytes([]byte(d.schema), map[string]any{"name": "Alice"})
			if err != nil {
				t.Fatalf("%s: unexpected error: %v", d.name, err)
			}
			if !res.Valid {
				t.Fatalf("%s: expected valid, got errors: %+v", d.name, res.Errors)
			}
		})
		t.Run(d.name+"_invalid", func(t *testing.T) {
			t.Parallel()
			res, err := ValidateFromBytes([]byte(d.schema), map[string]any{"age": 30})
			if err != nil {
				t.Fatalf("%s: unexpected error: %v", d.name, err)
			}
			if res.Valid {
				t.Fatalf("%s: expected invalid (missing required 'name'), got valid", d.name)
			}
		})
	}
}

func TestCompileSchemaBytes_Required(t *testing.T) {
	t.Parallel()
	sch, err := compileSchemaBytes([]byte(`{"type":"object","required":["name"]}`), nil)
	if err != nil {
		t.Fatalf("compile err: %v", err)
	}
	res, err := validateWithCompiled(sch, map[string]any{})
	if err != nil {
		t.Fatalf("validate err: %v", err)
	}
	if res.Valid || len(res.Errors) == 0 || res.Dialect != "draft-07" || res.DialectSource != "defaulted" {
		t.Fatalf("expected invalid draft-07 (defaulted) result, got %+v", res)
	}
}

func TestConcurrentValidation(t *testing.T) {
	t.Parallel()
	// simple valid config doc for embedded schema
	var data interface{}
	if err := yaml.Unmarshal([]byte("format: {}\nsecurity: {timeout: '5m'}"), &data); err != nil {
		t.Fatalf("failed to parse yaml: %v", err)
	}
	done := make(chan error, 10)
	for i := 0; i < 10; i++ {
		go func() {
			_, err := Validate(data, "goneat-config-v1.0.0")
			done <- err
		}()
	}
	for i := 0; i < 10; i++ {
		if err := <-done; err != nil {
			t.Fatalf("concurrent Validate error: %v", err)
		}
	}
}

package schema

import (
	"encoding/json"
	"testing"

	"github.com/xeipuuv/gojsonschema"
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
  "$schema": "http://json-schema.org/draft-04/schema#",
  "type": "object"
}`)
	_, err := ValidateFromBytes(schemaJSON, map[string]any{})
	if err == nil {
		t.Fatalf("expected unsupported $schema error")
	}
}

func TestGojsonschemaRequiredWorks(t *testing.T) {
	t.Parallel()
	schema := `{"type":"object","required":["name"]}`
	data := `{}`
	res, err := gojsonschema.Validate(gojsonschema.NewStringLoader(schema), gojsonschema.NewStringLoader(data))
	if err != nil {
		t.Fatalf("validate err: %v", err)
	}
	if res.Valid() {
		t.Fatalf("expected invalid from direct gojsonschema, got valid")
	}
}

func TestCompileSchemaBytes_Required(t *testing.T) {
	t.Parallel()
	schemaJSON := []byte(`{"type":"object","required":["name"]}`)
	sch, err := compileSchemaBytes(schemaJSON)
	if err != nil {
		t.Fatalf("compile err: %v", err)
	}
	res, err := sch.Validate(gojsonschema.NewStringLoader(`{}`))
	if err != nil {
		t.Fatalf("validate err: %v", err)
	}
	if res.Valid() {
		t.Fatalf("expected invalid using compiled schema, got valid")
	}
}

func TestManualPathLikeValidateFromBytes(t *testing.T) {
	t.Parallel()
	// Mirrors ValidateFromBytes path (YAML-first decode, then bytes loaders)
	schemaJSON := []byte(`{"$schema":"http://json-schema.org/draft-07/schema#","type":"object","required":["name"]}`)
	var tmp any
	if err := yaml.Unmarshal(schemaJSON, &tmp); err != nil {
		t.Fatalf("yaml unmarshal failed: %v", err)
	}
	jb, err := json.Marshal(tmp)
	if err != nil {
		t.Fatalf("json marshal failed: %v", err)
	}
	dataJSON := []byte(`{}`)
	res, err := gojsonschema.Validate(gojsonschema.NewBytesLoader(jb), gojsonschema.NewBytesLoader(dataJSON))
	if err != nil {
		t.Fatalf("validate err: %v", err)
	}
	if res.Valid() {
		t.Fatalf("expected invalid (manual path), got valid")
	}
}

func TestDirectGojsonschemaRequired(t *testing.T) {
	t.Parallel()
	// Test gojsonschema directly to ensure it works with required fields
	schema := `{"$schema":"http://json-schema.org/draft-07/schema#","type":"object","required":["name"]}`
	data := `{}`
	res, err := gojsonschema.Validate(gojsonschema.NewStringLoader(schema), gojsonschema.NewStringLoader(data))
	if err != nil {
		t.Fatalf("direct validate err: %v", err)
	}
	if res.Valid() {
		t.Fatalf("expected invalid from direct gojsonschema, got valid")
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

package schema

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// $schema is honoured on every path (it used to be stripped): a 2020-12
// schema applies the sibling of $ref, which draft-07 ignores.
func TestDeclaredDialectHonouredOnEveryPath(t *testing.T) {
	modern := `{"$schema":"https://json-schema.org/draft/2020-12/schema","$defs":{"s":{"type":"string"}},"$ref":"#/$defs/s","maxLength":2}`
	yamlModern := "$schema: https://json-schema.org/draft/2020-12/schema\n$defs:\n  s:\n    type: string\n$ref: '#/$defs/s'\nmaxLength: 2\n"
	for name, src := range map[string]string{"json": modern, "yaml": yamlModern} {
		res, err := ValidateFromBytes([]byte(src), "abc")
		if err != nil || res.Valid || res.Dialect != "draft/2020-12" || res.DialectSource != "declared" {
			t.Errorf("%s: %+v %v", name, res, err)
		}
	}
	t.Setenv("GONEAT_OFFLINE_SCHEMA_VALIDATION", "true")
	res, err := ValidateFromBytes([]byte(modern), "abc")
	if err != nil || res.Valid || res.Dialect != "draft/2020-12" {
		t.Errorf("offline env: %+v %v", res, err)
	}
	dir := t.TempDir()
	root := `{"$schema":"https://json-schema.org/draft/2020-12/schema","$id":"https://example.com/root.json","$ref":"https://example.com/s.json","maxLength":2}`
	if err := os.WriteFile(filepath.Join(dir, "s.json"), []byte(`{"$schema":"https://json-schema.org/draft/2020-12/schema","$id":"https://example.com/s.json","type":"string"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	res, err = ValidateFromBytesWithRefDirs([]byte(root), "abc", []string{dir})
	if err != nil || res.Valid {
		t.Errorf("ref-dir: %+v %v", res, err)
	}
	idx, err := BuildIDIndexFromRefDirs([]string{dir})
	if err != nil {
		t.Fatal(err)
	}
	if e, ok := idx.Get("https://example.com/s.json"); !ok || !strings.Contains(string(e.Normalized), `"$schema"`) {
		t.Errorf("index dropped $schema: %+v", e)
	}
	res, err = ValidateFromBytesWithIDIndex([]byte(root), "abc", idx)
	if err != nil || res.Valid {
		t.Errorf("id-index: %+v %v", res, err)
	}
	if res, err := ValidateFromBytesWithIDIndex([]byte(root), "ab", idx); err != nil || !res.Valid {
		t.Errorf("id-index valid: %+v %v", res, err)
	}
}

// Ref-dirs admit only documents that declare an $id; nothing is read by
// relative path or file URL, and a missing reference is an error.
func TestRefDirsAreAClosedResourceSet(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "noid.json"), []byte(`{"type":"integer"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	for name, root := range map[string]string{
		"relative file":   `{"$id":"https://example.com/root.json","$ref":"noid.json"}`,
		"file url":        `{"$ref":"file://` + filepath.ToSlash(filepath.Join(dir, "noid.json")) + `"}`,
		"unbundled https": `{"$ref":"https://example.com/missing.json"}`,
	} {
		_, err := ValidateFromBytesWithRefDirs([]byte(root), 1, []string{dir})
		var ae *AdmissionError
		if !errors.As(err, &ae) {
			t.Errorf("%s: %v", name, err)
		}
	}
	// The root may also live in the ref-dir.
	rootDoc := `{"$id":"https://example.com/self.json","type":"integer"}`
	if err := os.WriteFile(filepath.Join(dir, "self.json"), []byte(rootDoc), 0o600); err != nil {
		t.Fatal(err)
	}
	if res, err := ValidateFromBytesWithRefDirs([]byte(rootDoc), 1, []string{dir}); err != nil || !res.Valid {
		t.Errorf("root in ref-dir: %+v %v", res, err)
	}
	if _, err := ValidateFromBytesWithRefDirs([]byte(`{"$id":"https://example.com/self.json","type":"string"}`), 1, []string{dir}); err == nil || !strings.Contains(err.Error(), "duplicate schema $id") {
		t.Errorf("conflicting root accepted: %v", err)
	}
}

// Validating a schema against its draft's metaschema uses the engine's
// embedded copy: valid schemas pass and invalid ones fail in every draft.
func TestMetaSchemaValidationParity(t *testing.T) {
	for _, draft := range []string{"draft-04", "draft-06", "draft-07", "2019-09", "2020-12"} {
		v, err := NewValidatorFromMetaSchema(draft)
		if err != nil {
			t.Fatalf("%s: %v", draft, err)
		}
		for doc, want := range map[string]bool{
			`{"type":"object","properties":{"a":{"type":"string"}},"required":["a"]}`: true,
			`{"type":"strng"}`:     false,
			`{"minLength":-1}`:     false,
			`{"properties":[1]}`:   false,
			`{"required":"a"}`:     false,
			`{"x-extension":true}`: true,
		} {
			res, err := v.ValidateBytes([]byte(doc))
			if err != nil || res.Valid != want {
				t.Errorf("%s %s: %+v %v", draft, doc, res, err)
			}
		}
	}
	if _, err := NewValidatorFromMetaSchema("draft-03"); err == nil {
		t.Error("draft-03 metaschema available")
	}
}

func TestEmbeddedSchemaCompileErrorIsRetained(t *testing.T) {
	broken := errors.New("embedded schema zz-broken: failed to load schema: boom")
	regMu.Lock()
	schemaPaths["zz-broken"] = "embedded_schemas/schemas/config/v1.0.0/goneat-config.yaml"
	schemaRegistry["zz-broken"] = registryEntry{err: broken}
	regMu.Unlock()
	defer func() {
		regMu.Lock()
		delete(schemaPaths, "zz-broken")
		delete(schemaRegistry, "zz-broken")
		regMu.Unlock()
	}()
	for i := 0; i < 2; i++ {
		if _, err := GetEmbeddedValidator("zz-broken"); err != broken {
			t.Fatalf("call %d: %v", i, err)
		}
	}
}

func TestResultReportsCapability(t *testing.T) {
	res, err := Validate(map[string]any{}, "goneat-config-v1.0.0")
	if err != nil {
		t.Fatal(err)
	}
	if res.Engine == "" || res.Dialect == "" || res.DialectSource == "" || res.FormatMode != "assert" || res.LeapPolicy == "" {
		t.Fatalf("capability missing: %+v", res)
	}
}

package schemarun

import (
	"errors"
	"testing"

	"github.com/fulmenhq/goneat/internal/schemaengine"
)

func TestSchemaJSONKeepsJSONAndConvertsYAML(t *testing.T) {
	raw := []byte(`{"$schema":"https://json-schema.org/draft/2020-12/schema","const":9007199254740993}`)
	got, err := SchemaJSON(raw)
	if err != nil || string(got) != string(raw) {
		t.Fatalf("JSON input changed: %s %v", got, err)
	}
	got, err = SchemaJSON([]byte("$schema: https://json-schema.org/draft/2020-12/schema\ntype: string\n"))
	if err != nil || DeclaredID(got) != "" {
		t.Fatalf("yaml: %s %v", got, err)
	}
	c, err := Compile([]byte("$schema: https://json-schema.org/draft/2020-12/schema\ntype: string\n"), nil)
	if err != nil || c.Capability().Dialect != "draft/2020-12" || c.Capability().DialectSource != "declared" {
		t.Fatalf("YAML $schema not honoured: %v", err)
	}
	if _, err := Compile([]byte("\x00\x01: ["), nil); err == nil {
		t.Fatal("garbage schema admitted")
	} else {
		var ae *schemaengine.AdmissionError
		if !errors.As(err, &ae) || ae.Reason != schemaengine.ReasonInvalidJSON {
			t.Fatalf("garbage schema: %v", err)
		}
	}
}

func TestPointerPath(t *testing.T) {
	for ptr, want := range map[string]string{
		"":             "(root)",
		"/a":           "a",
		"/tools/x/0":   "tools.x.0",
		"/a~1b/c~0d":   "a/b.c~d",
		"/hooks/pre-c": "hooks.pre-c",
	} {
		if got := PointerPath(ptr); got != want {
			t.Errorf("%q: %q, want %q", ptr, got, want)
		}
	}
}

func TestImproveMessageByKeyword(t *testing.T) {
	for _, tt := range []struct {
		is   Issue
		name string
		want string
	}{
		{Issue{Path: "tools.x", Keyword: "not", Message: "'not' failed"}, "tools-config-v1.1.0", "Both 'install' and 'install_commands' cannot be present (mutually exclusive). Use only 'install' for v1.1.0+ package managers, or only 'install_commands' for legacy scripts. See schema: tools-config-v1.1.0"},
		{Issue{Path: "tools.x", Keyword: "additionalProperties", Message: "additional properties 'install' not allowed"}, "", "The 'install' property requires schema v1.1.0+. Either upgrade to v1.1.0 schema or use 'install_commands' instead."},
		{Issue{Path: "tools.x", Keyword: "additionalProperties", Message: "additional properties 'other' not allowed"}, "", "additional properties 'other' not allowed"},
		{Issue{Path: "a", Keyword: "not", Message: "'not' failed"}, "", "'not' failed (Schema constraint violation - check for mutually exclusive properties or invalid combinations)"},
		{Issue{Path: "a", Keyword: "required", Message: "missing properties 'x', 'y'"}, "", "missing required properties 'x', 'y'"},
		{Issue{Path: "a", Keyword: "required", Message: "missing property 'x'"}, "", "missing required property 'x'"},
		{Issue{Path: "a", Keyword: "type", Message: "got number, want string"}, "", "got number, want string"},
	} {
		if got := ImproveMessage(tt.is, tt.name); got != tt.want {
			t.Errorf("%+v: %q", tt.is, got)
		}
	}
}

func TestValidateReportsKeywordAndPath(t *testing.T) {
	c, err := Compile([]byte(`{"properties":{"tools":{"additionalProperties":{"allOf":[{"not":{"required":["a","b"]}}],"required":["name"]}}}}`), nil)
	if err != nil {
		t.Fatal(err)
	}
	valid, issues, err := Validate(c, map[string]any{"tools": map[string]any{"t1": map[string]any{"a": 1, "b": 2}}})
	if err != nil || valid {
		t.Fatalf("%v %v", valid, err)
	}
	seen := map[string]string{}
	for _, is := range issues {
		seen[is.Keyword] = is.Path
	}
	if seen["not"] != "tools.t1" || seen["required"] != "tools.t1" {
		t.Fatalf("issues: %+v", issues)
	}
}

func TestCompileMetaschema(t *testing.T) {
	for _, draft := range []string{"draft-04", "4", "06", "draft-07", "2019-09", "202012"} {
		c, err := CompileMetaschema(draft)
		if err != nil {
			t.Fatalf("%s: %v", draft, err)
		}
		if ok, _, _ := Validate(c, map[string]any{"type": "string"}); !ok {
			t.Errorf("%s: valid schema rejected", draft)
		}
		if ok, _, _ := Validate(c, map[string]any{"type": 5}); ok {
			t.Errorf("%s: invalid schema accepted", draft)
		}
	}
	if _, err := CompileMetaschema("draft-03"); err == nil {
		t.Fatal("draft-03 metaschema available")
	}
	for id, want := range map[string]string{
		"http://json-schema.org/draft-07/schema#":      "draft-07",
		"http://json-schema.org/draft-07/schema":       "draft-07",
		"https://json-schema.org/draft/2020-12/schema": "2020-12",
	} {
		if got, ok := MetaschemaDraft(id); !ok || got != want {
			t.Errorf("%s: %q %v", id, got, ok)
		}
	}
	if _, ok := MetaschemaDraft("https://json-schema.org/schema"); ok {
		t.Error("unversioned alias treated as a metaschema")
	}
}

func TestSameDocument(t *testing.T) {
	if !SameDocument([]byte(`{"a":1,"b":[1,2]}`), []byte("{\n  \"b\": [1, 2],\n  \"a\": 1\n}")) {
		t.Error("formatting difference treated as different")
	}
	if SameDocument([]byte(`{"a":1}`), []byte(`{"a":1.0}`)) {
		t.Error("different numbers treated as same")
	}
}

package assess

import (
	"testing"
)

func TestIsUnderSchemas(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"schemas/config/foo.yaml", true},
		{"docs/schemas/config.yaml", true},
		{"SCHEMAS/upper/ignored.yaml", false}, // case-sensitive segment
		{"config/schema.yaml", false},
	}
	for _, c := range cases {
		if got := isUnderSchemas(c.in); got != c.want {
			t.Errorf("isUnderSchemas(%q)=%v want %v", c.in, got, c.want)
		}
	}
}

func TestSanityCheckJSONSchema_Invalid(t *testing.T) {
	// required must be array
	m1 := map[string]any{
		"type":     "object",
		"required": "name",
	}
	if err := sanityCheckJSONSchema(m1); err == nil {
		t.Errorf("expected error for required as string")
	}
	// additionalProperties must be bool or object
	m2 := map[string]any{
		"type":                 "object",
		"additionalProperties": "nope",
	}
	if err := sanityCheckJSONSchema(m2); err == nil {
		t.Errorf("expected error for additionalProperties as string")
	}
	// invalid type
	m3 := map[string]any{"type": 123}
	if err := sanityCheckJSONSchema(m3); err == nil {
		t.Errorf("expected error for invalid type value")
	}
}

func TestSanityCheckJSONSchema_Valid(t *testing.T) {
	m := map[string]any{
		"type":     "object",
		"required": []any{"name"},
		"properties": map[string]any{
			"name": map[string]any{"type": "string"},
		},
		"additionalProperties": false,
	}
	if err := sanityCheckJSONSchema(m); err != nil {
		t.Fatalf("unexpected error for valid schema: %v", err)
	}
}

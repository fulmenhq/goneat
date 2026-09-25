package schema

import (
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// Repository convention (not a JSON Schema validity rule): a property whose
// name is a JSON Schema keyword must not have a boolean or scalar subschema.
// A real property schema here is always a mapping; `additionalProperties:
// false` nested one level too deep under `properties` parses as such a
// property and silently leaves the parent object open. JSON Schema allows
// boolean subschemas in general, so meta-schemas (which define keyword-named
// properties on purpose) are excluded.
var schemaKeywords = map[string]bool{
	"additionalProperties": true, "required": true, "type": true, "items": true,
	"patternProperties": true, "properties": true, "$ref": true, "enum": true,
	"oneOf": true, "anyOf": true, "allOf": true, "unevaluatedProperties": true,
	"minProperties": true, "maxProperties": true,
}

func findKeywordScalarProperties(node any, path string, report func(string)) {
	switch n := node.(type) {
	case map[string]any:
		if props, ok := n["properties"].(map[string]any); ok {
			for name, sub := range props {
				if _, isMap := sub.(map[string]any); !isMap && schemaKeywords[name] {
					report(path + "/properties/" + name)
				}
			}
		}
		for k, v := range n {
			findKeywordScalarProperties(v, path+"/"+k, report)
		}
	case []any:
		for i, v := range n {
			findKeywordScalarProperties(v, path+"/"+strconv.Itoa(i), report)
		}
	}
}

func TestSchemas_NoKeywordNamedScalarProperties(t *testing.T) {
	roots := []string{"../../schemas", "../assets/embedded_schemas"}
	checked := 0
	for _, root := range roots {
		err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				if d.Name() == "meta" {
					return filepath.SkipDir
				}
				return nil
			}
			ext := strings.ToLower(filepath.Ext(path))
			if ext != ".yaml" && ext != ".yml" && ext != ".json" {
				return nil
			}
			data, err := os.ReadFile(path) // #nosec G304 -- test walks repository schema directories
			if err != nil {
				return err
			}
			var doc any
			if ext == ".json" {
				if json.Unmarshal(data, &doc) != nil {
					return nil
				}
			} else if yaml.Unmarshal(data, &doc) != nil {
				return nil
			}
			checked++
			findKeywordScalarProperties(doc, "", func(p string) {
				t.Errorf("%s: %s is a keyword-named property with a non-schema value (likely a keyword indented one level too deep)", path, p)
			})
			return nil
		})
		if err != nil {
			t.Fatalf("walk %s: %v", root, err)
		}
	}
	if checked == 0 {
		t.Fatalf("no schema files checked")
	}
}

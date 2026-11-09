// Package schemarun connects goneat's schema callers to the single JSON Schema
// compilation adapter (internal/schemaengine). It converts YAML schema
// sources to JSON, shares one compile cache, and maps engine issues to the
// path/message form goneat reports.
package schemarun

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"

	"github.com/fulmenhq/goneat/internal/schemaengine"
	"gopkg.in/yaml.v3"
)

var cache schemaengine.Cache

// SchemaJSON returns schema source as JSON text. JSON input is returned
// unchanged so numbers and $schema are preserved exactly; anything else is
// parsed as YAML and encoded as JSON.
func SchemaJSON(src []byte) ([]byte, error) {
	if json.Valid(src) {
		return src, nil
	}
	var doc any
	if err := yaml.Unmarshal(src, &doc); err != nil {
		return nil, fmt.Errorf("invalid schema format (must be valid YAML or JSON): %w", err)
	}
	out, err := json.Marshal(doc)
	if err != nil {
		return nil, fmt.Errorf("failed to encode schema to JSON: %w", err)
	}
	return out, nil
}

// SameDocument reports whether two JSON documents decode to the same value.
func SameDocument(a, b []byte) bool {
	if string(a) == string(b) {
		return true
	}
	da, errA := schemaengine.DecodeJSON(a, 0)
	db, errB := schemaengine.DecodeJSON(b, 0)
	return errA == nil && errB == nil && reflect.DeepEqual(da, db)
}

// DeclaredID returns a schema document's $id (or draft-04 id), trimmed.
func DeclaredID(schemaJSON []byte) string {
	var m map[string]any
	if json.Unmarshal(schemaJSON, &m) != nil {
		return ""
	}
	id, _ := m["$id"].(string)
	if id == "" {
		id, _ = m["id"].(string)
	}
	return strings.TrimSpace(id)
}

// Compile admits and compiles a schema (JSON or YAML) with the given admitted
// resources through the shared cache. Admission failures are returned as
// *schemaengine.AdmissionError (possibly wrapped).
func Compile(src []byte, resources []schemaengine.Resource) (*schemaengine.Compiled, error) {
	root, err := SchemaJSON(src)
	if err != nil {
		return nil, &schemaengine.AdmissionError{Reason: schemaengine.ReasonInvalidJSON, Detail: "schema is not valid YAML or JSON", Err: err}
	}
	return cache.Get(root, schemaengine.Options{Resources: resources})
}

// metaschemaURLs are the dialect identities the engine embeds.
var metaschemaURLs = map[string]string{
	"draft-04": "http://json-schema.org/draft-04/schema#",
	"draft-06": "http://json-schema.org/draft-06/schema#",
	"draft-07": "http://json-schema.org/draft-07/schema#",
	"2019-09":  "https://json-schema.org/draft/2019-09/schema",
	"2020-12":  "https://json-schema.org/draft/2020-12/schema",
}

// NormalizeDraft maps goneat's accepted draft spellings to a canonical name.
func NormalizeDraft(draft string) (string, bool) {
	switch strings.TrimSpace(draft) {
	case "draft-04", "04", "4":
		return "draft-04", true
	case "draft-06", "06", "6":
		return "draft-06", true
	case "draft-07", "07", "7":
		return "draft-07", true
	case "2019-09", "2019", "201909":
		return "2019-09", true
	case "2020-12", "2020", "202012":
		return "2020-12", true
	}
	return "", false
}

// MetaschemaDraft reports the draft whose metaschema identity id is.
func MetaschemaDraft(id string) (string, bool) {
	id = strings.TrimSuffix(strings.TrimSpace(id), "#")
	for name, u := range metaschemaURLs {
		if strings.TrimSuffix(u, "#") == id {
			return name, true
		}
	}
	return "", false
}

// CompileMetaschema compiles a validator for the named draft's metaschema
// from the engine's embedded copy. Nothing is fetched.
func CompileMetaschema(draft string) (*schemaengine.Compiled, error) {
	name, ok := NormalizeDraft(draft)
	if !ok {
		return nil, fmt.Errorf("meta-schema for %s not available", draft)
	}
	u := metaschemaURLs[name]
	root, err := json.Marshal(map[string]string{"$schema": u, "$ref": u})
	if err != nil {
		return nil, err
	}
	return cache.Get(root, schemaengine.Options{})
}

// Issue is an engine issue mapped to goneat's reporting form.
type Issue struct {
	Path    string // dotted instance path, "(root)" for the document itself
	Keyword string // the keyword that failed
	Message string
}

// Validate checks decoded data (from YAML or JSON) against a compiled schema.
func Validate(c *schemaengine.Compiled, data any) (bool, []Issue, error) {
	raw, err := json.Marshal(data)
	if err != nil {
		return false, nil, fmt.Errorf("encode data to JSON: %w", err)
	}
	return ValidateJSON(c, raw)
}

// ValidateJSON checks JSON instance text against a compiled schema.
func ValidateJSON(c *schemaengine.Compiled, raw []byte) (bool, []Issue, error) {
	res, err := c.ValidateJSON(raw)
	if err != nil {
		return false, nil, fmt.Errorf("validation error: %w", err)
	}
	if res.Valid {
		return true, nil, nil
	}
	issues := make([]Issue, 0, len(res.Issues))
	for _, is := range res.Issues {
		issues = append(issues, Issue{
			Path:    PointerPath(is.InstanceLocation),
			Keyword: is.Keyword,
			Message: is.Message,
		})
	}
	return false, issues, nil
}

// PointerPath converts a JSON Pointer to the dotted form goneat reports.
func PointerPath(ptr string) string {
	if ptr == "" || ptr == "/" {
		return "(root)"
	}
	parts := strings.Split(strings.TrimPrefix(ptr, "/"), "/")
	for i, p := range parts {
		parts[i] = strings.NewReplacer("~1", "/", "~0", "~").Replace(p)
	}
	return strings.Join(parts, ".")
}

// ImproveMessage gives actionable text for known tools-config failures and
// marks generic "not" failures. schemaName, when set, is appended to the
// tools-config guidance.
func ImproveMessage(is Issue, schemaName string) string {
	see := ""
	if schemaName != "" {
		see = " See schema: " + schemaName
	}
	if strings.HasPrefix(is.Path, "tools.") {
		if is.Keyword == "not" {
			return "Both 'install' and 'install_commands' cannot be present (mutually exclusive). Use only 'install' for v1.1.0+ package managers, or only 'install_commands' for legacy scripts." + see
		}
		if is.Keyword == "additionalProperties" && strings.Contains(is.Message, "'install'") {
			return "The 'install' property requires schema v1.1.0+. Either upgrade to v1.1.0 schema or use 'install_commands' instead." + see
		}
	}
	if is.Keyword == "required" && strings.HasPrefix(is.Message, "missing propert") {
		return "missing required " + strings.TrimPrefix(is.Message, "missing ")
	}
	if is.Keyword == "not" {
		return is.Message + " (Schema constraint violation - check for mutually exclusive properties or invalid combinations)"
	}
	return is.Message
}

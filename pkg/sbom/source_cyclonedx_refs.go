package sbom

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/fulmenhq/goneat/internal/assets"
)

var sourceBOMLink = regexp.MustCompile(`^urn:cdx:[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}/[1-9][0-9]*#.+$`)

// Follow the bundled schema's reference types rather than guessing from member
// names. This covers references in evidence, annotations, declarations, formulae,
// and cryptographic assets as well as the ordinary dependency graph. Unknown
// extension values are not promoted into definitions merely by naming a member
// "bom-ref".
func validateSourceCycloneDXReferences(document map[string]any) error {
	version, _ := document["specVersion"].(string)
	if version != "1.6" && version != "1.7" {
		return fmt.Errorf("unsupported CycloneDX reference schema %q", version)
	}
	data, ok := assets.GetSchema("embedded_schemas/schemas/sbom/bom-" + version + ".schema.json")
	if !ok {
		return fmt.Errorf("missing CycloneDX reference schema")
	}
	schema, err := decodeSourceJSON(data)
	if err != nil {
		return err
	}
	definitions := make(map[string]string) // ID -> defining instance pointer
	references := make(map[string]bool)
	var walk func(any, map[string]any, string, string, bool, int) error
	walk = func(value any, rule map[string]any, pointer, property string, external bool, depth int) error {
		if rule == nil {
			return nil
		}
		if depth > 2048 {
			return fmt.Errorf("CycloneDX reference traversal exceeds depth budget")
		}
		for _, key := range []string{"oneOf", "anyOf"} {
			for _, alternative := range sourceArray(rule[key]) {
				if sourceObject(alternative)["$ref"] == "#/definitions/bomLinkElementType" {
					external = true
				}
			}
		}
		if ref, ok := rule["$ref"].(string); ok {
			switch ref {
			case "#/definitions/refType", "#/definitions/refLinkType":
				id, ok := value.(string)
				if !ok {
					return nil // Inactive scalar branch of an already-validated union.
				}
				if id == "" {
					return fmt.Errorf("invalid CycloneDX reference at %s", pointer)
				}
				if property == "bom-ref" && ref == "#/definitions/refType" {
					if previous, exists := definitions[id]; exists && previous != pointer {
						return fmt.Errorf("source SBOM has duplicate identifier %q", id)
					}
					definitions[id] = pointer
				} else if !external || !sourceBOMLink.MatchString(id) {
					references[id] = true
				}
				return nil
			case "#/definitions/bomLinkElementType", "#/definitions/bomLinkDocumentType":
				return nil // External BOM links are validated by JSON Schema.
			default:
				if strings.HasPrefix(ref, "#/") {
					resolved := schema
					for _, part := range strings.Split(strings.TrimPrefix(ref, "#/"), "/") {
						part = strings.ReplaceAll(strings.ReplaceAll(part, "~1", "/"), "~0", "~")
						resolved = sourceObject(resolved[part])
					}
					if resolved == nil {
						return fmt.Errorf("unresolved bundled CycloneDX schema reference %s", ref)
					}
					if err := walk(value, resolved, pointer, property, external, depth+1); err != nil {
						return err
					}
				}
				// Bundled external schemas define SPDX license strings,
				// signatures, and crypto enums, not local BOM references.
			}
		}
		for _, key := range []string{"allOf", "anyOf", "oneOf"} {
			for _, alternative := range sourceArray(rule[key]) {
				if err := walk(value, sourceObject(alternative), pointer, property, external, depth+1); err != nil {
					return err
				}
			}
		}
		for _, key := range []string{"then", "else"} {
			if err := walk(value, sourceObject(rule[key]), pointer, property, external, depth+1); err != nil {
				return err
			}
		}
		switch value := value.(type) {
		case map[string]any:
			properties := sourceObject(rule["properties"])
			for name, child := range value {
				escaped := strings.ReplaceAll(strings.ReplaceAll(name, "~", "~0"), "/", "~1")
				childRule, matched := properties[name]
				if err := walk(child, sourceObject(childRule), pointer+"/"+escaped, name, false, depth+1); err != nil {
					return err
				}
				for pattern, patternRule := range sourceObject(rule["patternProperties"]) {
					re, err := regexp.Compile(pattern)
					if err != nil {
						return err
					}
					if re.MatchString(name) {
						matched = true
						if err := walk(child, sourceObject(patternRule), pointer+"/"+escaped, name, false, depth+1); err != nil {
							return err
						}
					}
				}
				if !matched {
					if err := walk(child, sourceObject(rule["additionalProperties"]), pointer+"/"+escaped, name, false, depth+1); err != nil {
						return err
					}
				}
			}
		case []any:
			for i, child := range value {
				itemRule := sourceObject(rule["items"])
				if tuple := sourceArray(rule["items"]); i < len(tuple) {
					itemRule = sourceObject(tuple[i])
				}
				if err := walk(child, itemRule, pointer+"/"+strconv.Itoa(i), property, false, depth+1); err != nil {
					return err
				}
			}
		}
		return nil
	}
	if err := walk(document, schema, "", "", false, 0); err != nil {
		return err
	}
	for id := range references {
		if _, exists := definitions[id]; !exists {
			return fmt.Errorf("source SBOM has unresolved CycloneDX reference %q", id)
		}
	}
	return nil
}

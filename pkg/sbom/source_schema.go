package sbom

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"sync"

	"github.com/fulmenhq/goneat/internal/assets"
	"github.com/xeipuuv/gojsonschema"
)

var sourceSchemas = sync.OnceValues(compileSourceSchemas)

func compileSourceSchemas() (map[string]*gojsonschema.Schema, error) {
	loader := gojsonschema.NewSchemaLoader()
	loader.AutoDetect = false
	loader.Draft = gojsonschema.Draft7
	// All external references are audited below before compilation. Registering
	// this closed set by ID avoids a network-dependent validation path.
	files := []string{"bom-1.6.schema.json", "bom-1.7.schema.json", "spdx.schema.json", "jsf-0.82.schema.json", "cryptography-defs.schema.json", "spdx-2.3.schema.json"}
	documents := make(map[string]map[string]any)
	for _, name := range files {
		data, ok := assets.GetSchema("embedded_schemas/schemas/sbom/" + name)
		if !ok {
			return nil, fmt.Errorf("missing embedded SBOM schema %s", name)
		}
		document, err := decodeSourceJSON(data)
		if err != nil {
			return nil, fmt.Errorf("SBOM schema %s: %w", name, err)
		}
		id, ok := document["$id"].(string)
		if !ok || id == "" {
			return nil, fmt.Errorf("SBOM schema %s has no ID", name)
		}
		documents[id] = document
	}
	for id, document := range documents {
		base, err := url.Parse(id)
		if err != nil {
			return nil, err
		}
		if err := checkSourceSchemaRefs(document, base, documents); err != nil {
			return nil, err
		}
		if err := loader.AddSchema(id, gojsonschema.NewGoLoader(document)); err != nil {
			return nil, err
		}
	}
	compiled := make(map[string]*gojsonschema.Schema)
	for _, id := range []string{"http://cyclonedx.org/schema/bom-1.6.schema.json", "http://cyclonedx.org/schema/bom-1.7.schema.json", "http://spdx.org/rdf/terms/2.3"} {
		schema, err := loader.Compile(gojsonschema.NewReferenceLoader(id))
		if err != nil {
			return nil, fmt.Errorf("compile offline SBOM schema %s: %w", id, err)
		}
		compiled[id] = schema
	}
	return compiled, nil
}

func checkSourceSchemaRefs(value any, base *url.URL, documents map[string]map[string]any) error {
	switch value := value.(type) {
	case map[string]any:
		if id, ok := value["$id"].(string); ok {
			parsed, err := url.Parse(id)
			if err != nil {
				return err
			}
			base = base.ResolveReference(parsed)
		}
		if ref, ok := value["$ref"].(string); ok {
			parsed, err := url.Parse(ref)
			if err != nil {
				return err
			}
			resolved := base.ResolveReference(parsed)
			resolved.Fragment = ""
			if _, ok := documents[resolved.String()]; !ok {
				return fmt.Errorf("SBOM schema reference is not bundled: %s", ref)
			}
		}
		for _, child := range value {
			if err := checkSourceSchemaRefs(child, base, documents); err != nil {
				return err
			}
		}
	case []any:
		for _, child := range value {
			if err := checkSourceSchemaRefs(child, base, documents); err != nil {
				return err
			}
		}
	}
	return nil
}

func validateSourceDocument(document map[string]any, format string) error {
	var id string
	switch format {
	case "cyclonedx-json":
		version, _ := document["specVersion"].(string)
		if version != "1.6" && version != "1.7" {
			return fmt.Errorf("unsupported source SBOM CycloneDX version %q", version)
		}
		id = "http://cyclonedx.org/schema/bom-" + version + ".schema.json"
	case "spdx-json":
		if document["spdxVersion"] != "SPDX-2.3" {
			return fmt.Errorf("unsupported source SBOM SPDX version %v", document["spdxVersion"])
		}
		id = "http://spdx.org/rdf/terms/2.3"
	default:
		return fmt.Errorf("unsupported source SBOM format %q", format)
	}
	schemas, err := sourceSchemas()
	if err != nil {
		return err
	}
	// Use the lossless JSON model directly; never route output through YAML or
	// interface decoding that turns all JSON numbers into float64.
	data, err := json.Marshal(document)
	if err != nil {
		return err
	}
	result, err := schemas[id].Validate(gojsonschema.NewBytesLoader(data))
	if err != nil {
		return fmt.Errorf("validate source SBOM: %w", err)
	}
	if !result.Valid() {
		var messages []string
		for _, issue := range result.Errors() {
			messages = append(messages, issue.String())
		}
		return fmt.Errorf("source SBOM schema validation failed: %s", strings.Join(messages, "; "))
	}
	return nil
}

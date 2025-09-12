package assets

import (
	"embed"
	"encoding/json"
	"strings"

	"gopkg.in/yaml.v3"
)

//go:embed embedded_schemas
var schemaFS embed.FS

// SchemaInfo holds schema metadata.
type SchemaInfo struct {
	Name  string `json:"name"`
	Path  string `json:"path"`
	Draft string `json:"draft"`
}

// GetSchema returns the embedded schema bytes by relative path (e.g., "config/goneat-config-v1.0.0.yaml").
func GetSchema(relPath string) ([]byte, bool) {
	data, err := schemaFS.ReadFile(relPath)
	return data, err == nil
}

// GetSchemaNames returns list of available schemas with metadata (heuristic draft detection).
func GetSchemaNames() []SchemaInfo {
	// Directory-based versioned schemas (v1.0.0 is current version)
	knownSchemas := map[string]string{
		"goneat-config-v1.0.0":       "embedded_schemas/schemas/config/v1.0.0/goneat-config.yaml",
		"dates-v1.0.0":               "embedded_schemas/schemas/config/v1.0.0/dates.yaml",
		"lifecycle-phase-v1.0.0":     "embedded_schemas/schemas/config/v1.0.0/lifecycle-phase.json",
		"release-phase-v1.0.0":       "embedded_schemas/schemas/config/v1.0.0/release-phase.json",
		"security-policy-v1.0.0":     "embedded_schemas/schemas/config/v1.0.0/security-policy.yaml",
		"suppression-report-v1.0.0":  "embedded_schemas/schemas/output/v1.0.0/suppression-report.yaml",
		"hooks-manifest-v1.0.0":      "embedded_schemas/schemas/work/v1.0.0/hooks-manifest.yaml",
		"work-manifest-v1.0.0":       "embedded_schemas/schemas/work/v1.0.0/work-manifest.yaml",
		"docs-embed-manifest-v1.0.0": "embedded_schemas/schemas/content/v1.0.0/docs-embed-manifest.json",
		"content-find-report-v1.0.0": "embedded_schemas/schemas/output/v1.0.0/content-find-report.json",
	}

	var infos []SchemaInfo
	for name, path := range knownSchemas {
		// Verify the schema exists
		if _, ok := GetSchema(path); ok {
			draft := detectDraft(path)
			infos = append(infos, SchemaInfo{Name: name, Path: path, Draft: draft})
		}
	}

	return infos
}

// detectDraft heuristically detects draft from schema bytes via $schema key.
func detectDraft(path string) string {
	bytes, ok := GetSchema(path)
	if !ok {
		return "Unknown (07/2020-12 supported)"
	}
	var doc interface{}
	err := yaml.Unmarshal(bytes, &doc)
	if err != nil {
		err = json.Unmarshal(bytes, &doc)
		if err != nil {
			return "Unknown (07/2020-12 supported)"
		}
	}
	if m, ok := doc.(map[string]interface{}); ok {
		if v, ok := m["$schema"].(string); ok {
			if strings.Contains(v, "draft-07") {
				return "Draft-07"
			}
			if strings.Contains(v, "2020-12") {
				return "Draft-2020-12"
			}
		}
	}
	return "Unknown (07/2020-12 supported)"
}

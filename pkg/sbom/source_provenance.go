package sbom

import (
	"encoding/json"
	"fmt"
	"net/url"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"time"
)

type sourceJSONEdit struct {
	object map[string]any
	key    string
	before any
	after  any
	exists bool
}

type sourceMapper struct {
	capture *sourceCapture
	edits   []sourceJSONEdit
}

func (m *sourceMapper) set(object map[string]any, key string, value any) {
	before, exists := object[key]
	m.edits = append(m.edits, sourceJSONEdit{object, key, before, value, exists})
	object[key] = value
}

// path only rewrites an entire absolute path value contained by the snapshot.
// Prefix lookalikes, relative locations, opaque IDs, and descriptive prose are
// not paths to rewrite. Residual snapshot references are rejected separately.
func (m *sourceMapper) path(object map[string]any, key string, relative bool) {
	value, ok := object[key].(string)
	if !ok || !filepath.IsAbs(value) || !containedSourcePath(m.capture.path, value) {
		return
	}
	name, err := filepath.Rel(m.capture.path, value)
	if err != nil {
		return
	}
	replacement := filepath.ToSlash(name)
	if !relative {
		replacement = filepath.ToSlash(filepath.Join(m.capture.original, name))
	}
	m.set(object, key, replacement)
}

func sourceObject(value any) map[string]any {
	object, _ := value.(map[string]any)
	return object
}

func sourceArray(value any) []any {
	array, _ := value.([]any)
	return array
}

func (m *sourceMapper) component(component map[string]any) {
	if component == nil {
		return
	}
	if component["type"] == "file" {
		m.path(component, "name", false)
	}
	for _, item := range sourceArray(component["properties"]) {
		property := sourceObject(item)
		name, _ := property["name"].(string)
		parts := strings.Split(name, ":")
		if len(parts) == 4 && parts[0] == "syft" && parts[1] == "location" && (parts[3] == "path" || parts[3] == "accessPath") {
			if index, err := strconv.Atoi(parts[2]); err == nil && index >= 0 {
				m.path(property, "value", false)
			}
		}
	}
	evidence := sourceObject(component["evidence"])
	for _, item := range sourceArray(evidence["occurrences"]) {
		m.path(sourceObject(item), "location", false)
	}
	for _, child := range sourceArray(component["components"]) {
		m.component(sourceObject(child))
	}
	pedigree := sourceObject(component["pedigree"])
	for _, key := range []string{"ancestors", "descendants", "variants"} {
		for _, child := range sourceArray(pedigree[key]) {
			m.component(sourceObject(child))
		}
	}
}

func normalizeSourceProvenance(content []byte, format string, capture *sourceCapture, options SourceOptions) ([]byte, error) {
	original, err := decodeSourceJSON(content)
	if err != nil {
		return nil, err
	}
	if err := validateSourceDocument(original, format); err != nil {
		return nil, err
	}
	if err := validateSourceReferences(original, format); err != nil {
		return nil, err
	}
	document, err := decodeSourceJSON(content)
	if err != nil {
		return nil, err
	}
	m := sourceMapper{capture: capture}
	scope, err := json.Marshal(struct {
		Kind              string   `json:"kind"`
		Subject           string   `json:"subject"`
		ManifestSHA256    string   `json:"manifest_sha256"`
		NoIgnore          bool     `json:"no_ignore"`
		ForceInclude      []string `json:"force_include"`
		Excludes          []string `json:"literal_excludes"`
		ProtectedEvidence []string `json:"protected_root_go_evidence"`
		IgnorePolicy      string   `json:"ignore_policy"`
		CollectorPolicy   string   `json:"collector_policy"`
	}{"scoped-source-inventory", capture.original, capture.digest, options.NoIgnore, options.ForceInclude, capture.excludes, capture.protectedEvidence, "ordered-root-ignore; defaults/configured-hard; root-go-evidence-protected", sourceCollectorPolicy})
	if err != nil {
		return nil, err
	}
	switch format {
	case "cyclonedx-json":
		for _, component := range sourceArray(document["components"]) {
			m.component(sourceObject(component))
		}
		metadata := sourceObject(document["metadata"])
		if metadata == nil {
			metadata = make(map[string]any)
			m.set(document, "metadata", metadata)
		}
		m.component(sourceObject(metadata["component"]))
		tools := sourceObject(metadata["tools"])
		for _, component := range sourceArray(tools["components"]) {
			m.component(sourceObject(component))
		}
		properties := append([]any{}, sourceArray(metadata["properties"])...)
		for _, property := range properties {
			if sourceObject(property)["name"] == "goneat:source:provenance" {
				return nil, fmt.Errorf("collector already supplied goneat source provenance")
			}
		}
		properties = append(properties, map[string]any{"name": "goneat:source:provenance", "value": string(scope)})
		m.set(metadata, "properties", properties)
	case "spdx-json":
		for _, file := range sourceArray(document["files"]) {
			m.path(sourceObject(file), "fileName", true)
		}
		annotations := append([]any{}, sourceArray(document["annotations"])...)
		annotations = append(annotations, map[string]any{
			"annotationDate": time.Now().UTC().Format(time.RFC3339),
			"annotationType": "OTHER", "annotator": "Tool: goneat",
			"comment": "goneat:source:provenance=" + string(scope),
		})
		m.set(document, "annotations", annotations)
	}
	// Reverse the explicit schema-qualified edits and require whole-document
	// semantic equality. This includes unknown members, all inventory values,
	// numeric tokens, IDs, references, and every graph edge (including order).
	for i := len(m.edits) - 1; i >= 0; i-- {
		edit := m.edits[i]
		if edit.exists {
			edit.object[edit.key] = edit.before
		} else {
			delete(edit.object, edit.key)
		}
	}
	if !reflect.DeepEqual(original, document) {
		return nil, fmt.Errorf("source SBOM provenance mapping changed collected inventory")
	}
	for _, edit := range m.edits {
		edit.object[edit.key] = edit.after
	}
	if err := rejectSourceResidue(document, capture.path); err != nil {
		return nil, err
	}
	if err := validateSourceDocument(document, format); err != nil {
		return nil, err
	}
	if err := validateSourceReferences(document, format); err != nil {
		return nil, err
	}
	return json.Marshal(document)
}

func rejectSourceResidue(document any, snapshot string) error {
	root := filepath.ToSlash(snapshot)
	needles := []string{root, snapshot, url.PathEscape(root), url.QueryEscape(root), (&url.URL{Path: root}).EscapedPath()}
	var check func(any) error
	check = func(value any) error {
		switch value := value.(type) {
		case string:
			variants := []string{value, strings.ReplaceAll(value, "\\", "/")}
			if decoded, err := url.PathUnescape(value); err == nil {
				variants = append(variants, decoded)
			}
			for _, variant := range variants {
				for _, needle := range needles {
					if filepath.Separator == '\\' {
						variant, needle = strings.ToLower(variant), strings.ToLower(needle)
					}
					if strings.Contains(variant, needle) {
						return fmt.Errorf("source SBOM contains an unmapped private snapshot reference")
					}
				}
			}
		case map[string]any:
			for key, child := range value {
				if err := check(key); err != nil {
					return err
				}
				if err := check(child); err != nil {
					return err
				}
			}
		case []any:
			for _, child := range value {
				if err := check(child); err != nil {
					return err
				}
			}
		}
		return nil
	}
	return check(document)
}

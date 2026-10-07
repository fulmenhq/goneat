package sbom

import (
	"encoding/json"
	"net/url"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func sourceMappingFixture(t *testing.T, format string) ([]byte, *sourceCapture) {
	t.Helper()
	root := t.TempDir()
	capture := &sourceCapture{path: filepath.Join(root, "private"), original: filepath.Join(root, "original"), digest: strings.Repeat("a", 64), excludes: []string{"./bin/sibling"}}
	var document map[string]any
	if format == "cyclonedx-json" {
		document = map[string]any{
			"bomFormat": "CycloneDX", "specVersion": "1.6", "version": 1,
			"components": []any{
				map[string]any{"type": "file", "bom-ref": "opaque-file", "name": filepath.Join(capture.path, "bin", "named")},
				map[string]any{"type": "library", "bom-ref": "pkg-1", "name": "example", "version": "1.0", "purl": "pkg:generic/example@1.0", "properties": []any{map[string]any{"name": "syft:location:0:path", "value": "bin/named"}}},
				map[string]any{"type": "library", "bom-ref": "pkg-2", "name": "example", "version": "2.0", "purl": "pkg:generic/example@2.0"},
			},
			"dependencies": []any{map[string]any{"ref": "pkg-1", "dependsOn": []any{"pkg-2", "opaque-file"}}},
		}
	} else {
		document = map[string]any{
			"spdxVersion": "SPDX-2.3", "dataLicense": "CC0-1.0", "SPDXID": "SPDXRef-DOCUMENT", "name": "original",
			"documentNamespace": "https://example.org/document/test",
			"creationInfo":      map[string]any{"creators": []any{"Tool: syft-1.54.0"}, "created": "2026-10-05T00:00:00Z"},
			"files":             []any{map[string]any{"SPDXID": "SPDXRef-file", "fileName": filepath.Join(capture.path, "bin", "named"), "checksums": []any{map[string]any{"algorithm": "SHA256", "checksumValue": strings.Repeat("b", 64)}}}},
			"packages": []any{
				map[string]any{"SPDXID": "SPDXRef-pkg1", "name": "example", "versionInfo": "1.0", "downloadLocation": "NOASSERTION"},
				map[string]any{"SPDXID": "SPDXRef-pkg2", "name": "example", "versionInfo": "2.0", "downloadLocation": "NOASSERTION"},
			},
			"relationships": []any{map[string]any{"spdxElementId": "SPDXRef-pkg1", "relatedSpdxElement": "SPDXRef-file", "relationshipType": "CONTAINS"}},
		}
	}
	content, err := json.Marshal(document)
	if err != nil {
		t.Fatal(err)
	}
	return content, capture
}

func TestSourceProvenancePreservesInventory(t *testing.T) {
	for _, format := range []string{"cyclonedx-json", "spdx-json"} {
		t.Run(format, func(t *testing.T) {
			content, capture := sourceMappingFixture(t, format)
			mapped, err := normalizeSourceProvenance(content, format, capture, SourceOptions{ForceInclude: []string{"bin/named"}})
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(string(mapped), "private") || !strings.Contains(string(mapped), "scoped-source-inventory") || !strings.Contains(string(mapped), capture.digest) {
				t.Fatalf("incorrect provenance: %s", mapped)
			}
			before, _ := decodeSourceJSON(content)
			after, _ := decodeSourceJSON(mapped)
			key := "relationships"
			if format == "cyclonedx-json" {
				key = "dependencies"
				components := sourceArray(after["components"])
				if len(components) != 3 || sourceObject(components[0])["bom-ref"] != "opaque-file" || sourceObject(components[0])["name"] != filepath.ToSlash(filepath.Join(capture.original, "bin", "named")) {
					t.Fatal("file identity or inventory changed")
				}
				if !reflect.DeepEqual(sourceArray(before["components"])[1:], components[1:]) {
					t.Fatal("package identity or relative locations changed")
				}
			} else {
				if !reflect.DeepEqual(before["packages"], after["packages"]) || sourceObject(sourceArray(after["files"])[0])["fileName"] != "bin/named" {
					t.Fatal("packages changed or file path not root-relative")
				}
			}
			if !reflect.DeepEqual(before[key], after[key]) {
				t.Fatal("graph edges changed")
			}
		})
	}
}

func TestSourceProvenanceRecordsRootGoProtection(t *testing.T) {
	for _, format := range []string{"cyclonedx-json", "spdx-json"} {
		t.Run(format, func(t *testing.T) {
			content, capture := sourceMappingFixture(t, format)
			capture.protectedEvidence = []string{"go.mod", "go.sum"}
			mapped, err := normalizeSourceProvenance(content, format, capture, SourceOptions{})
			if err != nil {
				t.Fatal(err)
			}
			document, err := decodeSourceJSON(mapped)
			if err != nil {
				t.Fatal(err)
			}
			var scopeJSON string
			if format == "cyclonedx-json" {
				for _, value := range sourceArray(sourceObject(document["metadata"])["properties"]) {
					property := sourceObject(value)
					if property["name"] == "goneat:source:provenance" {
						scopeJSON, _ = property["value"].(string)
					}
				}
			} else {
				for _, value := range sourceArray(document["annotations"]) {
					comment, _ := sourceObject(value)["comment"].(string)
					if strings.HasPrefix(comment, "goneat:source:provenance=") {
						scopeJSON = strings.TrimPrefix(comment, "goneat:source:provenance=")
					}
				}
			}
			var scope struct {
				ProtectedEvidence []string `json:"protected_root_go_evidence"`
				IgnorePolicy      string   `json:"ignore_policy"`
				CollectorPolicy   string   `json:"collector_policy"`
			}
			if err := json.Unmarshal([]byte(scopeJSON), &scope); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(scope.ProtectedEvidence, capture.protectedEvidence) || !strings.Contains(scope.IgnorePolicy, "ordered-root-ignore") || scope.CollectorPolicy != sourceCollectorPolicy {
				t.Fatal("standalone SBOM lacks actual root evidence exceptions/selection policy")
			}
		})
	}
}

func TestSourceProvenanceRejectsResidueAndInvalidReferences(t *testing.T) {
	for _, mutation := range []string{"unknown-location", "prefix-lookalike", "opaque-id", "dangling", "duplicate-id"} {
		t.Run(mutation, func(t *testing.T) {
			content, capture := sourceMappingFixture(t, "cyclonedx-json")
			document, _ := decodeSourceJSON(content)
			components := sourceArray(document["components"])
			file := sourceObject(components[0])
			switch mutation {
			case "unknown-location":
				file["description"] = "unrecognized staging provenance " + capture.path
			case "prefix-lookalike":
				file["name"] = capture.path + "-sibling/file"
			case "opaque-id":
				file["bom-ref"] = capture.path + "/opaque"
				sourceObject(sourceArray(document["dependencies"])[0])["dependsOn"] = []any{file["bom-ref"]}
			case "dangling":
				sourceObject(sourceArray(document["dependencies"])[0])["dependsOn"] = []any{"missing"}
			case "duplicate-id":
				sourceObject(components[2])["bom-ref"] = "pkg-1"
			}
			content, err := json.Marshal(document)
			if err != nil {
				t.Fatal(err)
			}
			if output, err := normalizeSourceProvenance(content, "cyclonedx-json", capture, SourceOptions{}); err == nil || output != nil {
				t.Fatalf("accepted invalid mapping %s: %v", mutation, err)
			}
		})
	}
}

func TestSourceProvenancePreservesNumericEvidence(t *testing.T) {
	content, capture := sourceMappingFixture(t, "cyclonedx-json")
	document, err := decodeSourceJSON(content)
	if err != nil {
		t.Fatal(err)
	}
	component := sourceObject(sourceArray(document["components"])[1])
	component["evidence"] = map[string]any{
		"identity":    map[string]any{"field": "purl", "confidence": json.Number("0.1234567890123456789")},
		"occurrences": []any{map[string]any{"location": "bin/named", "line": json.Number("9007199254740993")}},
	}
	content, err = json.Marshal(document)
	if err != nil {
		t.Fatal(err)
	}
	mapped, err := normalizeSourceProvenance(content, "cyclonedx-json", capture, SourceOptions{})
	if err != nil {
		t.Fatal(err)
	}
	for _, value := range []string{"0.1234567890123456789", "9007199254740993"} {
		if !strings.Contains(string(mapped), value) {
			t.Fatalf("numeric evidence %s changed", value)
		}
	}
}

func TestSourceProvenanceResidualEncodedPaths(t *testing.T) {
	root := filepath.Join(t.TempDir(), "private snapshot")
	paths := []string{root, filepath.ToSlash(root), strings.ReplaceAll(url.PathEscape(root), "%2F", "%2f")}
	if filepath.Separator == '\\' {
		paths = append(paths, strings.ToLower(filepath.ToSlash(root)))
	}
	for _, path := range paths {
		if err := rejectSourceResidue(map[string]any{"unknown": path}, root); err == nil {
			t.Fatalf("unmapped encoded staging path accepted: %s", path)
		}
	}
}

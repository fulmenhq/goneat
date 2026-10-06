package sbom

import (
	"encoding/json"
	"testing"
)

func TestSourceReferencesEvidenceAndAnnotations(t *testing.T) {
	for _, version := range []string{"1.6", "1.7"} {
		for _, reference := range []string{"pkg-1", "missing", "urn:cdx:00000000-0000-0000-0000-000000000001/1#external-component"} {
			t.Run(version+"/"+reference, func(t *testing.T) {
				content, capture := sourceMappingFixture(t, "cyclonedx-json")
				document, err := decodeSourceJSON(content)
				if err != nil {
					t.Fatal(err)
				}
				document["specVersion"] = version
				document["annotations"] = []any{map[string]any{
					"subjects": []any{reference}, "timestamp": "2026-10-05T00:00:00Z", "text": "retained annotation",
					"annotator": map[string]any{"component": map[string]any{"type": "application", "name": "collector", "bom-ref": "tool-ref"}},
				}}
				content, err = json.Marshal(document)
				if err != nil {
					t.Fatal(err)
				}
				_, err = normalizeSourceProvenance(content, "cyclonedx-json", capture, SourceOptions{})
				if (err != nil) != (reference == "missing") {
					t.Fatalf("annotation reference %q: %v", reference, err)
				}
			})
		}
	}
}

func TestSourceReferencesRejectsUnresolvedEvidenceTool(t *testing.T) {
	content, capture := sourceMappingFixture(t, "cyclonedx-json")
	document, err := decodeSourceJSON(content)
	if err != nil {
		t.Fatal(err)
	}
	component := sourceObject(sourceArray(document["components"])[1])
	component["evidence"] = map[string]any{"identity": map[string]any{"field": "purl", "tools": []any{"missing-tool"}}}
	if err := validateSourceDocument(document, "cyclonedx-json"); err != nil {
		t.Fatalf("evidence fixture schema: %v", err)
	}
	content, err = json.Marshal(document)
	if err != nil {
		t.Fatal(err)
	}
	if output, err := normalizeSourceProvenance(content, "cyclonedx-json", capture, SourceOptions{}); err == nil || output != nil {
		t.Fatal("accepted a dangling reference in component evidence")
	}
}

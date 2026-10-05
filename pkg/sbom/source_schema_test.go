package sbom

import (
	"net/url"
	"testing"
)

func TestSourceSchemasOffline(t *testing.T) {
	schemas, err := sourceSchemas()
	if err != nil || len(schemas) != 3 {
		t.Fatalf("offline schema closure: %d schemas, %v", len(schemas), err)
	}
	for _, fixture := range []struct{ format, body string }{
		{"cyclonedx-json", `{"bomFormat":"CycloneDX","specVersion":"1.6","version":1,"components":[]}`},
		{"cyclonedx-json", `{"bomFormat":"CycloneDX","specVersion":"1.7","version":1,"components":[]}`},
		{"spdx-json", `{"spdxVersion":"SPDX-2.3","dataLicense":"CC0-1.0","SPDXID":"SPDXRef-DOCUMENT","name":"source","documentNamespace":"https://example.org/sbom/test","creationInfo":{"creators":["Tool: syft-1.54.0"],"created":"2026-10-05T00:00:00Z"}}`},
	} {
		document, err := decodeSourceJSON([]byte(fixture.body))
		if err != nil {
			t.Fatal(err)
		}
		if err := validateSourceDocument(document, fixture.format); err != nil {
			t.Fatalf("%s valid fixture: %v", fixture.format, err)
		}
		delete(document, "bomFormat")
		delete(document, "SPDXID")
		if err := validateSourceDocument(document, fixture.format); err == nil {
			t.Fatalf("%s accepted missing required field", fixture.format)
		}
	}
}

func TestSourceSchemasRejectUnbundledReference(t *testing.T) {
	base, err := url.Parse("https://example.org/schema")
	if err != nil {
		t.Fatal(err)
	}
	document := map[string]any{"$ref": "https://example.invalid/not-bundled.json"}
	if err := checkSourceSchemaRefs(document, base, map[string]map[string]any{}); err == nil {
		t.Fatal("accepted a network schema reference")
	}
}

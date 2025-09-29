package signature

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadDefaultManifestContainsEmbeddedSignatures(t *testing.T) {
	t.Setenv("GONEAT_HOME", t.TempDir())

	manifest, err := LoadDefaultManifest()
	if err != nil {
		t.Fatalf("LoadDefaultManifest() error = %v", err)
	}
	if len(manifest.Signatures) == 0 {
		t.Fatal("expected embedded signatures to be present")
	}

	if !manifestHasID(manifest, "json-schema-draft-07") {
		t.Error("embedded manifest is missing json-schema-draft-07 signature")
	}
}

func TestLoadDefaultManifestOverrides(t *testing.T) {
	home := t.TempDir()
	t.Setenv("GONEAT_HOME", home)

	configDir := filepath.Join(home, "config")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatalf("failed creating config dir: %v", err)
	}

	override := `version: v1
signatures:
  - id: json-schema-draft-07
    category: json-schema
    description: Custom override draft-07
    confidence_threshold: 0.9
    matchers:
      - type: contains
        value: custom
`
	if err := os.WriteFile(filepath.Join(configDir, "signatures.yaml"), []byte(override), 0o644); err != nil {
		t.Fatalf("write override: %v", err)
	}

	manifest, err := LoadDefaultManifest()
	if err != nil {
		t.Fatalf("LoadDefaultManifest() error = %v", err)
	}

	sig, ok := findSignature(manifest, "json-schema-draft-07")
	if !ok {
		t.Fatal("expected override signature to exist")
	}
	if sig.Description != "Custom override draft-07" {
		t.Fatalf("override description not applied: %s", sig.Description)
	}
	if sig.ConfidenceThreshold != 0.9 {
		t.Fatalf("override threshold not applied: %f", sig.ConfidenceThreshold)
	}
	if len(sig.Matchers) != 1 || sig.Matchers[0].Value != "custom" {
		t.Fatalf("override matchers not applied: %+v", sig.Matchers)
	}
}

func manifestHasID(manifest *Manifest, id string) bool {
	_, ok := findSignature(manifest, id)
	return ok
}

func findSignature(manifest *Manifest, id string) (Signature, bool) {
	target := strings.ToLower(id)
	for _, sig := range manifest.Signatures {
		if strings.ToLower(sig.ID) == target {
			return sig, true
		}
	}
	return Signature{}, false
}

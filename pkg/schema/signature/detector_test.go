package signature

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDetectorDetectJsonSchema(t *testing.T) {
	manifest := &Manifest{
		Version: "vtest",
		Signatures: []Signature{
			{
				ID:                  "json-schema-draft-07",
				Category:            "json-schema",
				ConfidenceThreshold: 0.6,
				Matchers: []Matcher{
					{Type: "contains", Value: "\"$schema\"", Weight: 0.3},
					{Type: "regex", Pattern: "draft-07", Weight: 0.7},
				},
			},
		},
	}

	detector, err := NewDetector(manifest)
	if err != nil {
		t.Fatalf("NewDetector() error = %v", err)
	}

	snippet := []byte(`{"$schema":"https://json-schema.org/draft-07/schema#","title":"Example"}`)
	match, ok := detector.Detect("schema.json", snippet, DetectOptions{})
	if !ok {
		t.Fatalf("expected detection to succeed")
	}
	if match.Signature.ID != "json-schema-draft-07" {
		t.Fatalf("unexpected signature id: %s", match.Signature.ID)
	}
	if match.Score <= 0.6 {
		t.Fatalf("expected score > 0.6, got %f", match.Score)
	}
}

func TestDetectorFiltersByCategory(t *testing.T) {
	manifest := &Manifest{
		Signatures: []Signature{
			{
				ID:       "json",
				Category: "json",
				Matchers: []Matcher{{Type: "contains", Value: "json", Weight: 1}},
			},
			{
				ID:       "yaml",
				Category: "yaml",
				Matchers: []Matcher{{Type: "contains", Value: "yaml", Weight: 1}},
			},
		},
	}

	detector, err := NewDetector(manifest)
	if err != nil {
		t.Fatalf("NewDetector() error = %v", err)
	}

	snippet := []byte("yaml document")
	opts := DetectOptions{AllowedCategories: map[string]struct{}{"yaml": {}}}
	match, ok := detector.Detect("file.yaml", snippet, opts)
	if !ok || match.Signature.ID != "yaml" {
		t.Fatalf("expected yaml match, got %+v", match)
	}

	opts = DetectOptions{AllowedCategories: map[string]struct{}{"json": {}}}
	if _, ok := detector.Detect("file.yaml", snippet, opts); ok {
		t.Fatalf("expected detection to be filtered out")
	}
}

func TestDetectorDetectProtobuf(t *testing.T) {
	t.Setenv("GONEAT_HOME", t.TempDir())

	manifest, err := LoadDefaultManifest()
	if err != nil {
		t.Fatalf("LoadDefaultManifest() error = %v", err)
	}

	detector, err := NewDetector(manifest)
	if err != nil {
		t.Fatalf("NewDetector() error = %v", err)
	}

	protoPath := filepath.Join("testdata", "proto", "helloworld.proto")
	snippet, err := os.ReadFile(protoPath)
	if err != nil {
		t.Fatalf("ReadFile(%s) error = %v", protoPath, err)
	}

	match, ok := detector.Detect(protoPath, snippet, DetectOptions{})
	if !ok {
		t.Fatalf("expected protobuf detection to succeed")
	}
	if match.Signature.ID != "protobuf-schema" {
		t.Fatalf("unexpected signature id: %s", match.Signature.ID)
	}
	if match.Score <= 0.5 {
		t.Fatalf("expected score > 0.5, got %f", match.Score)
	}
}

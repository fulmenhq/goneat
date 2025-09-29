package mapping

import "testing"

func TestResolverMatchesPathRules(t *testing.T) {
	manifest := Manifest{
		Version: ManifestVersionV1,
		Mappings: []MappingRule{
			{Pattern: "config/app.yaml", SchemaID: "app-schema", Source: SourceEmbedded},
			{Pattern: "**/*.config.yaml", SchemaID: "generic-schema", Source: SourceEmbedded},
		},
	}
	resolver := NewResolver(manifest)

	res, ok := resolver.Resolve("config/app.yaml")
	if !ok {
		t.Fatalf("expected resolution")
	}
	if res.SchemaID != "app-schema" {
		t.Fatalf("unexpected schema id %s", res.SchemaID)
	}

	// Ensure fallback pattern matches when direct rule missing.
	res, ok = resolver.Resolve("services/auth.config.yaml")
	if !ok {
		t.Fatalf("expected fallback resolution")
	}
	if res.SchemaID != "generic-schema" {
		t.Fatalf("unexpected fallback schema id %s", res.SchemaID)
	}

	// Check metrics.
	metrics := resolver.Metrics()
	if metrics.Mapped != 2 || metrics.FilesEvaluated != 2 {
		t.Fatalf("unexpected metrics: %+v", metrics)
	}
}

func TestResolverRespectsExclusions(t *testing.T) {
	manifest := Manifest{
		Version:    ManifestVersionV1,
		Mappings:   []MappingRule{{Pattern: "**/*.yaml", SchemaID: "generic"}},
		Exclusions: []ExclusionRule{{Pattern: "tmp/**/*.yaml", Reason: "temp", Action: ExclusionSkip}},
	}
	resolver := NewResolver(manifest)

	res, ok := resolver.Resolve("tmp/example.yaml")
	if !ok || !res.Excluded {
		t.Fatalf("expected exclusion, got %#v (ok=%v)", res, ok)
	}

	metrics := resolver.Metrics()
	if metrics.Excluded != 1 {
		t.Fatalf("expected excluded metric to be 1, got %+v", metrics)
	}
}

package mapping

func boolPtr(v bool) *bool { return &v }

func floatPtr(v float64) *float64 { return &v }

// builtinManifestV1 holds the default mapping behaviour distributed with goneat.
var builtinManifestV1 = Manifest{
	Version: ManifestVersionV1,
	Config: ConfigSettings{
		InferenceEnabled:      boolPtr(true),
		FallbackToContent:     boolPtr(true),
		StrictMode:            boolPtr(false),
		CacheInferences:       boolPtr(true),
		RespectExclusions:     boolPtr(true),
		AutoSuggestExclusions: boolPtr(false),
		MinConfidence:         floatPtr(0.75),
	},
	Mappings: []MappingRule{
		{
			Pattern:  "config/ascii/terminal-overrides.yaml",
			SchemaID: "terminal-overrides-v1.0.0",
			Source:   SourceEmbedded,
			Priority: PriorityHigh,
		},
		{
			Pattern:  ".goneat/config.yaml",
			SchemaID: "goneat-config-v1.0.0",
			Source:   SourceEmbedded,
			Priority: PriorityHigh,
		},
		{
			Pattern:  ".goneat/hooks.yaml",
			SchemaID: "hooks-manifest-v1.0.0",
			Source:   SourceEmbedded,
			Priority: PriorityHigh,
		},
		{
			Pattern:         "**/*-config.yaml",
			InferenceMethod: InferenceContent,
			Priority:        PriorityNormal,
		},
		{
			Pattern:  "**/database.yaml",
			SchemaID: "database-config-v1.0.0",
			Source:   SourceEmbedded,
			Priority: PriorityNormal,
		},
		{
			Pattern:         "schemas/**/*.yaml",
			InferenceMethod: InferenceMetaSchema,
			Priority:        PriorityLow,
		},
	},
	Exclusions: []ExclusionRule{
		{
			Pattern: "test/fixtures/**/*.yaml",
			Reason:  "Test fixtures, not validated",
			Action:  ExclusionSkip,
		},
		{
			Pattern: "docs/examples/**/*.json",
			Reason:  "Documentation examples",
			Action:  ExclusionSkip,
		},
		{
			Pattern:       "tools/*/output.yaml",
			ExcludeSchema: "goneat-config-v1.0.0",
			Reason:        "Tool outputs should not use goneat config schema",
			Action:        ExclusionRetryInference,
		},
		{
			Pattern:       "logs/*.json",
			Reason:        "Log files",
			Action:        ExclusionSkip,
			ExcludeSchema: "*",
		},
	},
}

// BuiltinManifest returns a clone of the built-in manifest definition.
func BuiltinManifest() Manifest {
	return builtinManifestV1.Clone()
}

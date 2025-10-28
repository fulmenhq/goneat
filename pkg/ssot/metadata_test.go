package ssot

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/fulmenhq/goneat/pkg/schema"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestGenerateSlug(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"simple", "crucible", "crucible"},
		{"with spaces", "my source", "my-source"},
		{"uppercase", "CruCible", "crucible"},
		{"special chars", "test@repo#2", "testrepo2"},
		{"multiple spaces", "a  b  c", "a--b--c"},
		{"leading/trailing spaces", " test ", "-test-"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := generateSlug(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestDetectVersion(t *testing.T) {
	t.Run("version file exists", func(t *testing.T) {
		// Create temp directory with VERSION file
		tmpDir := t.TempDir()
		versionFile := filepath.Join(tmpDir, "VERSION")
		err := os.WriteFile(versionFile, []byte("v1.2.3\n"), 0600)
		require.NoError(t, err)

		version, source, err := detectVersion(tmpDir, "VERSION")
		require.NoError(t, err)
		assert.Equal(t, "v1.2.3", version)
		assert.Equal(t, "VERSION", source)
	})

	t.Run("version file missing", func(t *testing.T) {
		tmpDir := t.TempDir()

		version, source, err := detectVersion(tmpDir, "VERSION")
		require.NoError(t, err)
		assert.Equal(t, "", version)
		assert.Equal(t, "not-found", source)
	})

	t.Run("custom version file", func(t *testing.T) {
		tmpDir := t.TempDir()
		versionFile := filepath.Join(tmpDir, "version.txt")
		err := os.WriteFile(versionFile, []byte("2025.10.2"), 0600)
		require.NoError(t, err)

		version, source, err := detectVersion(tmpDir, "version.txt")
		require.NoError(t, err)
		assert.Equal(t, "2025.10.2", version)
		assert.Equal(t, "version.txt", source)
	})
}

func TestIntrospectRepository(t *testing.T) {
	t.Run("non-git directory", func(t *testing.T) {
		tmpDir := t.TempDir()

		commit, ref, dirty, reason, repoRoot, err := introspectRepository(tmpDir)
		require.NoError(t, err)
		assert.Equal(t, "", commit)
		assert.Equal(t, "", ref)
		assert.True(t, dirty)
		assert.Equal(t, "non-git", reason)
		assert.Equal(t, "", repoRoot)
	})

	t.Run("git repository", func(t *testing.T) {
		// Note: This test assumes we're running in goneat's own repo
		// Skip if not in a git repo
		if _, err := os.Stat(".git"); os.IsNotExist(err) {
			t.Skip("Not in a git repository")
		}

		// Get current directory (should be goneat repo root or pkg/ssot)
		wd, err := os.Getwd()
		require.NoError(t, err)

		// Try to find repo root
		repoRoot := wd
		for i := 0; i < 5; i++ {
			if _, err := os.Stat(filepath.Join(repoRoot, ".git")); err == nil {
				break
			}
			repoRoot = filepath.Dir(repoRoot)
		}

		commit, ref, dirty, reason, detectedRoot, err := introspectRepository(repoRoot)
		require.NoError(t, err)
		assert.NotEmpty(t, commit)
		assert.Len(t, commit, 40) // Full SHA
		assert.NotEmpty(t, detectedRoot, "Repository root should be detected")
		// ref might be "HEAD" or branch name
		assert.NotEmpty(t, ref)
		// dirty state depends on working tree
		if dirty {
			assert.NotEmpty(t, reason)
		}
	})
}

func TestBuildProvenance(t *testing.T) {
	sources := []SourceMetadata{
		{
			Name:          "crucible",
			Slug:          "crucible",
			Method:        "local_path",
			RepoURL:       "https://github.com/fulmenhq/crucible",
			LocalPath:     "../crucible",
			Ref:           "main",
			Commit:        "abc123" + string(make([]byte, 34)), // 40 chars total
			Dirty:         false,
			Version:       "2025.10.2",
			VersionSource: "VERSION",
			Outputs: map[string]string{
				"docs":    "docs/crucible-go",
				"schemas": "schemas/crucible-go",
			},
		},
	}

	provenance := buildProvenance(sources)

	assert.Equal(t, "goneat.ssot.provenance", provenance.Schema.Name)
	assert.Equal(t, "v1", provenance.Schema.Version)
	assert.NotZero(t, provenance.GeneratedAt)
	assert.Len(t, provenance.Sources, 1)
	assert.Equal(t, "crucible", provenance.Sources[0].Name)
}

func TestProvenanceJSONMarshaling(t *testing.T) {
	provenance := &Provenance{
		Schema: SchemaDescriptor{
			Name:    "goneat.ssot.provenance",
			Version: "v1",
			URL:     "https://github.com/fulmenhq/goneat/schemas/ssot/provenance.v1.json",
		},
		GeneratedAt: mustParseTime("2025-10-27T18:00:00Z"),
		Sources: []SourceMetadata{
			{
				Name:    "test",
				Slug:    "test",
				Method:  "local_path",
				Outputs: map[string]string{"docs": "docs/test"},
			},
		},
	}

	data, err := json.MarshalIndent(provenance, "", "  ")
	require.NoError(t, err)

	// Unmarshal back
	var decoded Provenance
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, provenance.Schema.Name, decoded.Schema.Name)
	assert.Equal(t, provenance.Sources[0].Name, decoded.Sources[0].Name)
}

func TestWriteAggregateProvenance(t *testing.T) {
	t.Run("dry run", func(t *testing.T) {
		provenance := &Provenance{
			Schema: SchemaDescriptor{
				Name:    "goneat.ssot.provenance",
				Version: "v1",
				URL:     "https://github.com/fulmenhq/goneat/schemas/ssot/provenance.v1.json",
			},
			GeneratedAt: mustParseTime("2025-10-27T18:00:00Z"),
			Sources:     []SourceMetadata{{Name: "test", Slug: "test", Method: "local_path"}},
		}

		tmpDir := t.TempDir()
		outputPath := filepath.Join(tmpDir, ".goneat/ssot/provenance.json")

		// Dry run should not create file
		err := writeAggregateProvenance(provenance, outputPath, true)
		require.NoError(t, err)

		_, err = os.Stat(outputPath)
		assert.True(t, os.IsNotExist(err), "File should not exist in dry-run mode")
	})

	t.Run("actual write", func(t *testing.T) {
		provenance := &Provenance{
			Schema: SchemaDescriptor{
				Name:    "goneat.ssot.provenance",
				Version: "v1",
				URL:     "https://github.com/fulmenhq/goneat/schemas/ssot/provenance.v1.json",
			},
			GeneratedAt: mustParseTime("2025-10-27T18:00:00Z"),
			Sources:     []SourceMetadata{{Name: "test", Slug: "test", Method: "local_path"}},
		}

		tmpDir := t.TempDir()
		outputPath := filepath.Join(tmpDir, ".goneat/ssot/provenance.json")

		err := writeAggregateProvenance(provenance, outputPath, false)
		require.NoError(t, err)

		// Verify file exists and is valid JSON
		data, err := os.ReadFile(outputPath)
		require.NoError(t, err)

		var decoded Provenance
		err = json.Unmarshal(data, &decoded)
		require.NoError(t, err)
		assert.Equal(t, "test", decoded.Sources[0].Name)
	})
}

func TestWritePerSourceMirror(t *testing.T) {
	source := &SourceMetadata{
		Name:   "crucible",
		Slug:   "crucible",
		Method: "local_path",
		Outputs: map[string]string{
			"docs":    "docs/crucible-go",
			"schemas": "schemas/crucible-go",
		},
	}

	t.Run("yaml format", func(t *testing.T) {
		// Use current dir as temp base since it's relative path
		origDir, err := os.Getwd()
		require.NoError(t, err)
		defer os.Chdir(origDir) // nolint: errcheck

		tmpDir := t.TempDir()
		err = os.Chdir(tmpDir)
		require.NoError(t, err)

		err = writePerSourceMirror(source, "yaml", nil, false)
		require.NoError(t, err)

		// Check file exists
		mirrorPath := ".crucible/metadata/metadata.yaml"
		_, err = os.Stat(mirrorPath)
		require.NoError(t, err)
	})

	t.Run("dry run", func(t *testing.T) {
		origDir, err := os.Getwd()
		require.NoError(t, err)
		defer os.Chdir(origDir) // nolint: errcheck

		tmpDir := t.TempDir()
		err = os.Chdir(tmpDir)
		require.NoError(t, err)

		err = writePerSourceMirror(source, "yaml", nil, true)
		require.NoError(t, err)

		// File should not exist
		mirrorPath := ".crucible/metadata/metadata.yaml"
		_, err = os.Stat(mirrorPath)
		assert.True(t, os.IsNotExist(err))
	})
}

// Helper function to parse time for tests
func mustParseTime(s string) time.Time {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		panic(err)
	}
	return t
}

func TestProvenanceSchemaValidation(t *testing.T) {
	// Create sample provenance
	provenance := &Provenance{
		Schema: SchemaDescriptor{
			Name:    "goneat.ssot.provenance",
			Version: "v1",
			URL:     "https://github.com/fulmenhq/goneat/schemas/ssot/provenance.v1.json",
		},
		GeneratedAt: time.Now().UTC(),
		Sources: []SourceMetadata{
			{
				Name:          "crucible",
				Slug:          "crucible",
				Method:        "local_path",
				RepoURL:       "https://github.com/fulmenhq/crucible",
				LocalPath:     "../crucible",
				Ref:           "main",
				Commit:        "b64d22a0f0f94e4f1f128172c04fd166cf255056",
				Dirty:         false,
				VersionFile:   "VERSION",
				Version:       "2025.10.2",
				VersionSource: "VERSION",
				Outputs: map[string]string{
					"docs":    "docs/crucible-go",
					"schemas": "schemas/crucible-go",
				},
			},
		},
	}

	// Marshal to JSON
	data, err := json.Marshal(provenance)
	require.NoError(t, err)

	// Load embedded schema
	validator, err := schema.GetEmbeddedValidator("ssot-provenance-v1")
	require.NoError(t, err, "Failed to load provenance schema")

	// Validate
	result, err := validator.ValidateBytes(data)
	require.NoError(t, err, "Validation failed with error")
	assert.True(t, result.Valid, "Provenance should validate against schema")

	if !result.Valid {
		t.Logf("Validation errors: %v", result.Errors)
	}
}

func TestSourceMetadataSchemaValidation(t *testing.T) {
	// Create single-source provenance (per-source mirror format)
	singleSource := &Provenance{
		Schema: SchemaDescriptor{
			Name:    "goneat.ssot.source-metadata",
			Version: "v1",
			URL:     "https://github.com/fulmenhq/goneat/schemas/ssot/source-metadata.v1.json",
		},
		GeneratedAt: time.Now().UTC(),
		Sources: []SourceMetadata{
			{
				Name:          "crucible",
				Slug:          "crucible",
				Method:        "local_path",
				RepoURL:       "https://github.com/fulmenhq/crucible",
				Commit:        "abc1234567890123456789012345678901234567",
				Dirty:         false,
				Version:       "2025.10.2",
				VersionSource: "VERSION",
			},
		},
	}

	// Marshal to JSON for validation (even though mirrors are YAML)
	data, err := json.Marshal(singleSource)
	require.NoError(t, err)

	// Load embedded schema
	validator, err := schema.GetEmbeddedValidator("ssot-source-metadata-v1")
	require.NoError(t, err, "Failed to load source-metadata schema")

	// Validate
	result, err := validator.ValidateBytes(data)
	require.NoError(t, err, "Validation failed with error")
	assert.True(t, result.Valid, "Source metadata should validate against schema")

	if !result.Valid {
		t.Logf("Validation errors: %v", result.Errors)
	}
}

func TestSchemaMetaValidation(t *testing.T) {
	t.Run("provenance schema validates against meta-schema", func(t *testing.T) {
		// Read provenance schema
		schemaPath := "../../schemas/ssot/provenance.v1.json"
		if _, err := os.Stat(schemaPath); os.IsNotExist(err) {
			t.Skip("Schema file not found at expected path")
		}

		schemaData, err := os.ReadFile(schemaPath)
		require.NoError(t, err)

		// Validate against 2020-12 meta-schema
		validator, err := schema.GetEmbeddedValidator("json-schema-2020-12")
		require.NoError(t, err, "Failed to load meta-schema")

		result, err := validator.ValidateBytes(schemaData)
		require.NoError(t, err, "Meta-validation failed with error")
		assert.True(t, result.Valid, "Provenance schema should be valid JSON Schema 2020-12")

		if !result.Valid {
			t.Logf("Meta-validation errors: %v", result.Errors)
		}
	})

	t.Run("source-metadata schema validates against meta-schema", func(t *testing.T) {
		// Read source-metadata schema
		schemaPath := "../../schemas/ssot/source-metadata.v1.json"
		if _, err := os.Stat(schemaPath); os.IsNotExist(err) {
			t.Skip("Schema file not found at expected path")
		}

		schemaData, err := os.ReadFile(schemaPath)
		require.NoError(t, err)

		// Validate against 2020-12 meta-schema
		validator, err := schema.GetEmbeddedValidator("json-schema-2020-12")
		require.NoError(t, err, "Failed to load meta-schema")

		result, err := validator.ValidateBytes(schemaData)
		require.NoError(t, err, "Meta-validation failed with error")
		assert.True(t, result.Valid, "Source-metadata schema should be valid JSON Schema 2020-12")

		if !result.Valid {
			t.Logf("Meta-validation errors: %v", result.Errors)
		}
	})
}

func TestPerSourceConfigOverrides(t *testing.T) {
	t.Run("custom version file", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Create custom version file
		customFile := filepath.Join(tmpDir, "version.txt")
		err := os.WriteFile(customFile, []byte("custom-v1.0.0"), 0600)
		require.NoError(t, err)

		source := Source{
			Name:      "test",
			LocalPath: tmpDir,
			Metadata: SourceMetadataConfig{
				VersionFile: "version.txt",
			},
		}

		resolved := ResolvedSource{
			Name:    "test",
			Path:    tmpDir,
			IsLocal: true,
		}

		outputs := map[string]string{"docs": "docs/test"}

		metadata, err := captureSourceMetadata(source, resolved, outputs)
		require.NoError(t, err)

		assert.Equal(t, "custom-v1.0.0", metadata.Version)
		assert.Equal(t, "version.txt", metadata.VersionFile)
	})

	t.Run("custom mirror path", func(t *testing.T) {
		origDir, err := os.Getwd()
		require.NoError(t, err)
		defer os.Chdir(origDir) // nolint: errcheck

		tmpDir := t.TempDir()
		err = os.Chdir(tmpDir)
		require.NoError(t, err)

		source := &SourceMetadata{
			Name: "test",
			Slug: "test",
		}

		sourceConfig := &Source{
			Name: "test",
			Metadata: SourceMetadataConfig{
				MirrorPath: "custom/path/metadata.yaml",
			},
		}

		err = writePerSourceMirror(source, "yaml", sourceConfig, false)
		require.NoError(t, err)

		// Check custom path was used
		_, err = os.Stat("custom/path/metadata.yaml")
		require.NoError(t, err, "Custom mirror path should be used")
	})
}

func TestYAMLMirrorFormat(t *testing.T) {
	t.Run("yaml mirror is valid yaml", func(t *testing.T) {
		origDir, err := os.Getwd()
		require.NoError(t, err)
		defer os.Chdir(origDir) // nolint: errcheck

		tmpDir := t.TempDir()
		err = os.Chdir(tmpDir)
		require.NoError(t, err)

		source := &SourceMetadata{
			Name:    "test",
			Slug:    "test",
			Method:  "local_path",
			Commit:  "abc1234567890123456789012345678901234567",
			Dirty:   false,
			Version: "1.0.0",
		}

		err = writePerSourceMirror(source, "yaml", nil, false)
		require.NoError(t, err)

		// Read and parse YAML
		data, err := os.ReadFile(".test/metadata/metadata.yaml")
		require.NoError(t, err)

		var decoded Provenance
		err = yaml.Unmarshal(data, &decoded)
		require.NoError(t, err, "YAML should be valid")

		assert.Equal(t, "test", decoded.Sources[0].Name)
		assert.Equal(t, "1.0.0", decoded.Sources[0].Version)
	})
}

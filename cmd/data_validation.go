package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/fulmenhq/goneat/pkg/schema"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

type dataValidationOptions struct {
	schemaName       string
	schemaFile       string
	refDirs          []string
	schemaResolution string
	dataFile         string
	format           string
}

func runDataValidation(cmd *cobra.Command, opts dataValidationOptions) error {
	if strings.TrimSpace(opts.dataFile) == "" {
		return fmt.Errorf("--data is required")
	}
	if strings.TrimSpace(opts.schemaName) == "" && strings.TrimSpace(opts.schemaFile) == "" {
		return fmt.Errorf("either --schema or --schema-file is required")
	}
	if strings.TrimSpace(opts.schemaName) != "" && strings.TrimSpace(opts.schemaFile) != "" {
		return fmt.Errorf("both --schema and --schema-file provided; use one")
	}

	data, err := os.ReadFile(filepath.Clean(opts.dataFile))
	if err != nil {
		return fmt.Errorf("failed to read data file %s: %w", opts.dataFile, err)
	}

	// Parse data to interface{} (handle YAML/JSON)
	var doc interface{}
	ext := strings.ToLower(filepath.Ext(opts.dataFile))
	switch ext {
	case ".yaml", ".yml":
		if err := yaml.Unmarshal(data, &doc); err != nil {
			return fmt.Errorf("failed to parse %s as YAML: %w", opts.dataFile, err)
		}
	case ".json":
		if err := json.Unmarshal(data, &doc); err != nil {
			return fmt.Errorf("failed to parse %s as JSON: %w", opts.dataFile, err)
		}
	default:
		return fmt.Errorf("unsupported data format: %s (use .yaml/.yml or .json)", ext)
	}

	if err := validateSchemaResolution(opts.schemaResolution); err != nil {
		return err
	}

	var idIndex *schema.IDIndex
	if schemaResolutionMode(opts.schemaResolution) != schemaResolutionPathOnly && len(opts.refDirs) > 0 {
		idx, err := schema.BuildIDIndexFromRefDirs(opts.refDirs)
		if err != nil {
			return err
		}
		idIndex = idx
	}

	// Load schema (embedded or file)
	var result *schema.Result
	if strings.TrimSpace(opts.schemaFile) != "" {
		schemaBytes, err := os.ReadFile(filepath.Clean(opts.schemaFile))
		if err != nil {
			return fmt.Errorf("failed to read schema file %s: %w", opts.schemaFile, err)
		}
		if idIndex != nil {
			result, err = schema.ValidateFromBytesWithIDIndex(schemaBytes, doc, idIndex)
		} else {
			result, err = schema.ValidateFromBytesWithRefDirs(schemaBytes, doc, opts.refDirs)
		}
		if err != nil {
			return fmt.Errorf("validation failed: %w", err)
		}
	} else if strings.TrimSpace(opts.schemaName) != "" {
		schemaID := strings.TrimSpace(opts.schemaName)
		if isSchemaIDURL(schemaID) && schemaResolutionMode(opts.schemaResolution) != schemaResolutionPathOnly {
			if idIndex == nil {
				return fmt.Errorf("cannot resolve schema_id %q without --ref-dir", schemaID)
			}
			entry, ok := idIndex.Get(schemaID)
			if !ok {
				return fmt.Errorf("schema_id not found in --ref-dir index: %q", schemaID)
			}
			result, err = schema.ValidateFromBytesWithIDIndex(entry.Normalized, doc, idIndex)
		} else {
			result, err = schema.Validate(doc, schemaID)
		}
		if err != nil {
			return fmt.Errorf("validation failed: %w", err)
		}
	} else {
		return fmt.Errorf("either --schema or --schema-file required")
	}

	// Output
	switch strings.ToLower(strings.TrimSpace(opts.format)) {
	case "json":
		out, _ := json.MarshalIndent(result, "", "  ")
		cmd.Printf("%s\n", out)
	case "markdown":
		if result.Valid {
			cmd.Println("✅ Validation passed")
		} else {
			cmd.Println("❌ Validation failed:")
			for _, e := range result.Errors {
				cmd.Printf("- %s: %s\n", e.Path, e.Message)
			}
		}
	default:
		cmd.Println("Invalid format; use markdown or json")
		return nil
	}

	if !result.Valid {
		return fmt.Errorf("data validation failed")
	}
	return nil
}

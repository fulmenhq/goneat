package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/fulmenhq/goneat/pkg/safeio"
	"github.com/fulmenhq/goneat/pkg/schema"
	"github.com/fulmenhq/goneat/pkg/schema/signature"
	"github.com/spf13/cobra"
)

var (
	schemaValidateSchemaID string
	schemaValidateFormat   string
)

var schemaCmd = &cobra.Command{
	Use:   "schema",
	Short: "Schema validation utilities",
	RunE: func(cmd *cobra.Command, _ []string) error {
		return cmd.Help()
	},
}

var schemaValidateSchemaCmd = &cobra.Command{
	Use:   "validate-schema [files...]",
	Short: "Validate schema files against embedded meta-schemas",
	Long:  "Validate schema files (e.g., JSON Schema drafts) against embedded meta-schemas to ensure structural correctness.",
	Args:  cobra.MinimumNArgs(1),
	RunE:  runSchemaValidateSchema,
}

func init() {
	rootCmd.AddCommand(schemaCmd)
	schemaCmd.AddCommand(schemaValidateSchemaCmd)

	schemaValidateSchemaCmd.Flags().StringVar(&schemaValidateSchemaID, "schema-id", "", "Schema signature id to validate against (e.g., json-schema-draft-07)")
	schemaValidateSchemaCmd.Flags().StringVar(&schemaValidateFormat, "format", "text", "Output format: text|json")
}

type schemaValidateResult struct {
	File     string   `json:"file"`
	SchemaID string   `json:"schema_id"`
	Valid    bool     `json:"valid"`
	Errors   []string `json:"errors,omitempty"`
}

func runSchemaValidateSchema(cmd *cobra.Command, args []string) error {
	format := strings.ToLower(schemaValidateFormat)
	if format != "text" && format != "json" {
		return fmt.Errorf("unsupported format: %s", schemaValidateFormat)
	}

	manifest, err := signature.LoadDefaultManifest()
	if err != nil {
		return fmt.Errorf("load schema manifest: %w", err)
	}

	var providedID string
	if schemaValidateSchemaID != "" {
		sig, ok := findSignature(manifest, schemaValidateSchemaID)
		if !ok {
			return fmt.Errorf("schema-id %s not found in signature manifest", schemaValidateSchemaID)
		}
		providedID = sig.ID
	}

	var detector *signature.Detector
	if providedID == "" {
		var derr error
		detector, derr = signature.NewDetector(manifest)
		if derr != nil {
			return fmt.Errorf("prepare schema detector: %w", derr)
		}
	}

	results := make([]schemaValidateResult, 0, len(args))
	var failures int

	for _, input := range args {
		cleanPath, err := safeio.CleanUserPath(input)
		if err != nil {
			failures++
			results = append(results, schemaValidateResult{
				File:   input,
				Valid:  false,
				Errors: []string{fmt.Sprintf("invalid path: %v", err)},
			})
			continue
		}

		schemaBytes, err := os.ReadFile(cleanPath)
		if err != nil {
			failures++
			results = append(results, schemaValidateResult{
				File:   cleanPath,
				Valid:  false,
				Errors: []string{fmt.Sprintf("failed to read schema: %v", err)},
			})
			continue
		}

		schemaID := providedID
		if schemaID == "" {
			match, ok := detector.Detect(cleanPath, schemaBytes, signature.DetectOptions{})
			if !ok {
				failures++
				results = append(results, schemaValidateResult{
					File:   cleanPath,
					Valid:  false,
					Errors: []string{"unable to detect schema signature"},
				})
				continue
			}
			schemaID = match.Signature.ID
		}

		valid, validationErrs, verr := validateSchemaByID(schemaID, schemaBytes)
		if verr != nil {
			failures++
			results = append(results, schemaValidateResult{
				File:     cleanPath,
				SchemaID: schemaID,
				Valid:    false,
				Errors:   []string{verr.Error()},
			})
			continue
		}
		if !valid {
			failures++
			results = append(results, schemaValidateResult{
				File:     cleanPath,
				SchemaID: schemaID,
				Valid:    false,
				Errors:   validationErrs,
			})
			continue
		}

		results = append(results, schemaValidateResult{
			File:     cleanPath,
			SchemaID: schemaID,
			Valid:    true,
		})
	}

	if format == "json" {
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		if err := enc.Encode(results); err != nil {
			return fmt.Errorf("encode JSON output: %w", err)
		}
	} else {
		for _, res := range results {
			if res.Valid {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "✅ %s", res.File)
				if res.SchemaID != "" {
					_, _ = fmt.Fprintf(cmd.OutOrStdout(), " (%s)", res.SchemaID)
				}
				_, _ = fmt.Fprintln(cmd.OutOrStdout())
				continue
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "❌ %s", res.File)
			if res.SchemaID != "" {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), " (%s)", res.SchemaID)
			}
			_, _ = fmt.Fprintln(cmd.OutOrStdout())
			for _, msg := range res.Errors {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "    - %s\n", msg)
			}
		}
	}

	if failures > 0 {
		return fmt.Errorf("%d schema file(s) failed validation", failures)
	}
	return nil
}

func findSignature(manifest *signature.Manifest, id string) (signature.Signature, bool) {
	lower := strings.ToLower(id)
	for _, sig := range manifest.Signatures {
		if strings.ToLower(sig.ID) == lower {
			return sig, true
		}
		for _, alias := range sig.Aliases {
			if strings.ToLower(alias) == lower {
				return sig, true
			}
		}
	}
	return signature.Signature{}, false
}

func validateSchemaByID(schemaID string, schemaBytes []byte) (bool, []string, error) {
	switch schemaID {
	case "json-schema-draft-07":
		return validateJSONSchemaDraft("draft-07", schemaBytes)
	case "json-schema-2020-12":
		return validateJSONSchemaDraft("2020-12", schemaBytes)
	default:
		return false, nil, fmt.Errorf("schema-id %s not supported for validation", schemaID)
	}
}

func validateJSONSchemaDraft(draft string, schemaBytes []byte) (bool, []string, error) {
	validator, err := schema.NewValidatorFromMetaSchema(draft)
	if err != nil {
		return false, nil, err
	}

	res, err := validator.ValidateBytes(schemaBytes)
	if err != nil {
		return false, nil, err
	}
	if res.Valid {
		return true, nil, nil
	}

	errs := make([]string, 0, len(res.Errors))
	for _, e := range res.Errors {
		path := strings.TrimSpace(e.Path)
		if path == "" || path == "root" {
			errs = append(errs, e.Message)
		} else {
			errs = append(errs, fmt.Sprintf("%s: %s", path, e.Message))
		}
	}
	return false, errs, nil
}

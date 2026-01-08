package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var (
	schemaValidateDataSchema           string
	schemaValidateDataSchemaFile       string
	schemaValidateDataSchemaRefDirs    []string
	schemaValidateDataFile             string
	schemaValidateDataSchemaResolution string
	schemaValidateDataFormat           string
)

var schemaValidateDataCmd = &cobra.Command{
	Use:   "validate-data --schema SCHEMA --data FILE",
	Short: "Validate data against a schema",
	Long:  "Validate a JSON/YAML data file against an embedded schema (or an arbitrary schema file).",
	RunE:  runSchemaValidateData,
}

func init() {
	schemaCmd.AddCommand(schemaValidateDataCmd)

	schemaValidateDataCmd.Flags().StringVar(&schemaValidateDataSchema, "schema", "", "Schema name (embedded) or canonical schema ID URL (mutually exclusive with --schema-file)")
	schemaValidateDataCmd.Flags().StringVar(&schemaValidateDataSchemaFile, "schema-file", "", "Path to arbitrary schema file (JSON/YAML; overrides --schema)")
	schemaValidateDataCmd.Flags().StringSliceVar(&schemaValidateDataSchemaRefDirs, "ref-dir", []string{}, "Directory tree of schema files used to resolve absolute $ref URLs offline (repeatable). Safe if it also contains --schema-file")
	schemaValidateDataCmd.Flags().StringVar(&schemaValidateDataSchemaResolution, "schema-resolution", string(schemaResolutionPreferID), "Schema resolution strategy for schema IDs (prefer-id, id-strict, path-only)")
	schemaValidateDataCmd.Flags().StringVar(&schemaValidateDataFile, "data", "", "Data file to validate (required)")
	schemaValidateDataCmd.Flags().StringVar(&schemaValidateDataFormat, "format", "markdown", "Output format (markdown, json)")

	if err := schemaValidateDataCmd.MarkFlagRequired("data"); err != nil {
		panic(fmt.Sprintf("failed to mark data flag as required: %v", err))
	}
}

func runSchemaValidateData(cmd *cobra.Command, args []string) error {
	return runDataValidation(cmd, dataValidationOptions{
		schemaName:       schemaValidateDataSchema,
		schemaFile:       schemaValidateDataSchemaFile,
		refDirs:          schemaValidateDataSchemaRefDirs,
		schemaResolution: schemaValidateDataSchemaResolution,
		dataFile:         schemaValidateDataFile,
		format:           schemaValidateDataFormat,
	})
}

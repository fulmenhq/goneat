package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fulmenhq/goneat/internal/assess"
	"github.com/fulmenhq/goneat/internal/assets"
	"github.com/fulmenhq/goneat/internal/ops"
	"github.com/fulmenhq/goneat/pkg/config"
	"github.com/fulmenhq/goneat/pkg/logger"
	"github.com/spf13/cobra"
)

var (
	validateFormat       string
	validateVerbose      bool
	validateFailOn       string
	validateTimeout      time.Duration
	validateOutput       string
	validateIncludeFiles []string
	validateExcludeFiles []string
	validateAutoDetect   bool
	validateNoIgnore     bool
	validateForceInclude []string
	validateEnableMeta   bool
	validateScope        bool
	validateListSchemas  bool

	// Data validation flags
	validateDataSchema           string
	validateSchemaFile           string
	validateSchemaRefDirs        []string
	validateDataFile             string
	validateDataSchemaResolution string
)

var validateCmd = &cobra.Command{
	Use:   "validate [target]",
	Short: "Schema-aware validation (preview)",
	Long:  "Validate JSON/YAML files with syntax checks and schema-aware processing (preview).",
	Args:  cobra.MaximumNArgs(1),
	RunE:  runValidate,
}

var validateDataCmd = &cobra.Command{
	Use:   "data --schema SCHEMA --data FILE",
	Short: "Validate data against a specific schema",
	Long:  "Validate data file against named embedded schema (e.g., --schema goneat-config-v1.0.0 --data .goneat.yaml)",
	RunE:  runValidateData,
}

func init() {
	rootCmd.AddCommand(validateCmd)

	caps := ops.GetDefaultCapabilities(ops.GroupNeat, ops.CategoryValidation)
	if err := ops.RegisterCommandWithTaxonomy("validate", ops.GroupNeat, ops.CategoryValidation, caps, validateCmd, "Schema-aware validation (preview)"); err != nil {
		panic(fmt.Sprintf("Failed to register validate command: %v", err))
	}

	validateCmd.Flags().StringVar(&validateFormat, "format", "markdown", "Output format (markdown, json, html, both)")
	validateCmd.Flags().BoolVarP(&validateVerbose, "verbose", "v", false, "Verbose output")
	validateCmd.Flags().StringVar(&validateFailOn, "fail-on", "high", "Fail if issues at or above severity (critical, high, medium, low)")
	validateCmd.Flags().DurationVar(&validateTimeout, "timeout", 3*time.Minute, "Validation timeout")
	validateCmd.Flags().StringVarP(&validateOutput, "output", "o", "", "Output file (default: stdout)")
	validateCmd.Flags().StringSliceVar(&validateIncludeFiles, "include", []string{}, "Include only these files/patterns")
	validateCmd.Flags().StringSliceVar(&validateExcludeFiles, "exclude", []string{}, "Exclude these files/patterns")
	validateCmd.Flags().BoolVar(&validateAutoDetect, "auto-detect", false, "Auto-detect schema files (preview; uses extensions)")
	validateCmd.Flags().BoolVar(&validateNoIgnore, "no-ignore", false, "Disable .goneatignore/.gitignore for discovery (scan everything in scope)")
	validateCmd.Flags().StringSliceVar(&validateForceInclude, "force-include", []string{}, "Force-include paths or globs even if ignored (repeatable). Examples: --force-include tests/fixtures/schemas/bad/** --force-include \"**/*.schema.yaml\"")
	validateCmd.Flags().BoolVar(&validateEnableMeta, "enable-meta", false, "Attempt meta-schema validation using embedded drafts (may require network for remote $refs)")
	validateCmd.Flags().BoolVar(&validateScope, "scope", false, "Limit traversal scope to include paths and force-include anchors")
	validateCmd.Flags().BoolVar(&validateListSchemas, "list-schemas", false, "List available embedded schemas with drafts")

	// Subcommand for data validation
	validateCmd.AddCommand(validateDataCmd)
	validateDataCmd.Flags().StringVar(&validateDataSchema, "schema", "", "Schema name to validate against (use with --data; mutually exclusive with --schema-file)")
	validateDataCmd.Flags().StringVar(&validateSchemaFile, "schema-file", "", "Path to arbitrary schema file (JSON/YAML; overrides --schema)")
	validateDataCmd.Flags().StringSliceVar(&validateSchemaRefDirs, "ref-dir", []string{}, "Directory tree of schema files used to resolve absolute $ref URLs offline (repeatable). Safe if it also contains --schema-file")
	validateDataCmd.Flags().StringVar(&validateDataSchemaResolution, "schema-resolution", "prefer-id", "Schema resolution strategy for schema IDs (prefer-id, id-strict, path-only)")
	validateDataCmd.Flags().StringVar(&validateDataFile, "data", "", "Data file to validate (required)")
	if err := validateDataCmd.MarkFlagRequired("data"); err != nil {
		panic(fmt.Sprintf("failed to mark data flag as required: %v", err))
	}
	validateDataCmd.Flags().StringVar(&validateFormat, "format", "markdown", "Output format (markdown, json)")
}

// runValidate is the main validate runner (existing)
func runValidate(cmd *cobra.Command, args []string) error {
	target := "."
	if len(args) > 0 {
		target = args[0]
	}
	if _, err := os.Stat(target); os.IsNotExist(err) {
		return fmt.Errorf("target does not exist: %s", target)
	}

	// Handle --list-schemas flag
	if validateListSchemas {
		listSchemas(cmd)
		return nil
	}

	var outFmt assess.OutputFormat
	switch strings.ToLower(validateFormat) {
	case "markdown":
		outFmt = assess.FormatMarkdown
	case "json":
		outFmt = assess.FormatJSON
	case "html":
		outFmt = assess.FormatHTML
	case "both":
		outFmt = assess.FormatBoth
	case "concise":
		outFmt = assess.FormatConcise
	default:
		return fmt.Errorf("invalid format: %s", validateFormat)
	}

	var failOn assess.IssueSeverity
	switch validateFailOn {
	case "critical":
		failOn = assess.SeverityCritical
	case "high":
		failOn = assess.SeverityHigh
	case "medium":
		failOn = assess.SeverityMedium
	case "low":
		failOn = assess.SeverityLow
	case "info":
		failOn = assess.SeverityInfo
	default:
		return fmt.Errorf("invalid fail-on severity: %s", validateFailOn)
	}

	assessCfg := assess.AssessmentConfig{
		Mode:               assess.AssessmentModeCheck,
		Verbose:            validateVerbose,
		Timeout:            validateTimeout,
		IncludeFiles:       []string{},
		ExcludeFiles:       validateExcludeFiles,
		NoIgnore:           validateNoIgnore,
		ForceInclude:       validateForceInclude,
		SchemaEnableMeta:   validateEnableMeta,
		Scope:              validateScope,
		FailOnSeverity:     failOn,
		SelectedCategories: []string{string(assess.CategorySchema)},
	}

	// Load project config (preview) and compute include list
	if projCfg, err := config.LoadProjectConfig(); err == nil && projCfg != nil {
		sc := projCfg.GetSchemaConfig()
		var includes []string
		if len(validateIncludeFiles) > 0 {
			includes = append(includes, validateIncludeFiles...)
		} else if len(sc.Patterns) > 0 {
			for _, pat := range sc.Patterns {
				if strings.ContainsAny(pat, "*?[") {
					if matches, _ := filepath.Glob(pat); len(matches) > 0 {
						includes = append(includes, matches...)
					}
				} else {
					includes = append(includes, pat)
				}
			}
		} else if sc.AutoDetect {
			includes = append(includes, "schemas/")
		}
		assessCfg.IncludeFiles = includes
	} else {
		assessCfg.IncludeFiles = append(assessCfg.IncludeFiles, validateIncludeFiles...)
		if len(assessCfg.IncludeFiles) == 0 && validateAutoDetect {
			assessCfg.IncludeFiles = append(assessCfg.IncludeFiles, "schemas/")
		}
	}

	if len(assessCfg.IncludeFiles) == 0 {
		logger.Info("No schema include paths set. Use --include or enable schema.auto_detect (see docs/configuration/schema-config.md)")
	}

	engine := assess.NewAssessmentEngine()
	logger.Info(fmt.Sprintf("Validating schema files in %s", target))
	report, err := engine.RunAssessment(cmd.Context(), target, assessCfg)
	if err != nil {
		return fmt.Errorf("validation failed: %v", err)
	}

	formatter := assess.NewFormatter(outFmt)
	formatter.SetTargetPath(target)
	if validateOutput != "" {
		f, err := os.OpenFile(filepath.Clean(validateOutput), os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
		if err != nil {
			return fmt.Errorf("failed to create output: %v", err)
		}
		defer func() {
			if closeErr := f.Close(); closeErr != nil {
				logger.Warn(fmt.Sprintf("failed to close output file: %v", closeErr))
			}
		}()
		if err := formatter.WriteReport(f, report); err != nil {
			return fmt.Errorf("write report: %v", err)
		}
		logger.Info(fmt.Sprintf("Validation report written to %s", validateOutput))
	} else {
		if err := formatter.WriteReport(cmd.OutOrStdout(), report); err != nil {
			return fmt.Errorf("write report: %v", err)
		}
		logger.Info("Validation report written to stdout (use --output to save to file)")
	}

	if validateShouldFail(report, failOn) {
		validateSummarize(report)
		return fmt.Errorf("validation failed: found issues at or above %s", failOn)
	}
	return nil
}

// runValidateData runs data validation subcommand
func runValidateData(cmd *cobra.Command, args []string) error {
	return runDataValidation(cmd, dataValidationOptions{
		schemaName:       validateDataSchema,
		schemaFile:       validateSchemaFile,
		refDirs:          validateSchemaRefDirs,
		schemaResolution: validateDataSchemaResolution,
		dataFile:         validateDataFile,
		format:           validateFormat,
	})
}

// validateShouldFail determines if the report should cause failure
func validateShouldFail(report *assess.AssessmentReport, failOn assess.IssueSeverity) bool {
	total := 0
	for _, category := range report.Categories {
		total += len(category.Issues)
		for _, issue := range category.Issues {
			if issue.Severity >= failOn {
				return true
			}
		}
	}
	return false
}

// validateSummarize prints summary
func validateSummarize(report *assess.AssessmentReport) {
	total := 0
	for _, category := range report.Categories {
		total += len(category.Issues)
	}
	if total == 0 {
		return
	}
	logger.Info(fmt.Sprintf("Validation found %d issue(s)", total))
	// Simple summary
}

// listSchemas lists available embedded schemas.
func listSchemas(cmd *cobra.Command) {
	cmd.Println("Available embedded schemas (Draft-07 and Draft-2020-12 supported only):")
	infos := assets.GetSchemaNames()
	if len(infos) == 0 {
		cmd.Println("No schemas found.")
		return
	}
	for _, info := range infos {
		cmd.Printf("- %s (%s) - %s\n", info.Name, info.Path, info.Draft)
	}
	cmd.Println("\nNote: Use schema name without .yaml in 'validate data --schema <name>'. Heuristics match registry keys; drafts via $schema key.")
}

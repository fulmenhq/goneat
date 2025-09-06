package cmd

import (
    "fmt"
    "os"
    "path/filepath"
    "sort"
    "strings"
    "time"

	"github.com/3leaps/goneat/internal/assess"
	"github.com/3leaps/goneat/internal/ops"
    "github.com/3leaps/goneat/pkg/logger"
    cfgpkg "github.com/3leaps/goneat/pkg/config"
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
)

var validateCmd = &cobra.Command{
	Use:   "validate [target]",
	Short: "Schema-aware validation (preview)",
	Long:  "Validate JSON/YAML files with syntax checks and schema-aware processing (preview).",
	Args:  cobra.MaximumNArgs(1),
	RunE:  runValidate,
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
    // Ignore override flags
    validateCmd.Flags().BoolVar(&validateNoIgnore, "no-ignore", false, "Disable .goneatignore/.gitignore for discovery (scan everything in scope)")
    validateCmd.Flags().StringSliceVar(&validateForceInclude, "force-include", []string{}, "Force-include paths or globs even if ignored (repeatable). Examples: --force-include tests/fixtures/schemas/bad/** --force-include \"**/*.schema.yaml\"")
    // Schema validation options
    validateCmd.Flags().BoolVar(&validateEnableMeta, "enable-meta", false, "Attempt meta-schema validation using embedded drafts (may require network for remote $refs)")
    // Scoped discovery
    validateCmd.Flags().BoolVar(&validateScope, "scope", false, "Limit traversal scope to include paths and force-include anchors")
}

func runValidate(cmd *cobra.Command, args []string) error {
	target := "."
	if len(args) > 0 {
		target = args[0]
	}
	if _, err := os.Stat(target); os.IsNotExist(err) {
		return fmt.Errorf("target does not exist: %s", target)
	}

	// Output format
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

	// Fail-on severity
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
    if projCfg, err := cfgpkg.LoadProjectConfig(); err == nil && projCfg != nil {
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
        // Fallback to flags
        assessCfg.IncludeFiles = append(assessCfg.IncludeFiles, validateIncludeFiles...)
        if len(assessCfg.IncludeFiles) == 0 && validateAutoDetect {
            assessCfg.IncludeFiles = append(assessCfg.IncludeFiles, "schemas/")
        }
    }

    // Friendly hint when no include paths set
    if len(assessCfg.IncludeFiles) == 0 {
        logger.Info("No schema include paths set. Use --include or enable schema.auto_detect (see docs/configuration/schema-config.md)")
    }

	// Engine and run
	engine := assess.NewAssessmentEngine()
	logger.Info(fmt.Sprintf("Validating schema files in %s", target))
    report, err := engine.RunAssessment(cmd.Context(), target, assessCfg)
	if err != nil {
		return fmt.Errorf("validation failed: %v", err)
	}

	// Output
	formatter := assess.NewFormatter(outFmt)
	formatter.SetTargetPath(target)
    if validateOutput != "" {
        f, err := os.Create(validateOutput)
        if err != nil {
            return fmt.Errorf("failed to create output: %v", err)
        }
        defer func() {
            if err := f.Close(); err != nil {
                logger.Warn("Failed to close output file", logger.Err(err))
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

	// Fail-on gate
    if shouldFail(report, failOn) {
        summarizeValidation(report)
        return fmt.Errorf("validation failed: found issues at or above %s", failOn)
    }
    return nil
}

// summarizeValidation prints a brief summary for DX
func summarizeValidation(report *assess.AssessmentReport) {
    total := report.Summary.TotalIssues
    if total == 0 { return }
    counts := map[string]int{}
    var first []assess.Issue
    for _, cr := range report.Categories {
        if cr.Category != assess.CategorySchema { continue }
        for _, is := range cr.Issues {
            counts[is.File]++
            if len(first) < 3 { first = append(first, is) }
        }
    }
    logger.Info(fmt.Sprintf("Schema validation found %d issue(s)", total))
    // sort top files by count desc
    type kv struct{ f string; c int }
    var arr []kv
    for f, c := range counts { arr = append(arr, kv{f, c}) }
    sort.Slice(arr, func(i, j int) bool { if arr[i].c == arr[j].c { return arr[i].f < arr[j].f }; return arr[i].c > arr[j].c })
    for i := 0; i < len(arr) && i < 3; i++ {
        logger.Info(fmt.Sprintf("  - %s: %d", arr[i].f, arr[i].c))
    }
    for _, is := range first {
        logger.Info(fmt.Sprintf("    %s: %s", is.SubCategory, is.Message))
    }
}

/*
Copyright © 2025 3 Leaps <info@3leaps.com>
*/
package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"go/format"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"time"

	"github.com/fulmenhq/goneat/internal/doctor"
	"github.com/fulmenhq/goneat/internal/ops"
	"github.com/fulmenhq/goneat/pkg/config"
	formatpkg "github.com/fulmenhq/goneat/pkg/format"
	"github.com/fulmenhq/goneat/pkg/format/finalizer"
	"github.com/fulmenhq/goneat/pkg/logger"
	"github.com/fulmenhq/goneat/pkg/safeio"
	"github.com/fulmenhq/goneat/pkg/tools"
	"github.com/fulmenhq/goneat/pkg/work"
	"github.com/spf13/cobra"
)

// formatCmd represents the format command
var formatCmd = &cobra.Command{
	Use:   "format",
	Short: "Format code files",
	Long: `Format code files in the current directory or specified files.

Supports formatting Go, YAML, JSON, and other formats using appropriate tools.
By default, formats all supported files in the current directory.`,
	RunE: RunFormat,
}

func init() {
	rootCmd.AddCommand(formatCmd)

	// Register command in ops registry with taxonomy
	capabilities := ops.GetDefaultCapabilities(ops.GroupNeat, ops.CategoryFormatting)
	if err := ops.RegisterCommandWithTaxonomy("format", ops.GroupNeat, ops.CategoryFormatting, capabilities, formatCmd, "Format code files"); err != nil {
		panic(fmt.Sprintf("Failed to register format command: %v", err))
	}

	// File selection flags
	formatCmd.Flags().StringSlice("files", []string{}, "Specific files to format (explicit list, no patterns)")
	formatCmd.Flags().StringSlice("patterns", []string{}, "Glob patterns to filter files during discovery (e.g., '*.go', 'test_*.py')")
	formatCmd.Flags().Bool("include-config-dirs", false, "Include common configuration directories (.claude, .vscode, .idea, etc.)")
	formatCmd.Flags().StringSlice("folders", []string{}, "Folders to process (alternative to positional args)")

	// Operation mode flags
	formatCmd.Flags().Bool("check", false, "Check if files are formatted without modifying")
	formatCmd.Flags().Bool("quiet", false, "Suppress output except for errors")
	formatCmd.Flags().BoolP("verbose", "v", false, "Show detailed information including skipped files and processing details")
	formatCmd.Flags().Bool("dry-run", false, "Show what would be done without executing")
	formatCmd.Flags().Bool("plan-only", false, "Generate and display work plan without executing")
	formatCmd.Flags().String("plan-file", "", "Write work plan to specified file")

	// Discovery and filtering flags
	formatCmd.Flags().StringSlice("types", []string{}, "Content types to include ("+formatpkg.SupportedContentTypesHelp()+")")
	formatCmd.Flags().Int("max-depth", -1, "Maximum directory depth to traverse")

	// Execution strategy flags
	formatCmd.Flags().String("strategy", "parallel", "Execution strategy (parallel, sequential)")
	formatCmd.Flags().Bool("fallback-sequential", false, "If parallel strategy fails, retry sequentially")
	formatCmd.Flags().Int("workers", 0, "Number of parallel workers (0 = auto)")
	formatCmd.Flags().Bool("group-by-size", false, "Group work items by file size")
	formatCmd.Flags().Bool("group-by-type", false, "Group work items by content type")

	// Dogfooding helpers
	formatCmd.Flags().Bool("staged-only", false, "Only format staged files in git (changed and added)")
	formatCmd.Flags().Bool("ignore-missing-tools", false, "Use finalizer-only processing when a selected external formatter is missing")

	// EOF/Text normalization flags
	formatCmd.Flags().Bool("finalize-eof", true, "Ensure files end with exactly one newline")
	formatCmd.Flags().Bool("finalize-trim-trailing-spaces", true, "Remove trailing whitespace from all lines")
	formatCmd.Flags().String("finalize-line-endings", "", "Normalize line endings (lf, crlf, or auto)")
	formatCmd.Flags().Bool("finalize-remove-bom", false, "Remove Byte Order Mark (UTF-8, UTF-16, UTF-32)")
	formatCmd.Flags().Bool("text-normalize", true, "Apply generic text normalization to any text file (unknown extensions included)")
	formatCmd.Flags().String("text-encoding-policy", "utf8-only", "Encoding policy for text normalization: utf8-only|utf8-or-bom|any-text")
	formatCmd.Flags().Bool("preserve-md-linebreaks", true, "Preserve Markdown hard line breaks (two trailing spaces)")

	// Import alignment (Go) - opt-in
	formatCmd.Flags().Bool("use-goimports", false, "Organize Go imports with goimports (after gofmt)")

	// JSON prettification options
	formatCmd.Flags().String("json-indent", "  ", "Indentation for JSON prettification (e.g., '  ' or '\t')")
	formatCmd.Flags().Int("json-indent-count", 2, "Number of spaces for JSON indentation (1-10, 0 to skip prettification)")
	formatCmd.Flags().Int("json-size-warning", 500, "Size threshold in MB for JSON file warnings (0 to disable)")

	// XML prettification options
	formatCmd.Flags().String("xml-indent", "  ", "Indentation for XML prettification (e.g., '  ' or '\t')")
	formatCmd.Flags().Int("xml-indent-count", 2, "Number of spaces for XML indentation (1-10, 0 to skip prettification)")
	formatCmd.Flags().Int("xml-size-warning", 500, "Size threshold in MB for XML file warnings (0 to disable)")
}

// findToolPath finds a tool by name, checking PATH first then known shim directories.
// This handles tools installed via brew, bun, go-install, etc. that may not be in PATH
// (e.g., in CI environments where PATH wasn't updated after bootstrap).
//
// Returns the full path to the tool or empty string if not found.
func findToolPath(toolName string) string {
	// First check PATH (fast path for normal case)
	if path, err := exec.LookPath(toolName); err == nil {
		return path
	}

	// Check brew bin directory (covers tools installed via brew)
	if _, brewPath, err := tools.DetectBrew(); err == nil && brewPath != "" {
		binDir := filepath.Dir(brewPath)
		candidate := filepath.Join(binDir, toolName)
		if _, err := os.Stat(candidate); err == nil {
			logger.Debug(fmt.Sprintf("found %s in brew bin: %s", toolName, candidate))
			return candidate
		}
	}

	// Check other known shim paths
	shimDirs := []string{
		doctor.GetShimPath("bun"),
		doctor.GetShimPath("go-install"),
		doctor.GetShimPath("mise"),
		doctor.GetShimPath("scoop"),
	}

	for _, shimDir := range shimDirs {
		if shimDir == "" {
			continue
		}
		candidate := filepath.Join(shimDir, toolName)
		if runtime.GOOS == "windows" && !strings.HasSuffix(candidate, ".exe") {
			candidate += ".exe"
		}
		if _, err := os.Stat(candidate); err == nil {
			logger.Debug(fmt.Sprintf("found %s in shim dir: %s", toolName, candidate))
			return candidate
		}
	}

	return ""
}

var formatToolResolver = findToolPath

type formatToolPaths struct {
	work.FormatProcessorToolPaths
	Goimports string
}

type formatToolRequirement struct {
	name        string
	contentType string
	guidance    string
}

func preflightFormatTools(files []string, useGoimports, ignoreMissingTools bool) (formatToolPaths, error) {
	selectedTypes := make(map[string]bool)
	for _, file := range files {
		selectedTypes[getContentTypeFromPath(file)] = true
	}

	requirements := []formatToolRequirement{
		{name: "goimports", contentType: "go", guidance: "Install with: goneat doctor tools --scope go --install"},
		{name: "yamlfmt", contentType: "yaml", guidance: "Install with: goneat doctor tools --scope foundation --install"},
		{name: "prettier", contentType: "markdown", guidance: "Install with: goneat doctor tools --scope foundation --install"},
		{name: "ruff", contentType: "python", guidance: "Install with: goneat doctor tools --scope python --install"},
		{name: "biome", contentType: "javascript/typescript", guidance: "Install with: goneat doctor tools --scope typescript --install"},
	}

	var paths formatToolPaths
	var unavailable []error
	for _, requirement := range requirements {
		required := selectedTypes[requirement.contentType]
		switch requirement.name {
		case "goimports":
			required = useGoimports && selectedTypes["go"]
		case "biome":
			required = selectedTypes["javascript"] || selectedTypes["typescript"]
		}
		if !required {
			continue
		}

		path := formatToolResolver(requirement.name)
		switch requirement.name {
		case "goimports":
			paths.Goimports = path
		case "yamlfmt":
			paths.Yamlfmt = path
		case "prettier":
			paths.Prettier = path
		case "ruff":
			paths.Ruff = path
		case "biome":
			paths.Biome = path
		}
		if path != "" {
			continue
		}

		err := formatpkg.ToolUnavailable("", requirement.name, requirement.guidance)
		if ignoreMissingTools {
			logger.Warn(
				fmt.Sprintf("%s unavailable for %s; using finalizer-only processing", requirement.name, requirement.contentType),
				logger.String("result_class", string(formatpkg.ResultToolUnavailable)),
				logger.String("tool", requirement.name),
				logger.String("content_type", requirement.contentType),
				logger.String("policy", "finalizer-only"),
			)
			continue
		}
		unavailable = append(unavailable, err)
	}

	return paths, errors.Join(unavailable...)
}

func RunFormat(cmd *cobra.Command, args []string) (runErr error) {
	defer func() {
		if formatpkg.ClassOf(runErr) == "" {
			return
		}
		root := cmd.Root()
		root.SilenceErrors = true
		root.SilenceUsage = true
	}()

	logger.Info("Starting format command")

	// Load configuration
	cfg, err := config.LoadProjectConfig()
	if err != nil {
		// Check if this is a validation error (config exists but is invalid)
		if strings.Contains(err.Error(), "validation failed") {
			return fmt.Errorf("invalid project configuration: %w", err)
		}
		// Config loading failed, use defaults (this is normal if no config file exists)
		cfg = &config.Config{}
	}

	// Get all flags
	explicitFiles, _ := cmd.Flags().GetStringSlice("files")
	patterns, _ := cmd.Flags().GetStringSlice("patterns")
	folders, _ := cmd.Flags().GetStringSlice("folders")
	includeConfigDirs, _ := cmd.Flags().GetBool("include-config-dirs")
	checkOnly, _ := cmd.Flags().GetBool("check")
	quiet, _ := cmd.Flags().GetBool("quiet")
	verbose, _ := cmd.Flags().GetBool("verbose")
	dryRun, _ := cmd.Flags().GetBool("dry-run")
	planOnly, _ := cmd.Flags().GetBool("plan-only")
	planFile, _ := cmd.Flags().GetString("plan-file")
	contentTypes, _ := cmd.Flags().GetStringSlice("types")
	maxDepth, _ := cmd.Flags().GetInt("max-depth")
	strategy, _ := cmd.Flags().GetString("strategy")
	groupBySize, _ := cmd.Flags().GetBool("group-by-size")
	groupByType, _ := cmd.Flags().GetBool("group-by-type")
	noOp, _ := cmd.Flags().GetBool("no-op")
	stagedOnly, _ := cmd.Flags().GetBool("staged-only")
	fallbackSequential, _ := cmd.Flags().GetBool("fallback-sequential")
	workers, _ := cmd.Flags().GetInt("workers")
	ignoreMissingTools, _ := cmd.Flags().GetBool("ignore-missing-tools")
	finalizeEOF, _ := cmd.Flags().GetBool("finalize-eof")
	finalizeTrimTrailingSpaces, _ := cmd.Flags().GetBool("finalize-trim-trailing-spaces")
	finalizeLineEndings, _ := cmd.Flags().GetString("finalize-line-endings")
	finalizeRemoveBOM, _ := cmd.Flags().GetBool("finalize-remove-bom")
	textNormalize, _ := cmd.Flags().GetBool("text-normalize")
	textEncodingPolicy, _ := cmd.Flags().GetString("text-encoding-policy")
	preserveMd, _ := cmd.Flags().GetBool("preserve-md-linebreaks")
	useGoimports, _ := cmd.Flags().GetBool("use-goimports")
	jsonIndent, _ := cmd.Flags().GetString("json-indent")
	jsonIndentCount, _ := cmd.Flags().GetInt("json-indent-count")
	jsonSizeWarningMB, _ := cmd.Flags().GetInt("json-size-warning")
	xmlIndent, _ := cmd.Flags().GetString("xml-indent")
	xmlIndentCount, _ := cmd.Flags().GetInt("xml-indent-count")
	xmlSizeWarningMB, _ := cmd.Flags().GetInt("xml-size-warning")

	// Validate flag combinations per Arch Eagle's precedence rules
	if len(explicitFiles) > 0 {
		if len(patterns) > 0 {
			return fmt.Errorf("cannot use --files with --patterns: --files processes exact files, --patterns filters discovered files")
		}
		if len(folders) > 0 {
			return fmt.Errorf("cannot use --files with --folders: --files processes exact files, --folders specifies discovery scope")
		}
		if len(args) > 0 {
			// Check if any positional args are files (not directories)
			for _, arg := range args {
				if info, err := os.Stat(arg); err == nil && !info.IsDir() {
					return fmt.Errorf("cannot use --files with positional file arguments: use --files for explicit files OR positional args for files/folders")
				}
			}
		}
	}

	// Check if no-op mode is enabled
	if noOp {
		logger.Info("Running in no-op mode - tasks will be executed but no files will be modified")
	}

	var filesToProcess []string
	var usePlanner = true
	var plannerConfig work.PlannerConfig

	// Handle explicit files mode (highest priority)
	if len(explicitFiles) > 0 {
		filesToProcess = explicitFiles
		usePlanner = false

		// Validate that all explicit files exist
		for _, file := range explicitFiles {
			if _, err := os.Stat(file); os.IsNotExist(err) {
				return fmt.Errorf("file not found: %s", file)
			}
		}
	} else {
		// Determine paths for discovery-based processing
		var paths []string
		if len(folders) > 0 {
			paths = folders
		} else if len(args) > 0 {
			paths = args
		} else {
			paths = []string{"."}
		}

		forceInclude := append([]string(nil), explicitFiles...)
		// Also treat positional args as explicit targets (avoid ignore surprises for .goneat/, etc.)
		if len(folders) == 0 {
			forceInclude = append(forceInclude, args...)
		}

		// Create planner configuration
		supportedTypes := formatpkg.SupportedContentTypes()
		if textNormalize {
			supportedTypes = append(supportedTypes, "unknown")
		}

		plannerConfig = work.PlannerConfig{
			Command:              "format",
			Paths:                paths,
			IncludePatterns:      patterns, // Use new patterns flag
			ContentTypes:         contentTypes,
			MaxDepth:             maxDepth,
			ExecutionStrategy:    strategy,
			GroupBySize:          groupBySize,
			GroupByContentType:   groupByType,
			IgnoreFile:           ".goneatignore",                           // Enable .goneatignore support
			EnableFinalizer:      finalizeEOF || finalizeTrimTrailingSpaces, // Enable finalizer support
			Verbose:              verbose,                                   // Enable verbose logging
			IncludeConfigDirs:    includeConfigDirs,                         // Include config directories like .claude
			ForceIncludePatterns: forceInclude,
			// For parallel execution, filter to only content types the FormatProcessor supports
			SupportedContentTypes: supportedTypes,
		}
	}

	if stagedOnly {
		if len(explicitFiles) > 0 {
			return fmt.Errorf("cannot use --staged-only with --files: staged mode discovers files from git, --files specifies exact files")
		}
		staged, err := formatStagedFiles()
		if err != nil {
			return fmt.Errorf("failed to get staged files: %v", err)
		}
		// Filter by content types and patterns if provided
		allowed := make(map[string]bool)
		for _, t := range contentTypes {
			allowed[strings.ToLower(strings.TrimSpace(t))] = true
		}
		for _, f := range staged {
			// Apply pattern filters if provided
			if len(patterns) > 0 {
				matched := false
				for _, pattern := range patterns {
					if matched, _ = filepath.Match(pattern, filepath.Base(f)); matched {
						break
					}
				}
				if !matched {
					continue
				}
			}
			ct := getContentTypeFromPath(f)
			if len(allowed) == 0 || allowed[ct] {
				filesToProcess = append(filesToProcess, f)
			}
		}
		usePlanner = false

		// In staged-only + (planOnly or dryRun), synthesize a small manifest for plan output
		if planOnly || dryRun {
			synth := &work.WorkManifest{Plan: work.Plan{Command: "format", Timestamp: time.Now(), WorkingDirectory: ".", TotalFiles: len(filesToProcess), FilteredFiles: len(filesToProcess), ExecutionStrategy: strategy}}
			return handlePlanOnly(cmd, synth, planFile, dryRun)
		}
	} else if usePlanner {
		// Planner path for discovery-based runs
		planner := work.NewPlanner(plannerConfig)
		manifest, err := planner.GenerateManifest()
		if err != nil {
			logger.Error("Failed to generate work manifest", logger.Err(err))
			return err
		}
		// Handle plan-only mode
		if planOnly || dryRun {
			return handlePlanOnly(cmd, manifest, planFile, dryRun)
		}
		// Extract files from manifest
		filesToProcess = make([]string, len(manifest.WorkItems))
		for i, item := range manifest.WorkItems {
			filesToProcess[i] = item.Path
		}
	}

	// Handle plan-only mode for explicit files
	if (planOnly || dryRun) && !usePlanner {
		synth := &work.WorkManifest{Plan: work.Plan{Command: "format", Timestamp: time.Now(), WorkingDirectory: ".", TotalFiles: len(filesToProcess), FilteredFiles: len(filesToProcess), ExecutionStrategy: strategy}}
		return handlePlanOnly(cmd, synth, planFile, dryRun)
	}

	if len(filesToProcess) == 0 {
		if !quiet {
			logger.Info("No supported files found to format")
		}
		return nil
	}

	requireGoimports := useGoimports && (strategy != "parallel" || fallbackSequential || stagedOnly)
	toolPaths, err := preflightFormatTools(filesToProcess, requireGoimports, ignoreMissingTools)
	if err != nil {
		return err
	}

	if !quiet {
		logger.Info(fmt.Sprintf("Processing %d files using %s strategy", len(filesToProcess), strategy))
	}

	// Create normalization options
	options := finalizer.NormalizationOptions{
		EnsureEOF:                  finalizeEOF,
		TrimTrailingWhitespace:     finalizeTrimTrailingSpaces,
		NormalizeLineEndings:       finalizeLineEndings,
		RemoveUTF8BOM:              finalizeRemoveBOM,
		PreserveMarkdownHardBreaks: preserveMd,
		EncodingPolicy:             textEncodingPolicy,
	}

	// Execute based on strategy
	if strategy == "parallel" && !dryRun && !planOnly && !stagedOnly {
		if useGoimports {
			logger.Warn("use-goimports is enabled but parallel processor does not apply goimports yet; skipping import alignment in parallel mode")
		}
		err := executeParallel(filesToProcess, cfg, quiet, checkOnly, noOp, ignoreMissingTools, options, textNormalize, jsonIndent, jsonIndentCount, jsonSizeWarningMB, workers, toolPaths)
		if err == nil || !fallbackSequential {
			return err
		}
		logger.Warn(fmt.Sprintf("Parallel strategy failed (%v); retrying sequentially", err))
		return executeSequentialWithOptions(filesToProcess, checkOnly || noOp, quiet, cfg, ignoreMissingTools, options, useGoimports, textNormalize, jsonIndent, jsonIndentCount, jsonSizeWarningMB, xmlIndent, xmlIndentCount, xmlSizeWarningMB, toolPaths)
	}

	return executeSequentialWithOptions(filesToProcess, checkOnly || noOp, quiet, cfg, ignoreMissingTools, options, useGoimports, textNormalize, jsonIndent, jsonIndentCount, jsonSizeWarningMB, xmlIndent, xmlIndentCount, xmlSizeWarningMB, toolPaths)
}

// removed unused findSupportedFiles helper

func processFile(file string, checkOnly bool, _ bool, cfg *config.Config, ignoreMissingTools bool, options finalizer.NormalizationOptions, useGoimports bool, textNormalize bool, jsonIndent string, jsonIndentCount int, jsonSizeWarningMB int, xmlIndent string, xmlIndentCount int, xmlSizeWarningMB int, toolPaths formatToolPaths) error {
	ext := filepath.Ext(file)
	contentType := formatpkg.ContentTypeForExtension(ext)

	var err error

	switch contentType {
	case "go":
		err = formatGoFile(file, checkOnly, cfg, useGoimports, ignoreMissingTools, options, toolPaths.Goimports)

	case "yaml":
		if toolPaths.Yamlfmt == "" {
			err = formatpkg.ErrAlreadyFormatted
			break
		}
		err = formatYAMLFile(file, checkOnly, cfg, options, toolPaths.Yamlfmt)

	case "json":
		err = formatJSONFile(file, checkOnly, cfg, options, jsonIndent, jsonIndentCount, jsonSizeWarningMB)

	case "xml":
		err = formatXMLFile(file, checkOnly, cfg, options, xmlIndent, xmlIndentCount, xmlSizeWarningMB)

	case "markdown":
		if toolPaths.Prettier == "" {
			err = formatpkg.ErrAlreadyFormatted
			break
		}
		err = formatMarkdownFile(file, checkOnly, cfg, options, toolPaths.Prettier)

	case "python":
		if toolPaths.Ruff == "" {
			err = formatpkg.ErrAlreadyFormatted
			break
		}
		err = formatPythonFile(file, checkOnly, cfg, options, ignoreMissingTools, toolPaths.Ruff)

	case "javascript", "typescript":
		if toolPaths.Biome == "" {
			err = formatpkg.ErrAlreadyFormatted
			break
		}
		err = formatJavaScriptFile(file, checkOnly, cfg, options, ignoreMissingTools, toolPaths.Biome)

	default:
		// Check if file is XML by content (starts with <?xml)
		if isXMLFile(file) {
			err = formatXMLFile(file, checkOnly, cfg, options, xmlIndent, xmlIndentCount, xmlSizeWarningMB)
		} else {
			// Non-primary types: apply finalizer to supported extensions or any text file when textNormalize is enabled
			if options.EnsureEOF || options.TrimTrailingWhitespace || options.NormalizeLineEndings != "" || options.RemoveUTF8BOM || textNormalize {
				if finalizer.IsSupportedExtension(ext) {
					return applyFinalizer(file, checkOnly, options)
				}
				if textNormalize {
					return applyFinalizer(file, checkOnly, options)
				}
			}
			supportedExts := []string{".go", ".yaml", ".yml", ".json", ".xml", ".md", ".markdown", ".py", ".pyi", ".js", ".jsx", ".ts", ".tsx"}
			return fmt.Errorf("unsupported file type '%s' for file %s. Supported extensions: %v. Use --types flag to filter specific content types", ext, file, supportedExts)
		}
	}

	// Apply finalizer after primary formatter (when enabled and extension supported)
	// Always apply finalizer for supported extensions when finalizer options are enabled,
	// regardless of whether the primary formatter made changes
	if options.EnsureEOF || options.TrimTrailingWhitespace || options.NormalizeLineEndings != "" || options.RemoveUTF8BOM {
		if finalizer.IsSupportedExtension(ext) {
			if ferr := applyFinalizer(file, checkOnly, options); ferr != nil {
				// If finalizer made changes, it takes precedence over primary formatter
				if errors.Is(ferr, formatpkg.ErrFinalized) || errors.Is(ferr, formatpkg.ErrFormatDrift) {
					// Finalizer found issues - return needs formatting
					return formatpkg.FormatDrift(file)
				}
				// If primary formatter said "needs formatting", that takes precedence
				// over finalizer saying "already formatted"
				if errors.Is(err, formatpkg.ErrFormatDrift) {
					return err
				}
				// If primary formatter said "already formatted" but finalizer found changes,
				// the finalizer result takes precedence
				if errors.Is(err, formatpkg.ErrAlreadyFormatted) {
					return ferr
				}
				// If primary formatter had other errors, return the primary error
				if err != nil {
					return err
				}
				// If primary formatter succeeded but finalizer found issues, return finalizer error
				return ferr
			}
			// Finalizer says file is OK ("already formatted" in check mode, nil in format mode)
			// If primary formatter said "needs formatting", that takes precedence
			if errors.Is(err, formatpkg.ErrFormatDrift) {
				return err
			}
		}
	}

	return err
}

// applyFinalizer applies comprehensive file normalization
func applyFinalizer(file string, checkOnly bool, options finalizer.NormalizationOptions) error {
	// Validate file path to prevent path traversal
	file = filepath.Clean(file)
	if strings.Contains(file, "..") {
		return fmt.Errorf("invalid file path: contains path traversal")
	}

	// Read the file content
	content, err := os.ReadFile(file)
	if err != nil {
		return formatpkg.FileIO(file, err)
	}

	if checkOnly {
		// Use the same comprehensive normalization function for consistent checking
		if _, changed, err := finalizer.ComprehensiveFileNormalization(content, options); err != nil {
			return fmt.Errorf("normalization error: %v", err)
		} else if changed {
			logger.Info(fmt.Sprintf("File %s needs formatting (EOF, trailing whitespace, line endings, or BOM issues)", file))
			return formatpkg.FormatDrift(file)
		}
		return formatpkg.ErrAlreadyFormatted
	}

	// Apply comprehensive normalization
	finalized, changed, err := finalizer.ComprehensiveFileNormalization(content, options)
	if err != nil {
		return fmt.Errorf("finalizer error: %v", err)
	}

	if changed {
		// Write back the finalized content
		if err := safeio.WriteFileValidated(file, finalized, 0o600); err != nil {
			return formatpkg.FileIO(file, fmt.Errorf("failed to write finalized file: %v", err))
		}
		// Return an error to indicate changes were made (finalizer takes precedence)
		return formatpkg.ErrFinalized
	}

	// No changes needed
	return nil
}

func formatGoFile(file string, checkOnly bool, cfg *config.Config, useGoimports bool, ignoreMissingTools bool, options finalizer.NormalizationOptions, goimportsPath string) error {
	// Validate file path to prevent path traversal
	file = filepath.Clean(file)
	if strings.Contains(file, "..") {
		return fmt.Errorf("invalid file path: contains path traversal")
	}

	original, err := os.ReadFile(file)
	if err != nil {
		return formatpkg.FileIO(file, err)
	}

	// Get Go formatting configuration
	goConfig := cfg.GetGoConfig()

	// Step 1: gofmt (go/format)
	if goConfig.Simplify {
		// Note: go/format doesn't directly support simplify; placeholder for future AST-based simplify
		logger.Debug(fmt.Sprintf("Go simplify option enabled for %s", file))
	}
	gofmtOut, err := format.Source(original)
	if err != nil {
		return formatpkg.ToolExecution(file, "gofmt", fmt.Errorf("gofmt failed for %s: %w", file, err))
	}

	final := gofmtOut
	// Step 2: goimports (optional)
	if useGoimports {
		if goimportsPath == "" {
			if !ignoreMissingTools {
				return formatpkg.ToolUnavailable(file, "goimports", "Install with: goneat doctor tools --scope go --install")
			}
			logger.Debug("goimports not found; skipping import alignment due to --ignore-missing-tools")
		} else {
			// #nosec G204 - goimportsPath comes from findToolPath which validates paths
			cmd := exec.Command(goimportsPath)
			cmd.Dir = filepath.Dir(file)
			cmd.Stdin = strings.NewReader(string(final))
			out, cmdErr := cmd.Output()
			if cmdErr != nil {
				return formatpkg.ToolExecution(file, "goimports", fmt.Errorf("goimports failed for %s: %v", file, cmdErr))
			}
			final = out
		}
	}

	changed := !bytes.Equal(original, final)

	// Check if finalizer would make additional changes
	finalizerWillChange := false
	if options.EnsureEOF || options.TrimTrailingWhitespace || options.NormalizeLineEndings != "" || options.RemoveUTF8BOM {
		if _, fChanged, err := finalizer.ComprehensiveFileNormalization(final, options); err == nil && fChanged {
			finalizerWillChange = true
		}
	}

	if checkOnly {
		if changed || finalizerWillChange {
			return formatpkg.FormatDrift(file)
		}
		return formatpkg.ErrAlreadyFormatted
	}

	if changed || finalizerWillChange {
		// Apply finalizer if needed
		if finalizerWillChange {
			finalized, _, err := finalizer.ComprehensiveFileNormalization(final, options)
			if err != nil {
				return fmt.Errorf("finalizer error: %v", err)
			}
			final = finalized
		}

		logger.Info(fmt.Sprintf("Applying Go formatting changes to %s", file))
		if err := safeio.WriteFileValidated(file, final, 0o600); err != nil {
			return formatpkg.FileIO(file, err)
		}
		return nil
	}

	return formatpkg.ErrAlreadyFormatted
}

func formatYAMLFile(file string, checkOnly bool, cfg *config.Config, options finalizer.NormalizationOptions, yamlfmtPath string) error {
	// Clean file path to prevent path traversal
	file = filepath.Clean(file)

	if yamlfmtPath == "" {
		return formatpkg.ToolUnavailable(file, "yamlfmt", "Install with: goneat doctor tools --scope foundation --install")
	}

	yamlConfig := cfg.GetYAMLConfig()

	// For proper change detection, read original content first
	originalContent, err := os.ReadFile(file)
	if err != nil {
		return formatpkg.FileIO(file, err)
	}

	// Build yamlfmt arguments via the shared helper so this sequential path,
	// pkg/work/format_processor.go (parallel/assess), and any future caller
	// pass identical formatter options. Limensafe v0.5.12 surfaced a drift
	// here: this path previously omitted pad_line_comments, so `goneat
	// format` was a no-op on YAML that `yamlfmt -lint` (and goneat assess
	// format) correctly flagged. See docs/appnotes/yaml-format-lint-alignment.md.
	args := yamlConfig.YamlfmtFormatterArgs()

	// Route both check and apply through the shared yamlfmt+finalizer helper so
	// they compare against the identical canonical output. The previous check
	// path used `yamlfmt -lint` (flags differences the finalizer reverts → false
	// "needs formatting") and the previous apply path compared `-dry` *text*
	// output against file *content* (always unequal → spurious "will change").
	runFinalizer := options.EnsureEOF || options.TrimTrailingWhitespace || options.NormalizeLineEndings != "" || options.RemoveUTF8BOM
	formatted, err := work.FormatYAMLContent(yamlfmtPath, args, originalContent, options, runFinalizer)
	if err != nil {
		return formatpkg.ToolExecution(file, "yamlfmt", err)
	}

	if bytes.Equal(originalContent, formatted) {
		return formatpkg.ErrAlreadyFormatted
	}

	if checkOnly {
		return formatpkg.FormatDrift(file)
	}

	// Preserve the file's existing mode. This full-file write now handles the
	// primary YAML rewrite (previously delegated to in-place `yamlfmt`, which
	// preserves mode); writing a fixed 0o600 here would otherwise tighten a
	// normal 0644 YAML file. (Per kilo-devrev review of v0.5.13.)
	if err := safeio.WriteFilePreservePerms(file, formatted); err != nil {
		return formatpkg.FileIO(file, fmt.Errorf("failed to write formatted file: %v", err))
	}

	logger.Info(fmt.Sprintf("Applying YAML formatting changes to %s", file))
	return nil
}

func formatJSONFile(file string, checkOnly bool, cfg *config.Config, options finalizer.NormalizationOptions, jsonIndent string, jsonIndentCount int, jsonSizeWarningMB int) error {
	// Validate file path to prevent path traversal
	file = filepath.Clean(file)
	if strings.Contains(file, "..") {
		return fmt.Errorf("invalid file path: contains path traversal")
	}

	jsonConfig := cfg.GetJSONConfig()

	// Validate flags
	if jsonIndent != "  " && jsonIndentCount != 2 {
		return fmt.Errorf("cannot specify both --json-indent and --json-indent-count")
	}
	if jsonIndentCount < 0 || jsonIndentCount > 10 {
		return fmt.Errorf("--json-indent-count must be between 0 and 10")
	}

	// Determine indent string
	var indent string
	if jsonIndentCount == 0 {
		// Skip prettification
		indent = ""
	} else if jsonIndentCount != 2 {
		// Use count to generate spaces
		indent = strings.Repeat(" ", jsonIndentCount)
	} else {
		// Use provided string or default
		indent = jsonIndent
	}

	// Read the original content
	originalContent, err := os.ReadFile(file)
	if err != nil {
		return formatpkg.FileIO(file, err)
	}

	// Use built-in JSON prettification
	var formatted string
	if jsonConfig.Compact {
		// For compact mode, use minimal indent (empty string for jq-like compact)
		output, changed, err := formatpkg.PrettifyJSON(originalContent, "", jsonSizeWarningMB)
		if err != nil {
			return fmt.Errorf("JSON prettification failed: %v", err)
		}
		if changed {
			formatted = string(output)
		} else {
			formatted = string(originalContent)
		}
	} else if indent == "" {
		// Skip prettification if indent-count is 0
		formatted = string(originalContent)
	} else {
		// For pretty printing, use specified indent
		output, changed, err := formatpkg.PrettifyJSON(originalContent, indent, jsonSizeWarningMB)
		if err != nil {
			return fmt.Errorf("JSON prettification failed: %v", err)
		}
		if changed {
			formatted = string(output)
		} else {
			formatted = string(originalContent)
		}
	}

	// Handle trailing newline
	if jsonConfig.TrailingNewline && !strings.HasSuffix(formatted, "\n") {
		formatted += "\n"
	} else if !jsonConfig.TrailingNewline && strings.HasSuffix(formatted, "\n") {
		formatted = strings.TrimSuffix(formatted, "\n")
	}

	// Apply finalizer options if requested (EOF, trailing whitespace, etc.)
	if options.EnsureEOF || options.TrimTrailingWhitespace || options.NormalizeLineEndings != "" || options.RemoveUTF8BOM {
		finalized, changed, err := finalizer.ComprehensiveFileNormalization([]byte(formatted), options)
		if err != nil {
			return fmt.Errorf("finalizer error: %v", err)
		}
		if changed {
			formatted = string(finalized)
		}
	}

	// Check if any formatting changed the content
	contentChanged := !bytes.Equal(originalContent, []byte(formatted))

	if !contentChanged {
		return formatpkg.ErrAlreadyFormatted
	}

	if checkOnly {
		return formatpkg.FormatDrift(file)
	}

	logger.Info(fmt.Sprintf("Applying JSON formatting changes to %s", file))
	if err := safeio.WriteFileValidated(file, []byte(formatted), 0o600); err != nil {
		return formatpkg.FileIO(file, err)
	}
	return nil
}

func formatXMLFile(file string, checkOnly bool, cfg *config.Config, options finalizer.NormalizationOptions, xmlIndent string, xmlIndentCount int, xmlSizeWarningMB int) error {
	// Validate file path to prevent path traversal
	file = filepath.Clean(file)
	if strings.Contains(file, "..") {
		return fmt.Errorf("invalid file path: contains path traversal")
	}

	// Validate flags
	if xmlIndent != "  " && xmlIndentCount != 2 {
		return fmt.Errorf("cannot specify both --xml-indent and --xml-indent-count")
	}
	if xmlIndentCount < 0 || xmlIndentCount > 10 {
		return fmt.Errorf("--xml-indent-count must be between 0 and 10")
	}

	// Determine indent string
	var indent string
	if xmlIndentCount == 0 {
		// Skip prettification
		indent = ""
	} else if xmlIndentCount != 2 {
		// Use count to generate spaces
		indent = strings.Repeat(" ", xmlIndentCount)
	} else {
		// Use provided string or default
		indent = xmlIndent
	}

	// Read the original content
	originalContent, err := os.ReadFile(file)
	if err != nil {
		return formatpkg.FileIO(file, err)
	}

	// Use built-in XML prettification
	var formatted string
	if indent == "" {
		// Skip prettification if indent-count is 0
		formatted = string(originalContent)
	} else {
		// For pretty printing, use specified indent
		output, changed, err := formatpkg.PrettifyXML(originalContent, indent, xmlSizeWarningMB)
		if err != nil {
			return fmt.Errorf("XML prettification failed: %v", err)
		}
		if changed {
			formatted = string(output)
		} else {
			formatted = string(originalContent)
		}
	}

	// Apply finalizer options if requested (EOF, trailing whitespace, etc.)
	if options.EnsureEOF || options.TrimTrailingWhitespace || options.NormalizeLineEndings != "" || options.RemoveUTF8BOM {
		finalized, changed, err := finalizer.ComprehensiveFileNormalization([]byte(formatted), options)
		if err != nil {
			return fmt.Errorf("finalizer error: %v", err)
		}
		if changed {
			formatted = string(finalized)
		}
	}

	// Check if any formatting changed the content
	contentChanged := !bytes.Equal(originalContent, []byte(formatted))

	if !contentChanged {
		return formatpkg.ErrAlreadyFormatted
	}

	if checkOnly {
		return formatpkg.FormatDrift(file)
	}

	logger.Info(fmt.Sprintf("Applying XML formatting changes to %s", file))
	if err := safeio.WriteFileValidated(file, []byte(formatted), 0o600); err != nil {
		return formatpkg.FileIO(file, err)
	}
	return nil
}

func formatMarkdownFile(file string, checkOnly bool, cfg *config.Config, options finalizer.NormalizationOptions, prettierPath string) error {
	// Validate file path to prevent path traversal
	file = filepath.Clean(file)
	if strings.Contains(file, "..") {
		return fmt.Errorf("invalid file path: contains path traversal")
	}

	if prettierPath == "" {
		return formatpkg.ToolUnavailable(file, "prettier", "Install with: goneat doctor tools --scope foundation --install")
	}

	mdConfig := cfg.GetMarkdownConfig()

	// Read the original content
	originalContent, err := os.ReadFile(file)
	if err != nil {
		return formatpkg.FileIO(file, err)
	}

	// Build prettier arguments
	args := []string{"--parser", "markdown"}

	// Add configuration options
	if mdConfig.LineLength > 0 {
		args = append(args, "--print-width", fmt.Sprintf("%d", mdConfig.LineLength))
	}

	// Handle reference style
	switch mdConfig.ReferenceStyle {
	case "collapsed":
		args = append(args, "--reference-style", "collapsed")
	case "full":
		args = append(args, "--reference-style", "full")
	case "shortcut":
		args = append(args, "--reference-style", "shortcut")
	}

	// Handle code block style (prettier doesn't have direct option, but we can note it)
	logger.Debug(fmt.Sprintf("Code block style preference: %s (handled by prettier defaults)", mdConfig.CodeBlockStyle))

	// Add file argument
	args = append(args, "--stdin-filepath", file)

	// #nosec G204 - prettierPath comes from findToolPath which validates paths
	cmd := exec.Command(prettierPath, args...)
	cmd.Stdin = strings.NewReader(string(originalContent))
	output, cmdErr := cmd.Output()
	if cmdErr != nil {
		return formatpkg.ToolExecution(file, "prettier", fmt.Errorf("prettier failed: %v", cmdErr))
	}

	// Handle trailing spaces - use finalizer options if specified, otherwise use config
	formatted := string(output)
	if options.TrimTrailingWhitespace {
		// Use finalizer logic which handles markdown hard breaks properly
		lines := strings.Split(formatted, "\n")
		for i, line := range lines {
			if options.PreserveMarkdownHardBreaks {
				// Count trailing spaces
				n := 0
				for j := len(line) - 1; j >= 0; j-- {
					if line[j] == ' ' {
						n++
						continue
					}
					break
				}
				if n >= 2 {
					// Collapse to exactly two spaces (preserve markdown hard breaks)
					lines[i] = strings.TrimRight(line, " \t") + "  "
					continue
				}
			}
			// Trim all trailing whitespace
			lines[i] = strings.TrimRight(line, " \t")
		}
		formatted = strings.Join(lines, "\n")
	} else if mdConfig.TrailingSpaces {
		// Fallback to config-based trimming
		lines := strings.Split(formatted, "\n")
		for i, line := range lines {
			lines[i] = strings.TrimRight(line, " \t")
		}
		formatted = strings.Join(lines, "\n")
	}

	// Apply remaining finalizer options if requested (EOF, line endings, BOM)
	if options.EnsureEOF || options.NormalizeLineEndings != "" || options.RemoveUTF8BOM {
		// Create options without TrimTrailingWhitespace since we handled it above
		finalizerOptions := options
		finalizerOptions.TrimTrailingWhitespace = false

		finalized, changed, err := finalizer.ComprehensiveFileNormalization([]byte(formatted), finalizerOptions)
		if err != nil {
			return fmt.Errorf("finalizer error: %v", err)
		}
		if changed {
			formatted = string(finalized)
		}
	}

	// Check if any formatting changed the content
	contentChanged := !bytes.Equal(originalContent, []byte(formatted))

	if !contentChanged {
		return formatpkg.ErrAlreadyFormatted
	}

	if checkOnly {
		return formatpkg.FormatDrift(file)
	}

	logger.Info(fmt.Sprintf("Applying Markdown formatting changes to %s", file))
	if err := safeio.WriteFileValidated(file, []byte(formatted), 0o600); err != nil {
		return formatpkg.FileIO(file, err)
	}
	return nil
}

// formatPythonFile formats a Python file using ruff format
func formatPythonFile(file string, checkOnly bool, _ *config.Config, options finalizer.NormalizationOptions, ignoreMissingTools bool, ruffPath string) error {
	// Validate file path to prevent path traversal
	file = filepath.Clean(file)
	if strings.Contains(file, "..") {
		return fmt.Errorf("invalid file path: contains path traversal")
	}

	if ruffPath == "" {
		if ignoreMissingTools {
			// Fall back to finalizer-only
			return applyFinalizer(file, checkOnly, options)
		}
		return formatpkg.ToolUnavailable(file, "ruff", "Install with: goneat doctor tools --scope python --install")
	}

	// Read the original content
	originalContent, err := os.ReadFile(file)
	if err != nil {
		return formatpkg.FileIO(file, err)
	}

	if checkOnly {
		// Use ruff check --select=I --diff to check formatting
		// #nosec G204 - ruffPath comes from findToolPath which validates paths
		cmd := exec.Command(ruffPath, "format", "--check", file)
		output, err := cmd.CombinedOutput()

		if err != nil {
			// ruff returns exit code 1 if formatting is needed
			if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
				return formatpkg.FormatDrift(file)
			}
			return formatpkg.ToolExecution(file, "ruff", fmt.Errorf("ruff format check failed: %v\nOutput: %s", err, string(output)))
		}
		// Exit code 0 means file is already formatted
		return formatpkg.ErrAlreadyFormatted
	}

	// Format the file
	// #nosec G204 - ruffPath comes from findToolPath which validates paths
	cmd := exec.Command(ruffPath, "format", file)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return formatpkg.ToolExecution(file, "ruff", fmt.Errorf("ruff format failed: %v\nOutput: %s", err, string(output)))
	}

	// Re-read formatted content
	formattedContent, err := os.ReadFile(file)
	if err != nil {
		return formatpkg.FileIO(file, fmt.Errorf("failed to re-read file after ruff format: %v", err))
	}

	// Apply finalizer
	if options.EnsureEOF || options.TrimTrailingWhitespace || options.NormalizeLineEndings != "" || options.RemoveUTF8BOM {
		finalized, changed, err := finalizer.ComprehensiveFileNormalization(formattedContent, options)
		if err != nil {
			return fmt.Errorf("finalizer error: %v", err)
		}
		if changed {
			if err := safeio.WriteFileValidated(file, finalized, 0o600); err != nil {
				return formatpkg.FileIO(file, fmt.Errorf("failed to write finalized file: %v", err))
			}
			formattedContent = finalized
		}
	}

	// Check if content changed
	if bytes.Equal(originalContent, formattedContent) {
		return formatpkg.ErrAlreadyFormatted
	}

	logger.Info(fmt.Sprintf("Applying Python formatting changes to %s", file))
	return nil
}

// formatJavaScriptFile formats a JavaScript/TypeScript file using biome format
func formatJavaScriptFile(file string, checkOnly bool, _ *config.Config, options finalizer.NormalizationOptions, ignoreMissingTools bool, biomePath string) error {
	// Validate file path to prevent path traversal
	file = filepath.Clean(file)
	if strings.Contains(file, "..") {
		return fmt.Errorf("invalid file path: contains path traversal")
	}

	if biomePath == "" {
		if ignoreMissingTools {
			// Fall back to finalizer-only
			return applyFinalizer(file, checkOnly, options)
		}
		return formatpkg.ToolUnavailable(file, "biome", "Install with: goneat doctor tools --scope typescript --install")
	}

	// Verify biome version is 2.x (1.x used --check which was removed in 2.x)
	if err := checkBiomeVersionCmd(biomePath); err != nil {
		return formatpkg.ToolExecution(file, "biome", err)
	}

	// Read the original content
	originalContent, err := os.ReadFile(file)
	if err != nil {
		return formatpkg.FileIO(file, err)
	}

	// Resolve biome invocation context for nested config support
	cmdDir, fileArg, err := work.ResolveBiomeContext(file)
	if err != nil {
		return formatpkg.FileIO(file, fmt.Errorf("biome context resolution failed for %s: %w", file, err))
	}

	if checkOnly {
		// Use biome format (without --write) to check formatting
		// Biome 2.x: running without --write performs a dry-run check (exit 0=formatted, 1=needs formatting)
		// #nosec G204 - biomePath comes from findToolPath which validates paths
		cmd := exec.Command(biomePath, "format", fileArg)
		cmd.Dir = cmdDir
		output, err := cmd.CombinedOutput()

		if err != nil {
			// biome returns exit code 1 if formatting is needed
			if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
				return formatpkg.FormatDrift(file)
			}
			return formatpkg.ToolExecution(file, "biome", fmt.Errorf("biome format check failed: %v\nOutput: %s", err, string(output)))
		}
		// Exit code 0 means file is already formatted
		return formatpkg.ErrAlreadyFormatted
	}

	// Format the file
	// #nosec G204 - biomePath comes from findToolPath which validates paths
	cmd := exec.Command(biomePath, "format", "--write", fileArg)
	cmd.Dir = cmdDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		return formatpkg.ToolExecution(file, "biome", fmt.Errorf("biome format failed: %v\nOutput: %s", err, string(output)))
	}

	// Re-read formatted content
	formattedContent, err := os.ReadFile(file)
	if err != nil {
		return formatpkg.FileIO(file, fmt.Errorf("failed to re-read file after biome format: %v", err))
	}

	// Apply finalizer
	if options.EnsureEOF || options.TrimTrailingWhitespace || options.NormalizeLineEndings != "" || options.RemoveUTF8BOM {
		finalized, changed, err := finalizer.ComprehensiveFileNormalization(formattedContent, options)
		if err != nil {
			return fmt.Errorf("finalizer error: %v", err)
		}
		if changed {
			if err := safeio.WriteFileValidated(file, finalized, 0o600); err != nil {
				return formatpkg.FileIO(file, fmt.Errorf("failed to write finalized file: %v", err))
			}
			formattedContent = finalized
		}
	}

	// Check if content changed
	if bytes.Equal(originalContent, formattedContent) {
		return formatpkg.ErrAlreadyFormatted
	}

	logger.Info(fmt.Sprintf("Applying JavaScript/TypeScript formatting changes to %s", file))
	return nil
}

// executeSequential executes work items sequentially
type formatExecutionError struct {
	formatDrift     int
	toolUnavailable int
	toolExecution   int
	fileIO          int
	other           int
	causes          []error
}

func (e *formatExecutionError) Error() string {
	if e.toolUnavailable == 0 && e.toolExecution == 0 && e.fileIO == 0 && e.other == 0 {
		return fmt.Sprintf("%d files need formatting", e.formatDrift)
	}
	return fmt.Sprintf(
		"format failed: need-format=%d, tool-unavailable=%d, tool-execution=%d, file-io=%d, other=%d",
		e.formatDrift,
		e.toolUnavailable,
		e.toolExecution,
		e.fileIO,
		e.other,
	)
}

func (e *formatExecutionError) Unwrap() []error {
	return e.causes
}

// executeSequential executes work items sequentially
func executeSequentialWithOptions(files []string, checkOnly, quiet bool, cfg *config.Config, ignoreMissingTools bool, options finalizer.NormalizationOptions, useGoimports bool, textNormalize bool, jsonIndent string, jsonIndentCount int, jsonSizeWarningMB int, xmlIndent string, xmlIndentCount int, xmlSizeWarningMB int, toolPaths formatToolPaths) error {
	start := time.Now()
	var formattedCount, unchangedCount int
	runErr := &formatExecutionError{}
	totalFiles := len(files)
	showProgress := totalFiles > 10 && !quiet // Show progress for larger file sets

	if showProgress {
		logger.Info(fmt.Sprintf("Processing %d files...", totalFiles))
	}

	for i, file := range files {
		if err := processFile(file, checkOnly, quiet, cfg, ignoreMissingTools, options, useGoimports, textNormalize, jsonIndent, jsonIndentCount, jsonSizeWarningMB, xmlIndent, xmlIndentCount, xmlSizeWarningMB, toolPaths); err != nil {
			if errors.Is(err, formatpkg.ErrFormatDrift) || errors.Is(err, formatpkg.ErrFinalized) {
				// "finalized" is returned when finalizer makes changes (EOF, trailing spaces, line endings)
				// It should be treated as successful formatting, not an error
				if checkOnly {
					logger.Error(
						fmt.Sprintf("Formatting differs for %s", file),
						logger.Err(err),
						logger.String("result_class", string(formatpkg.ResultFormatDrift)),
					)
					runErr.formatDrift++
					runErr.causes = append(runErr.causes, err)
				} else {
					formattedCount++
					if !quiet {
						logger.Info(fmt.Sprintf("Formatted %s", file))
					}
				}
			} else if errors.Is(err, formatpkg.ErrAlreadyFormatted) {
				unchangedCount++
				if !quiet && !checkOnly {
					logger.Debug(fmt.Sprintf("%s already properly formatted", file))
				}
			} else {
				class := formatpkg.ClassOf(err)
				fields := []logger.Field{logger.Err(err)}
				if class != "" {
					fields = append(fields, logger.String("result_class", string(class)))
				}
				logger.Error(fmt.Sprintf("Failed to process %s", file), fields...)
				switch class {
				case formatpkg.ResultToolUnavailable:
					runErr.toolUnavailable++
				case formatpkg.ResultToolExecution:
					runErr.toolExecution++
				case formatpkg.ResultFileIO:
					runErr.fileIO++
				default:
					runErr.other++
				}
				runErr.causes = append(runErr.causes, err)
			}
		} else {
			// For cases where no error is returned (shouldn't happen with new logic)
			formattedCount++
			if !quiet && !checkOnly {
				logger.Info(fmt.Sprintf("Formatted %s", file))
			}
		}

		// Show progress for larger file sets
		if showProgress && (i+1)%10 == 0 {
			progress := float64(i+1) / float64(totalFiles) * 100
			logger.Info(fmt.Sprintf("Progress: %d/%d files (%.1f%%) - %d formatted, %d unchanged, %d errors",
				i+1, totalFiles, progress, formattedCount, unchangedCount, len(runErr.causes)))
		}
	}

	duration := time.Since(start)

	if !quiet {
		if checkOnly {
			if runErr.formatDrift > 0 {
				logger.Warn(fmt.Sprintf("Found %d files that need formatting", runErr.formatDrift))
			} else {
				logger.Info("All files are properly formatted")
			}
			logger.Info(fmt.Sprintf("Summary (sequential): files=%d, ok=%d, need-format=%d, tool-unavailable=%d, tool-execution=%d, file-io=%d, other=%d, runtime=%v",
				len(files), unchangedCount, runErr.formatDrift, runErr.toolUnavailable, runErr.toolExecution, runErr.fileIO, runErr.other, duration))
		} else {
			logger.Info(fmt.Sprintf("Processed %d files (%d formatted, %d unchanged, %d errors) in %v (workers=1)",
				len(files), formattedCount, unchangedCount, len(runErr.causes), duration))
		}
	}

	if len(runErr.causes) > 0 {
		return runErr
	}

	return nil
}

// getStagedFilesFormat returns staged files (ACMR) for format command
func getStagedFilesFormat() ([]string, error) {
	cmd := exec.Command("git", "diff", "--cached", "--name-only", "--diff-filter=ACMR")
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	var files []string
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			files = append(files, line)
		}
	}
	return files, nil
}

var formatStagedFiles = getStagedFilesFormat

// executeParallel executes work items in parallel using the dispatcher
func executeParallel(files []string, cfg *config.Config, quiet bool, checkOnly bool, noOp bool, ignoreMissingTools bool, options finalizer.NormalizationOptions, textNormalize bool, jsonIndent string, jsonIndentCount int, jsonSizeWarningMB int, workers int, toolPaths formatToolPaths) error {
	// Supported content types for parallel processing (must match FormatProcessor.GetSupportedContentTypes)
	supportedTypes := make(map[string]bool)
	for _, contentType := range formatpkg.SupportedContentTypes() {
		supportedTypes[contentType] = true
	}
	if textNormalize {
		supportedTypes["unknown"] = true
	}

	// Create work items, filtering to only supported content types
	var workItems []work.WorkItem
	skippedCount := 0
	for i, file := range files {
		contentType := getContentTypeFromPath(file)

		// Skip unsupported content types
		if !supportedTypes[contentType] {
			if !quiet {
				logger.Debug(fmt.Sprintf("Skipping %s: unsupported content type '%s' for parallel processing", file, contentType))
			}
			skippedCount++
			continue
		}

		size := getFileSize(file)

		workItems = append(workItems, work.WorkItem{
			ID:            fmt.Sprintf("file_%d", i),
			Path:          file,
			ContentType:   contentType,
			Size:          size,
			Priority:      1,
			EstimatedTime: 1.0, // Simplified
			Metadata:      make(map[string]interface{}),
		})
	}

	if skippedCount > 0 && !quiet {
		logger.Info(fmt.Sprintf("Skipped %d files with unsupported content types for parallel processing", skippedCount))
	}

	if len(workItems) == 0 {
		if !quiet {
			logger.Info("No supported files for parallel processing")
		}
		return nil
	}

	// Create work item IDs slice
	workItemIDs := make([]string, len(workItems))
	for i := range workItems {
		workItemIDs[i] = workItems[i].ID
	}

	// Create a simple group
	group := work.WorkGroup{
		ID:                         "all_files",
		Name:                       "All Files",
		Strategy:                   "parallel",
		WorkItemIDs:                workItemIDs,
		EstimatedTotalTime:         float64(len(workItems)),
		RecommendedParallelization: resolveParallelWorkers(workers),
	}

	// Create manifest
	manifest := &work.WorkManifest{
		Plan: work.Plan{
			Command:           "format",
			Timestamp:         time.Now(),
			WorkingDirectory:  ".",
			TotalFiles:        len(files),
			FilteredFiles:     len(workItems),
			ExecutionStrategy: "parallel",
		},
		WorkItems: workItems,
		Groups:    []work.WorkGroup{group},
	}

	// Create processor and dispatcher
	workerCount := resolveParallelWorkers(workers)
	processor := work.NewFormatProcessorWithOptions(cfg, work.FormatProcessorOptions{
		FinalizerOptions:   options,
		IgnoreMissingTools: ignoreMissingTools,
		TextNormalize:      textNormalize,
		JSONIndent:         jsonIndent,
		JSONIndentCount:    jsonIndentCount,
		JSONSizeWarningMB:  jsonSizeWarningMB,
		ToolPaths:          toolPaths.FormatProcessorToolPaths,
		ToolsResolved:      true,
	})

	// Progress tracking for parallel execution
	var processedCount int32
	totalFiles := len(workItems)
	showProgress := totalFiles > 10 && !quiet

	dispatcher := work.NewDispatcher(work.DispatcherConfig{
		MaxWorkers: workerCount,
		DryRun:     false,
		NoOp:       checkOnly || noOp, // Check mode uses NoOp to prevent modifications
		ProgressCallback: func(result work.ExecutionResult) {
			processed := int(atomic.AddInt32(&processedCount, 1))

			if !quiet {
				if result.Success {
					if showProgress && processed%10 == 0 {
						progress := float64(processed) / float64(totalFiles) * 100
						logger.Info(fmt.Sprintf("Progress: %d/%d files (%.1f%%) - parallel processing", processed, totalFiles, progress))
					} else if !showProgress {
						logger.Info(fmt.Sprintf("Processed %s", result.WorkItemID))
					}
				} else {
					fields := []logger.Field{logger.String("work_item_id", result.WorkItemID)}
					if result.ResultClass != "" {
						fields = append(fields, logger.String("result_class", string(result.ResultClass)))
					}
					logger.Error(fmt.Sprintf("Failed %s: %s", result.WorkItemID, result.Error), fields...)
				}
			}
		},
	}, processor)

	// Execute
	ctx := context.Background()
	summary, err := dispatcher.ExecuteManifest(ctx, manifest)
	if err != nil {
		return fmt.Errorf("parallel execution failed: %v", err)
	}

	runErr := &formatExecutionError{}
	for _, result := range summary.Results {
		if result.Success {
			continue
		}
		resultErr := formatpkg.NewResultError(result.ResultClass, "", "", errors.New(result.Error))
		switch result.ResultClass {
		case formatpkg.ResultFormatDrift:
			runErr.formatDrift++
		case formatpkg.ResultToolUnavailable:
			runErr.toolUnavailable++
		case formatpkg.ResultToolExecution:
			runErr.toolExecution++
		case formatpkg.ResultFileIO:
			runErr.fileIO++
		default:
			runErr.other++
			resultErr = errors.New(result.Error)
		}
		runErr.causes = append(runErr.causes, resultErr)
	}

	// Report results
	if !quiet {
		avgPerFile := time.Duration(0)
		if len(files) > 0 {
			avgPerFile = summary.TotalDuration / time.Duration(len(files))
		}
		logger.Info(fmt.Sprintf("Parallel execution: files=%d, workers=%d, ok=%d, need-format=%d, tool-unavailable=%d, tool-execution=%d, file-io=%d, other=%d, total=%v, avg/file=%v",
			len(files), workerCount, summary.Successful, runErr.formatDrift, runErr.toolUnavailable, runErr.toolExecution, runErr.fileIO, runErr.other, summary.TotalDuration, avgPerFile))
	}

	if summary.Failed > 0 {
		return runErr
	}

	return nil
}

func resolveParallelWorkers(requested int) int {
	if requested > 0 {
		return requested
	}
	workers := runtime.NumCPU()
	if workers < 1 {
		workers = 1
	}
	if workers > 8 {
		workers = 8
	}
	return workers
}

// getContentTypeFromPath determines content type from file path
func getContentTypeFromPath(path string) string {
	return formatpkg.ContentTypeForPath(path)
}

// isXMLFile checks if a file is XML by reading the first few bytes
func isXMLFile(file string) bool {
	// #nosec G304 - file path comes from controlled sources (filesystem discovery or git operations)
	data, err := os.ReadFile(file)
	if err != nil {
		return false
	}
	// Check if file starts with <?xml
	return bytes.HasPrefix(data, []byte("<?xml"))
}

// getFileSize gets file size
func getFileSize(path string) int64 {
	info, err := os.Stat(path)
	if err != nil {
		return 0
	}
	return info.Size()
}

// handlePlanOnly handles --dry-run and --plan-only modes
func handlePlanOnly(cmd *cobra.Command, manifest *work.WorkManifest, planFile string, dryRun bool) error {
	logger.Info(fmt.Sprintf("handlePlanOnly called with planFile='%s', dryRun=%t", planFile, dryRun))
	out := cmd.OutOrStdout()

	if dryRun {
		if _, err := fmt.Fprintln(out, "🔍 DRY RUN - Would process the following:"); err != nil {
			return fmt.Errorf("failed to write dry run header: %v", err)
		}
		if _, err := fmt.Fprintln(out, ""); err != nil {
			return fmt.Errorf("failed to write dry run newline: %v", err)
		}
	}

	if _, err := fmt.Fprintf(out, "📋 Work Plan for '%s' command\n", manifest.Plan.Command); err != nil {
		return fmt.Errorf("failed to write plan header: %v", err)
	}
	if _, err := fmt.Fprintf(out, "Generated: %s\n", manifest.Plan.Timestamp.Format(time.RFC3339)); err != nil {
		return fmt.Errorf("failed to write timestamp: %v", err)
	}
	if _, err := fmt.Fprintf(out, "Working Directory: %s\n", manifest.Plan.WorkingDirectory); err != nil {
		return fmt.Errorf("failed to write working directory: %v", err)
	}
	if _, err := fmt.Fprintf(out, "Execution Strategy: %s\n", manifest.Plan.ExecutionStrategy); err != nil {
		return fmt.Errorf("failed to write execution strategy: %v", err)
	}
	if _, err := fmt.Fprintln(out, ""); err != nil {
		return fmt.Errorf("failed to write section separator: %v", err)
	}

	if _, err := fmt.Fprintf(out, "📊 Summary:\n"); err != nil {
		return fmt.Errorf("failed to write summary header: %v", err)
	}
	if _, err := fmt.Fprintf(out, "  Total files discovered: %d\n", manifest.Plan.TotalFiles); err != nil {
		return fmt.Errorf("failed to write total files: %v", err)
	}
	if _, err := fmt.Fprintf(out, "  Files after filtering: %d\n", manifest.Plan.FilteredFiles); err != nil {
		return fmt.Errorf("failed to write filtered files: %v", err)
	}
	if len(manifest.Plan.RedundantPaths) > 0 {
		if _, err := fmt.Fprintf(out, "  Redundant paths eliminated: %d\n", len(manifest.Plan.RedundantPaths)); err != nil {
			return fmt.Errorf("failed to write redundant paths: %v", err)
		}
	}
	if _, err := fmt.Fprintln(out, ""); err != nil {
		return fmt.Errorf("failed to write summary separator: %v", err)
	}

	if _, err := fmt.Fprintf(out, "📁 Files by Type:\n"); err != nil {
		return fmt.Errorf("failed to write files by type header: %v", err)
	}
	for contentType, count := range manifest.Statistics.FilesByType {
		if _, err := fmt.Fprintf(out, "  %s: %d files\n", contentType, count); err != nil {
			return fmt.Errorf("failed to write file type count: %v", err)
		}
	}
	if _, err := fmt.Fprintln(out, ""); err != nil {
		return fmt.Errorf("failed to write file types separator: %v", err)
	}

	if _, err := fmt.Fprintf(out, "📂 Work Groups (%d groups):\n", len(manifest.Groups)); err != nil {
		return fmt.Errorf("failed to write work groups header: %v", err)
	}
	for _, group := range manifest.Groups {
		if _, err := fmt.Fprintf(out, "  • %s (%s): %d items, %.1fms estimated\n",
			group.Name, group.Strategy, len(group.WorkItemIDs), group.EstimatedTotalTime); err != nil {
			return fmt.Errorf("failed to write work group info: %v", err)
		}
		if group.RecommendedParallelization > 1 {
			if _, err := fmt.Fprintf(out, "    Recommended parallelization: %d workers\n", group.RecommendedParallelization); err != nil {
				return fmt.Errorf("failed to write parallelization info: %v", err)
			}
		}
	}
	if _, err := fmt.Fprintln(out, ""); err != nil {
		return fmt.Errorf("failed to write work groups separator: %v", err)
	}

	if _, err := fmt.Fprintf(out, "⏱️  Estimated Execution Times:\n"); err != nil {
		return fmt.Errorf("failed to write execution times header: %v", err)
	}
	if _, err := fmt.Fprintf(out, "  Sequential: %.1fms\n", manifest.Statistics.EstimatedExecutionTime.Sequential); err != nil {
		return fmt.Errorf("failed to write sequential time: %v", err)
	}
	if manifest.Statistics.EstimatedExecutionTime.Parallel2 > 0 {
		if _, err := fmt.Fprintf(out, "  Parallel (2 workers): %.1fms\n", manifest.Statistics.EstimatedExecutionTime.Parallel2); err != nil {
			return fmt.Errorf("failed to write parallel 2 time: %v", err)
		}
		if _, err := fmt.Fprintf(out, "  Parallel (4 workers): %.1fms\n", manifest.Statistics.EstimatedExecutionTime.Parallel4); err != nil {
			return fmt.Errorf("failed to write parallel 4 time: %v", err)
		}
		if _, err := fmt.Fprintf(out, "  Parallel (8 workers): %.1fms\n", manifest.Statistics.EstimatedExecutionTime.Parallel8); err != nil {
			return fmt.Errorf("failed to write parallel 8 time: %v", err)
		}
	}
	if _, err := fmt.Fprintln(out, ""); err != nil {
		return fmt.Errorf("failed to write execution times separator: %v", err)
	}

	if dryRun {
		if _, err := fmt.Fprintln(out, "💡 This was a dry run - no files were modified"); err != nil {
			return fmt.Errorf("failed to write dry run notice: %v", err)
		}
		if _, err := fmt.Fprintln(out, "   Remove --dry-run flag to execute the plan"); err != nil {
			return fmt.Errorf("failed to write dry run instruction: %v", err)
		}
	}

	// Write plan to file if requested
	if planFile != "" {
		// Use scratchpad directory if no absolute path provided
		if !filepath.IsAbs(planFile) {
			scratchpadDir, err := config.GetScratchpadDir()
			if err != nil {
				logger.Warn(fmt.Sprintf("Failed to get scratchpad directory: %v", err))
			} else {
				planFile = filepath.Join(scratchpadDir, planFile)
			}
		}

		if err := writePlanToFile(manifest, planFile); err != nil {
			logger.Warn(fmt.Sprintf("Failed to write plan to file: %v", err))
		} else {
			if _, err := fmt.Fprintf(out, "📄 Plan written to: %s\n", planFile); err != nil {
				return fmt.Errorf("failed to write plan file location: %v", err)
			}
		}
	}

	return nil
}

// writePlanToFile writes the work manifest to a file
func writePlanToFile(manifest *work.WorkManifest, filename string) error {
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}

	return safeio.WriteFileValidated(filename, data, 0o600)
}

// checkBiomeVersionCmd verifies that biome is version 2.x or higher.
// Biome 1.x used --check flag which was removed in 2.x.
func checkBiomeVersionCmd(biomePath string) error {
	// #nosec G204 -- biomePath comes from findToolPath
	cmd := exec.Command(biomePath, "--version")
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("failed to get biome version: %w", err)
	}

	// Output format: "Version: 2.3.10" or similar
	versionStr := strings.TrimSpace(string(output))
	versionStr = strings.TrimPrefix(versionStr, "Version: ")

	// Extract major version
	parts := strings.Split(versionStr, ".")
	if len(parts) == 0 {
		return fmt.Errorf("could not parse biome version: %s", versionStr)
	}

	major := 0
	for _, c := range parts[0] {
		if c >= '0' && c <= '9' {
			major = major*10 + int(c-'0')
		}
	}

	if major < 2 {
		return fmt.Errorf("biome version %s is not supported; goneat requires biome 2.x or higher. Upgrade with: npm install -g @biomejs/biome@latest", versionStr)
	}

	return nil
}

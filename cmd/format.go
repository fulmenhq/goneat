/*
Copyright © 2025 3 Leaps <info@3leaps.com>
*/
package cmd

import (
	"bytes"
	"context"
	"encoding/json"
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
	"github.com/fulmenhq/goneat/pkg/exitcode"
	formatpkg "github.com/fulmenhq/goneat/pkg/format"
	"github.com/fulmenhq/goneat/pkg/format/finalizer"
	"github.com/fulmenhq/goneat/pkg/logger"
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
	formatCmd.Flags().StringSlice("types", []string{}, "Content types to include (go, yaml, json, markdown)")
	formatCmd.Flags().Int("max-depth", -1, "Maximum directory depth to traverse")

	// Execution strategy flags
	formatCmd.Flags().String("strategy", "sequential", "Execution strategy (sequential, parallel)")
	formatCmd.Flags().Bool("group-by-size", false, "Group work items by file size")
	formatCmd.Flags().Bool("group-by-type", false, "Group work items by content type")

	// Dogfooding helpers
	formatCmd.Flags().Bool("staged-only", false, "Only format staged files in git (changed and added)")
	formatCmd.Flags().Bool("ignore-missing-tools", false, "Skip files requiring external formatters if tools are missing")

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

// toolExists checks if a tool is available (in PATH or shim directories)
func toolExists(toolName string) bool {
	return findToolPath(toolName) != ""
}

func RunFormat(cmd *cobra.Command, args []string) error {
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
		}
	}

	if stagedOnly {
		if len(explicitFiles) > 0 {
			return fmt.Errorf("cannot use --staged-only with --files: staged mode discovers files from git, --files specifies exact files")
		}
		staged, err := getStagedFilesFormat()
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

	if !quiet {
		logger.Info(fmt.Sprintf("Processing %d files using %s strategy", len(filesToProcess), strategy))
	}

	// Execute based on strategy
	if strategy == "parallel" && !dryRun && !planOnly && !stagedOnly {
		if useGoimports {
			logger.Warn("use-goimports is enabled but parallel processor does not apply goimports yet; skipping import alignment in parallel mode")
		}
		return executeParallel(filesToProcess, cfg, quiet, noOp)
	} else {
		// Create normalization options
		options := finalizer.NormalizationOptions{
			EnsureEOF:                  finalizeEOF,
			TrimTrailingWhitespace:     finalizeTrimTrailingSpaces,
			NormalizeLineEndings:       finalizeLineEndings,
			RemoveUTF8BOM:              finalizeRemoveBOM,
			PreserveMarkdownHardBreaks: preserveMd,
			EncodingPolicy:             textEncodingPolicy,
		}

		return executeSequentialWithOptions(filesToProcess, checkOnly || noOp, quiet, cfg, ignoreMissingTools, options, useGoimports, textNormalize, jsonIndent, jsonIndentCount, jsonSizeWarningMB, xmlIndent, xmlIndentCount, xmlSizeWarningMB)
	}
}

// removed unused findSupportedFiles helper

func processFile(file string, checkOnly bool, _ bool, cfg *config.Config, ignoreMissingTools bool, options finalizer.NormalizationOptions, useGoimports bool, textNormalize bool, jsonIndent string, jsonIndentCount int, jsonSizeWarningMB int, xmlIndent string, xmlIndentCount int, xmlSizeWarningMB int) error {
	ext := filepath.Ext(file)

	var err error

	switch ext {
	case ".go":
		err = formatGoFile(file, checkOnly, cfg, useGoimports, ignoreMissingTools, options)

	case ".yaml", ".yml":
		// Skip if yamlfmt missing and ignoring missing tools
		if ignoreMissingTools {
			if !toolExists("yamlfmt") {
				logger.Warn("yamlfmt not found; skipping YAML formatting for this file")
				// Even if skipping primary formatter, still allow finalizer below
				break
			}
		}
		err = formatYAMLFile(file, checkOnly, cfg, options)

	case ".json":
		if ignoreMissingTools {
			if !toolExists("jq") {
				logger.Warn("jq not found; skipping JSON formatting for this file")
				break
			}
		}
		err = formatJSONFile(file, checkOnly, cfg, options, jsonIndent, jsonIndentCount, jsonSizeWarningMB)

	case ".xml":
		err = formatXMLFile(file, checkOnly, cfg, options, xmlIndent, xmlIndentCount, xmlSizeWarningMB)

	case ".md":
		if ignoreMissingTools {
			if !toolExists("prettier") {
				logger.Warn("prettier not found; skipping Markdown formatting for this file")
				break
			}
		}
		err = formatMarkdownFile(file, checkOnly, cfg, options)

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
			supportedExts := []string{".go", ".yaml", ".yml", ".json", ".xml", ".md", ".markdown"}
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
				if ferr.Error() == "finalized" {
					// Finalizer made changes - this takes precedence over "already formatted"
					return fmt.Errorf("needs formatting")
				}
				// If primary formatter said "already formatted" but finalizer found changes,
				// the finalizer result takes precedence
				if err != nil && err.Error() == "already formatted" {
					return ferr
				}
				// If primary formatter had other errors, return the finalizer error
				if err != nil {
					return ferr
				}
				// If primary formatter succeeded but finalizer found issues, return finalizer error
				return ferr
			}
			// If finalizer succeeded and primary formatter said "already formatted",
			// we should indicate that changes were made
			if err != nil && err.Error() == "already formatted" {
				return fmt.Errorf("needs formatting")
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
		return err
	}

	if checkOnly {
		// Use the same comprehensive normalization function for consistent checking
		if _, changed, err := finalizer.ComprehensiveFileNormalization(content, options); err != nil {
			return fmt.Errorf("normalization error: %v", err)
		} else if changed {
			logger.Info(fmt.Sprintf("File %s needs formatting (EOF, trailing whitespace, line endings, or BOM issues)", file))
			return fmt.Errorf("needs formatting")
		}
		return fmt.Errorf("already formatted")
	}

	// Apply comprehensive normalization
	finalized, changed, err := finalizer.ComprehensiveFileNormalization(content, options)
	if err != nil {
		return fmt.Errorf("finalizer error: %v", err)
	}

	if changed {
		// Write back the finalized content
		if err := os.WriteFile(file, finalized, 0600); err != nil {
			return fmt.Errorf("failed to write finalized file: %v", err)
		}
		// Return an error to indicate changes were made (finalizer takes precedence)
		return fmt.Errorf("finalized")
	}

	// No changes needed
	return nil
}

func formatGoFile(file string, checkOnly bool, cfg *config.Config, useGoimports bool, ignoreMissingTools bool, options finalizer.NormalizationOptions) error {
	// Validate file path to prevent path traversal
	file = filepath.Clean(file)
	if strings.Contains(file, "..") {
		return fmt.Errorf("invalid file path: contains path traversal")
	}

	original, err := os.ReadFile(file)
	if err != nil {
		return err
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
		return err
	}

	final := gofmtOut
	// Step 2: goimports (optional)
	if useGoimports {
		goimportsPath := findToolPath("goimports")
		if goimportsPath == "" {
			if !ignoreMissingTools {
				return fmt.Errorf("goimports not found but --use-goimports was specified.\nInstall with: go install golang.org/x/tools/cmd/goimports@latest\nOr run: goneat doctor tools --install goimports\nTip: use --ignore-missing-tools to skip import alignment")
			}
			logger.Debug("goimports not found; skipping import alignment due to --ignore-missing-tools")
		} else {
			// #nosec G204 - goimportsPath comes from findToolPath which validates paths
			cmd := exec.Command(goimportsPath)
			cmd.Dir = filepath.Dir(file)
			cmd.Stdin = strings.NewReader(string(final))
			out, cmdErr := cmd.Output()
			if cmdErr != nil {
				return fmt.Errorf("goimports failed for %s: %v\nTry: go install golang.org/x/tools/cmd/goimports@latest or 'goneat doctor tools --install goimports'", file, cmdErr)
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
			return fmt.Errorf("needs formatting")
		}
		return fmt.Errorf("already formatted")
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
		return os.WriteFile(file, final, 0600)
	}

	return fmt.Errorf("already formatted")
}

func formatYAMLFile(file string, checkOnly bool, cfg *config.Config, options finalizer.NormalizationOptions) error {
	// Clean file path to prevent path traversal
	file = filepath.Clean(file)

	// Find yamlfmt (checks PATH, then shim directories for CI environments)
	yamlfmtPath := findToolPath("yamlfmt")
	if yamlfmtPath == "" {
		return fmt.Errorf("yamlfmt not found. Install with: goneat doctor tools --install yamlfmt")
	}

	yamlConfig := cfg.GetYAMLConfig()

	// For proper change detection, read original content first
	originalContent, err := os.ReadFile(file)
	if err != nil {
		return err
	}

	// Build yamlfmt arguments
	var args []string

	// Add configuration options using -formatter flag
	var formatterOpts []string
	if yamlConfig.Indent != 2 {
		formatterOpts = append(formatterOpts, fmt.Sprintf("indent=%d", yamlConfig.Indent))
	}
	if yamlConfig.LineLength != 80 {
		formatterOpts = append(formatterOpts, fmt.Sprintf("line_length=%d", yamlConfig.LineLength))
	}
	// Note: yamlfmt's current version has limited formatter options
	// Quote style and other options would need a .yamlfmt config file

	for _, opt := range formatterOpts {
		args = append(args, "-formatter", opt)
	}

	if checkOnly {
		// Use -lint flag for check mode
		args = append(args, "-lint", file)

		logger.Debug(fmt.Sprintf("Running yamlfmt with args: %v", args))

		// #nosec G204 - yamlfmtPath comes from findToolPath which validates paths
		cmd := exec.Command(yamlfmtPath, args...)
		output, err := cmd.CombinedOutput()

		// In lint mode, yamlfmt returns exit code 1 if formatting is needed
		if err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
				return fmt.Errorf("needs formatting")
			}
			// Real error
			return fmt.Errorf("yamlfmt failed: %v\nOutput: %s", err, string(output))
		}
		// Exit code 0 means file is already formatted
		return fmt.Errorf("already formatted")
	}

	// For format mode, we need to detect if changes will be made
	// First run with -dry flag to check
	dryArgs := append([]string{"-dry"}, args...)
	dryArgs = append(dryArgs, file)

	// #nosec G204 - yamlfmtPath comes from findToolPath which validates paths
	dryCmd := exec.Command(yamlfmtPath, dryArgs...)
	dryOutput, dryErr := dryCmd.CombinedOutput()

	if dryErr != nil && !strings.Contains(string(dryOutput), "---") {
		// Real error, not just formatting output
		return fmt.Errorf("yamlfmt dry run failed: %v\nOutput: %s", dryErr, string(dryOutput))
	}

	// Check if yamlfmt formatting is needed by comparing dry output
	yamlfmtWillChange := len(dryOutput) > 0 && !bytes.Equal(originalContent, dryOutput)

	// Check if finalizer would make changes
	finalizerWillChange := false
	if options.EnsureEOF || options.TrimTrailingWhitespace || options.NormalizeLineEndings != "" || options.RemoveUTF8BOM {
		if _, changed, err := finalizer.ComprehensiveFileNormalization(originalContent, options); err == nil && changed {
			finalizerWillChange = true
		}
	}

	if !yamlfmtWillChange && !finalizerWillChange {
		return fmt.Errorf("already formatted")
	}

	// Now apply the formatting
	formatArgs := append(args, file)
	logger.Debug(fmt.Sprintf("Running yamlfmt with args: %v", formatArgs))

	// #nosec G204 - yamlfmtPath comes from findToolPath which validates paths
	cmd := exec.Command(yamlfmtPath, formatArgs...)
	output, err := cmd.CombinedOutput()

	if err != nil {
		return fmt.Errorf("yamlfmt failed: %v\nOutput: %s", err, string(output))
	}

	// Apply finalizer options if requested (EOF, trailing whitespace, etc.)
	// Re-read the file to get the current content after yamlfmt formatting
	currentContent, readErr := os.ReadFile(file)
	if readErr != nil {
		return fmt.Errorf("failed to re-read file after formatting: %v", readErr)
	}

	if options.EnsureEOF || options.TrimTrailingWhitespace || options.NormalizeLineEndings != "" || options.RemoveUTF8BOM {
		finalized, changed, err := finalizer.ComprehensiveFileNormalization(currentContent, options)
		if err != nil {
			return fmt.Errorf("finalizer error: %v", err)
		}
		if changed {
			if err := os.WriteFile(file, finalized, 0600); err != nil {
				return fmt.Errorf("failed to write finalized file: %v", err)
			}
		}
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
		return err
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
		if checkOnly {
			return fmt.Errorf("already formatted")
		}
		return fmt.Errorf("already formatted") // Signal that no changes were made
	}

	if checkOnly {
		return fmt.Errorf("needs formatting")
	}

	logger.Info(fmt.Sprintf("Applying JSON formatting changes to %s", file))
	return os.WriteFile(file, []byte(formatted), 0600)
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
		return err
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
		if checkOnly {
			return fmt.Errorf("already formatted")
		}
		return fmt.Errorf("already formatted") // Signal that no changes were made
	}

	if checkOnly {
		return fmt.Errorf("needs formatting")
	}

	logger.Info(fmt.Sprintf("Applying XML formatting changes to %s", file))
	return os.WriteFile(file, []byte(formatted), 0600)
}

func formatMarkdownFile(file string, checkOnly bool, cfg *config.Config, options finalizer.NormalizationOptions) error {
	// Validate file path to prevent path traversal
	file = filepath.Clean(file)
	if strings.Contains(file, "..") {
		return fmt.Errorf("invalid file path: contains path traversal")
	}

	// Find prettier (checks PATH, then shim directories for CI environments)
	prettierPath := findToolPath("prettier")
	if prettierPath == "" {
		return fmt.Errorf("prettier not found. Install with: goneat doctor tools --install prettier")
	}

	mdConfig := cfg.GetMarkdownConfig()

	// Read the original content
	originalContent, err := os.ReadFile(file)
	if err != nil {
		return err
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
		return fmt.Errorf("prettier failed: %v", cmdErr)
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
		if checkOnly {
			return fmt.Errorf("already formatted")
		}
		return fmt.Errorf("already formatted") // Signal that no changes were made
	}

	if checkOnly {
		return fmt.Errorf("needs formatting")
	}

	logger.Info(fmt.Sprintf("Applying Markdown formatting changes to %s", file))
	return os.WriteFile(file, []byte(formatted), 0600)
}

// executeSequential executes work items sequentially
func executeSequentialWithOptions(files []string, checkOnly, quiet bool, cfg *config.Config, ignoreMissingTools bool, options finalizer.NormalizationOptions, useGoimports bool, textNormalize bool, jsonIndent string, jsonIndentCount int, jsonSizeWarningMB int, xmlIndent string, xmlIndentCount int, xmlSizeWarningMB int) error {
	start := time.Now()
	var formattedCount, unchangedCount, errorCount int
	totalFiles := len(files)
	showProgress := totalFiles > 10 && !quiet // Show progress for larger file sets

	if showProgress {
		logger.Info(fmt.Sprintf("Processing %d files...", totalFiles))
	}

	for i, file := range files {
		if err := processFile(file, checkOnly, quiet, cfg, ignoreMissingTools, options, useGoimports, textNormalize, jsonIndent, jsonIndentCount, jsonSizeWarningMB, xmlIndent, xmlIndentCount, xmlSizeWarningMB); err != nil {
			if err.Error() == "needs formatting" || err.Error() == "finalized" {
				// "finalized" is returned when finalizer makes changes (EOF, trailing spaces, line endings)
				// It should be treated as successful formatting, not an error
				if checkOnly {
					logger.Error(fmt.Sprintf("Failed to process %s", file), logger.Err(err))
					errorCount++
				} else {
					formattedCount++
					if !quiet {
						logger.Info(fmt.Sprintf("Formatted %s", file))
					}
				}
			} else if err.Error() == "already formatted" {
				unchangedCount++
				if !quiet && !checkOnly {
					logger.Debug(fmt.Sprintf("%s already properly formatted", file))
				}
			} else {
				logger.Error(fmt.Sprintf("Failed to process %s", file), logger.Err(err))
				errorCount++
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
				i+1, totalFiles, progress, formattedCount, unchangedCount, errorCount))
		}
	}

	duration := time.Since(start)

	if !quiet {
		if checkOnly {
			if errorCount > 0 {
				logger.Warn(fmt.Sprintf("Found %d files that need formatting", errorCount))
			} else {
				logger.Info("All files are properly formatted")
			}
			logger.Info(fmt.Sprintf("Summary (sequential): files=%d, ok=%d, need-format=%d, runtime=%v",
				len(files), unchangedCount, errorCount, duration))
		} else {
			logger.Info(fmt.Sprintf("Processed %d files (%d formatted, %d unchanged, %d errors) in %v (workers=1)",
				len(files), formattedCount, unchangedCount, errorCount, duration))
		}
	}

	if checkOnly && errorCount > 0 {
		logger.Error(fmt.Sprintf("%d files need formatting", errorCount))
		os.Exit(exitcode.GeneralError)
	}

	if errorCount > 0 {
		os.Exit(exitcode.GeneralError)
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

// executeParallel executes work items in parallel using the dispatcher
func executeParallel(files []string, cfg *config.Config, quiet bool, noOp bool) error {
	// Create a simple manifest for the files
	workItems := make([]work.WorkItem, len(files))
	for i, file := range files {
		contentType := getContentTypeFromPath(file)
		size := getFileSize(file)

		workItems[i] = work.WorkItem{
			ID:            fmt.Sprintf("file_%d", i),
			Path:          file,
			ContentType:   contentType,
			Size:          size,
			Priority:      1,
			EstimatedTime: 1.0, // Simplified
			Metadata:      make(map[string]interface{}),
		}
	}

	// Create a simple group
	group := work.WorkGroup{
		ID:                         "all_files",
		Name:                       "All Files",
		Strategy:                   "parallel",
		WorkItemIDs:                make([]string, len(workItems)),
		EstimatedTotalTime:         float64(len(workItems)),
		RecommendedParallelization: runtime.NumCPU(),
	}
	for i := range workItems {
		group.WorkItemIDs[i] = workItems[i].ID
	}

	// Create manifest
	manifest := &work.WorkManifest{
		Plan: work.Plan{
			Command:           "format",
			Timestamp:         time.Now(),
			WorkingDirectory:  ".",
			TotalFiles:        len(files),
			FilteredFiles:     len(files),
			ExecutionStrategy: "parallel",
		},
		WorkItems: workItems,
		Groups:    []work.WorkGroup{group},
	}

	// Create processor and dispatcher
	workers := runtime.NumCPU()
	processor := work.NewFormatProcessor(cfg)

	// Progress tracking for parallel execution
	var processedCount int32
	totalFiles := len(files)
	showProgress := totalFiles > 10 && !quiet

	dispatcher := work.NewDispatcher(work.DispatcherConfig{
		MaxWorkers: workers,
		DryRun:     false,
		NoOp:       noOp,
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
					logger.Error(fmt.Sprintf("Failed %s: %s", result.WorkItemID, result.Error))
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

	// Report results
	if !quiet {
		avgPerFile := time.Duration(0)
		if len(files) > 0 {
			avgPerFile = summary.TotalDuration / time.Duration(len(files))
		}
		logger.Info(fmt.Sprintf("Parallel execution: files=%d, workers=%d, ok=%d, failed=%d, total=%v, avg/file=%v",
			len(files), workers, summary.Successful, summary.Failed, summary.TotalDuration, avgPerFile))
	}

	if summary.Failed > 0 {
		return fmt.Errorf("%d files failed to process", summary.Failed)
	}

	return nil
}

// getContentTypeFromPath determines content type from file path
func getContentTypeFromPath(path string) string {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".go":
		return "go"
	case ".yaml", ".yml":
		return "yaml"
	case ".json":
		return "json"
	case ".xml":
		return "xml"
	case ".md":
		return "markdown"
	default:
		return "unknown"
	}
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

	return os.WriteFile(filename, data, 0600)
}

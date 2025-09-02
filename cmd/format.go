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
	"slices"
	"strings"
	"time"

	"github.com/3leaps/goneat/internal/ops"
	"github.com/3leaps/goneat/pkg/config"
	"github.com/3leaps/goneat/pkg/exitcode"
	"github.com/3leaps/goneat/pkg/format/finalizer"
	"github.com/3leaps/goneat/pkg/logger"
	"github.com/3leaps/goneat/pkg/work"
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

	formatCmd.Flags().StringSliceP("files", "f", []string{}, "Specific files to format")
	formatCmd.Flags().Bool("check", false, "Check if files are formatted without modifying")
	formatCmd.Flags().Bool("quiet", false, "Suppress output except for errors")
	formatCmd.Flags().Bool("dry-run", false, "Show what would be done without executing")
	formatCmd.Flags().Bool("plan-only", false, "Generate and display work plan without executing")
	formatCmd.Flags().String("plan-file", "", "Write work plan to specified file")
	formatCmd.Flags().StringSlice("folders", []string{}, "Folders to process (alternative to positional args)")
	formatCmd.Flags().StringSlice("types", []string{}, "Content types to include (go, yaml, json, markdown)")
	formatCmd.Flags().Int("max-depth", -1, "Maximum directory depth to traverse")
	formatCmd.Flags().String("strategy", "sequential", "Execution strategy (sequential, parallel)")
	formatCmd.Flags().Bool("group-by-size", false, "Group work items by file size")
	formatCmd.Flags().Bool("group-by-type", false, "Group work items by content type")

	// Dogfooding helpers
	formatCmd.Flags().Bool("staged-only", false, "Only format staged files in git (changed and added)")
	formatCmd.Flags().Bool("ignore-missing-tools", false, "Skip files requiring external formatters if tools are missing")

	// EOF finalizer flags
	formatCmd.Flags().Bool("finalize-eof", true, "Ensure files end with exactly one newline")
	formatCmd.Flags().Bool("finalize-trim-trailing-spaces", false, "Remove trailing whitespace from all lines")
	formatCmd.Flags().String("finalize-line-endings", "", "Normalize line endings (lf, crlf, or auto)")
	formatCmd.Flags().Bool("finalize-remove-bom", false, "Remove Byte Order Mark (UTF-8, UTF-16, UTF-32)")

	// Import alignment (Go) - opt-in
	formatCmd.Flags().Bool("use-goimports", false, "Organize Go imports with goimports (after gofmt)")
}

func RunFormat(cmd *cobra.Command, args []string) error {
	logger.Info("Starting format command")

	// Load configuration
	cfg, err := config.LoadProjectConfig()
	if err != nil {
		// Config loading failed, use defaults (this is normal if no config file exists)
		cfg = &config.Config{}
	}

	// Get all flags
	files, _ := cmd.Flags().GetStringSlice("files")
	folders, _ := cmd.Flags().GetStringSlice("folders")
	checkOnly, _ := cmd.Flags().GetBool("check")
	quiet, _ := cmd.Flags().GetBool("quiet")
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
	useGoimports, _ := cmd.Flags().GetBool("use-goimports")

	// Check if no-op mode is enabled
	if noOp {
		logger.Info("Running in no-op mode - tasks will be executed but no files will be modified")
	}

	// Determine paths to process
	var paths []string
	if len(folders) > 0 {
		paths = folders
	} else if len(args) > 0 {
		paths = args
	} else {
		paths = []string{"."}
	}

	// Create planner configuration
	plannerConfig := work.PlannerConfig{
		Command:            "format",
		Paths:              paths,
		IncludePatterns:    files, // Use files as include patterns if specified
		ContentTypes:       contentTypes,
		MaxDepth:           maxDepth,
		ExecutionStrategy:  strategy,
		GroupBySize:        groupBySize,
		GroupByContentType: groupByType,
		IgnoreFile:         ".goneatignore",                           // Enable .goneatignore support
		EnableFinalizer:    finalizeEOF || finalizeTrimTrailingSpaces, // Enable finalizer support
	}

	var filesToProcess []string

	if stagedOnly {
		staged, err := getStagedFilesFormat()
		if err != nil {
			return fmt.Errorf("failed to get staged files: %v", err)
		}
		// Filter by content types if provided
		allowed := make(map[string]bool)
		for _, t := range contentTypes {
			allowed[strings.ToLower(strings.TrimSpace(t))] = true
		}
		for _, f := range staged {
			// If --files provided, restrict to those
			if len(files) > 0 && !slices.Contains(files, f) {
				continue
			}
			ct := getContentTypeFromPath(f)
			if len(allowed) == 0 || allowed[ct] {
				filesToProcess = append(filesToProcess, f)
			}
		}
		// In staged-only + (planOnly or dryRun), synthesize a small manifest for plan output
		if planOnly || dryRun {
			synth := &work.WorkManifest{Plan: work.Plan{Command: "format", Timestamp: time.Now(), WorkingDirectory: ".", TotalFiles: len(filesToProcess), FilteredFiles: len(filesToProcess), ExecutionStrategy: strategy}}
			return handlePlanOnly(cmd, synth, planFile, dryRun)
		}
	} else {
		// Planner path for non-staged runs
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
			EnsureEOF:              finalizeEOF,
			TrimTrailingWhitespace: finalizeTrimTrailingSpaces,
			NormalizeLineEndings:   finalizeLineEndings,
			RemoveUTF8BOM:          finalizeRemoveBOM,
		}

		return executeSequentialWithOptions(filesToProcess, checkOnly || noOp, quiet, cfg, ignoreMissingTools, options, useGoimports)
	}
}

// removed unused findSupportedFiles helper

func processFile(file string, checkOnly bool, _ bool, cfg *config.Config, ignoreMissingTools bool, options finalizer.NormalizationOptions, useGoimports bool) error {
	ext := filepath.Ext(file)

	var err error

	switch ext {
	case ".go":
		err = formatGoFile(file, checkOnly, cfg, useGoimports, ignoreMissingTools)

	case ".yaml", ".yml":
		// Skip if yamlfmt missing and ignoring missing tools
		if ignoreMissingTools {
			if _, e := exec.LookPath("yamlfmt"); e != nil {
				logger.Warn("yamlfmt not found; skipping YAML formatting for this file")
				// Even if skipping primary formatter, still allow finalizer below
				break
			}
		}
		err = formatYAMLFile(file, checkOnly, cfg)

	case ".json":
		if ignoreMissingTools {
			if _, e := exec.LookPath("jq"); e != nil {
				logger.Warn("jq not found; skipping JSON formatting for this file")
				break
			}
		}
		err = formatJSONFile(file, checkOnly, cfg)

	case ".md":
		if ignoreMissingTools {
			if _, e := exec.LookPath("prettier"); e != nil {
				logger.Warn("prettier not found; skipping Markdown formatting for this file")
				break
			}
		}
		err = formatMarkdownFile(file, checkOnly, cfg)

	default:
		// Non-primary types: apply finalizer only if enabled and extension supported
		if options.EnsureEOF || options.TrimTrailingWhitespace || options.NormalizeLineEndings != "" || options.RemoveUTF8BOM {
			if finalizer.IsSupportedExtension(ext) {
				return applyFinalizer(file, checkOnly, options)
			}
		}
		return fmt.Errorf("unsupported file type: %s", ext)
	}

	// Apply finalizer after primary formatter (when enabled and extension supported)
	if options.EnsureEOF || options.TrimTrailingWhitespace || options.NormalizeLineEndings != "" || options.RemoveUTF8BOM {
		if finalizer.IsSupportedExtension(ext) {
			if ferr := applyFinalizer(file, checkOnly, options); ferr != nil {
				return ferr
			}
		}
	}

	return err
}

// applyFinalizer applies comprehensive file normalization
func applyFinalizer(file string, checkOnly bool, options finalizer.NormalizationOptions) error {
	// Read the file content
	content, err := os.ReadFile(file)
	if err != nil {
		return err
	}

	// Apply comprehensive normalization
	finalized, changed, err := finalizer.ComprehensiveFileNormalization(content, options)
	if err != nil {
		return fmt.Errorf("finalizer error: %v", err)
	}

	if changed {
		if checkOnly {
			return fmt.Errorf("needs formatting")
		}

		// Write back the finalized content
		if err := os.WriteFile(file, finalized, 0644); err != nil {
			return fmt.Errorf("failed to write finalized file: %v", err)
		}
	} else {
		if checkOnly {
			return fmt.Errorf("already formatted")
		}
		// For apply mode, when no changes are needed, we should indicate this
		// But since the caller expects nil for success, we need a different approach
		// The issue is that the current design doesn't distinguish between "changed" and "unchanged" in apply mode
		// For now, we'll return nil and accept that unchanged files are counted as "formatted"
	}

	return nil
}

func formatGoFile(file string, checkOnly bool, cfg *config.Config, useGoimports bool, ignoreMissingTools bool) error {
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
		if _, lookErr := exec.LookPath("goimports"); lookErr != nil {
			if !ignoreMissingTools {
				return fmt.Errorf("goimports not found but --use-goimports was specified.\nInstall with: go install golang.org/x/tools/cmd/goimports@latest\nOr run: goneat doctor tools --install goimports\nTip: use --ignore-missing-tools to skip import alignment")
			}
			logger.Debug("goimports not found; skipping import alignment due to --ignore-missing-tools")
		} else {
			cmd := exec.Command("goimports")
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

	if checkOnly {
		if changed {
			return fmt.Errorf("needs formatting")
		}
		return fmt.Errorf("already formatted")
	}

	if changed {
		logger.Info(fmt.Sprintf("Applying Go formatting changes to %s", file))
		return os.WriteFile(file, final, 0644)
	}

	return fmt.Errorf("already formatted")
}

func formatYAMLFile(file string, checkOnly bool, cfg *config.Config) error {
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

		cmd := exec.Command("yamlfmt", args...)
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

	dryCmd := exec.Command("yamlfmt", dryArgs...)
	dryOutput, dryErr := dryCmd.CombinedOutput()

	if dryErr != nil && !strings.Contains(string(dryOutput), "---") {
		// Real error, not just formatting output
		return fmt.Errorf("yamlfmt dry run failed: %v\nOutput: %s", dryErr, string(dryOutput))
	}

	// Check if formatting is needed by comparing dry output
	willChange := len(dryOutput) > 0 && !bytes.Equal(originalContent, dryOutput)

	if !willChange {
		return fmt.Errorf("already formatted")
	}

	// Now apply the formatting
	formatArgs := append(args, file)
	logger.Debug(fmt.Sprintf("Running yamlfmt with args: %v", formatArgs))

	cmd := exec.Command("yamlfmt", formatArgs...)
	output, err := cmd.CombinedOutput()

	if err != nil {
		return fmt.Errorf("yamlfmt failed: %v\nOutput: %s", err, string(output))
	}

	logger.Info(fmt.Sprintf("Applying YAML formatting changes to %s", file))
	return nil
}

func formatJSONFile(file string, checkOnly bool, cfg *config.Config) error {
	jsonConfig := cfg.GetJSONConfig()

	// Read the original content
	originalContent, err := os.ReadFile(file)
	if err != nil {
		return err
	}

	var output []byte
	var cmdErr error

	if jsonConfig.Compact {
		args := []string{"-c", "."}
		cmd := exec.Command("jq", args...)
		cmd.Stdin = strings.NewReader(string(originalContent))
		output, cmdErr = cmd.Output()
		if cmdErr != nil {
			return fmt.Errorf("jq compact failed: %v", cmdErr)
		}
	} else {
		// For pretty printing, use jq with indent to ensure formatting
		args := []string{"--indent", "2", "."}
		cmd := exec.Command("jq", args...)
		cmd.Stdin = strings.NewReader(string(originalContent))
		output, cmdErr = cmd.Output()
		if cmdErr != nil {
			return fmt.Errorf("jq format failed: %v", cmdErr)
		}
	}

	// Handle trailing newline
	formatted := string(output)
	if jsonConfig.TrailingNewline && !strings.HasSuffix(formatted, "\n") {
		formatted += "\n"
	} else if !jsonConfig.TrailingNewline && strings.HasSuffix(formatted, "\n") {
		formatted = strings.TrimSuffix(formatted, "\n")
	}

	// Check if formatting changed the content
	if bytes.Equal(originalContent, []byte(formatted)) {
		if checkOnly {
			return fmt.Errorf("already formatted")
		}
		return fmt.Errorf("already formatted") // Signal that no changes were made
	}

	if checkOnly {
		return fmt.Errorf("needs formatting")
	}

	logger.Info(fmt.Sprintf("Applying JSON formatting changes to %s", file))
	return os.WriteFile(file, []byte(formatted), 0644)
}

func formatMarkdownFile(file string, checkOnly bool, cfg *config.Config) error {
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

	cmd := exec.Command("prettier", args...)
	cmd.Stdin = strings.NewReader(string(originalContent))
	output, cmdErr := cmd.Output()
	if cmdErr != nil {
		return fmt.Errorf("prettier failed: %v", cmdErr)
	}

	// Handle trailing spaces if configured
	formatted := string(output)
	if mdConfig.TrailingSpaces {
		lines := strings.Split(formatted, "\n")
		for i, line := range lines {
			lines[i] = strings.TrimRight(line, " \t")
		}
		formatted = strings.Join(lines, "\n")
	}

	// Check if formatting changed the content
	if bytes.Equal(originalContent, []byte(formatted)) {
		if checkOnly {
			return fmt.Errorf("already formatted")
		}
		return fmt.Errorf("already formatted") // Signal that no changes were made
	}

	if checkOnly {
		return fmt.Errorf("needs formatting")
	}

	logger.Info(fmt.Sprintf("Applying Markdown formatting changes to %s", file))
	return os.WriteFile(file, []byte(formatted), 0644)
}

// executeSequential executes work items sequentially
func executeSequentialWithOptions(files []string, checkOnly, quiet bool, cfg *config.Config, ignoreMissingTools bool, options finalizer.NormalizationOptions, useGoimports bool) error {
	start := time.Now()
	var formattedCount, unchangedCount, errorCount int

	for _, file := range files {
		if err := processFile(file, checkOnly, quiet, cfg, ignoreMissingTools, options, useGoimports); err != nil {
			if err.Error() == "needs formatting" {
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
	dispatcher := work.NewDispatcher(work.DispatcherConfig{
		MaxWorkers: workers,
		DryRun:     false,
		NoOp:       noOp,
		ProgressCallback: func(result work.ExecutionResult) {
			if !quiet {
				if result.Success {
					logger.Info(fmt.Sprintf("Processed %s", result.WorkItemID))
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
	case ".md":
		return "markdown"
	default:
		return "unknown"
	}
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

	return os.WriteFile(filename, data, 0644)
}

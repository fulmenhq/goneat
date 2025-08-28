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
	"time"

	"github.com/3leaps/goneat/pkg/config"
	"github.com/3leaps/goneat/pkg/exitcode"
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
	} else if len(files) > 0 {
		// Extract directories from files
		pathMap := make(map[string]bool)
		for _, file := range files {
			pathMap[filepath.Dir(file)] = true
		}
		for path := range pathMap {
			paths = append(paths, path)
		}
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
	}

	// Create planner and generate manifest
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
	filesToProcess := make([]string, len(manifest.WorkItems))
	for i, item := range manifest.WorkItems {
		filesToProcess[i] = item.Path
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
	if strategy == "parallel" && !dryRun && !planOnly {
		return executeParallel(filesToProcess, cfg, quiet, noOp)
	} else {
		return executeSequential(filesToProcess, checkOnly || noOp, quiet, cfg)
	}
}

func findSupportedFiles(dir string) ([]string, error) {
	var files []string
	supportedExts := []string{".go", ".yaml", ".yml", ".json", ".md"}

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			// Skip common directories
			if info.Name() == ".git" || info.Name() == "vendor" || info.Name() == "node_modules" {
				return filepath.SkipDir
			}
			return nil
		}

		ext := filepath.Ext(path)
		for _, supportedExt := range supportedExts {
			if ext == supportedExt {
				files = append(files, path)
				break
			}
		}
		return nil
	})

	return files, err
}

func processFile(file string, checkOnly, quiet bool, cfg *config.Config) error {
	ext := filepath.Ext(file)

	switch ext {
	case ".go":
		return formatGoFile(file, checkOnly, cfg)
	case ".yaml", ".yml":
		return formatYAMLFile(file, checkOnly, cfg)
	case ".json":
		return formatJSONFile(file, checkOnly, cfg)
	case ".md":
		return formatMarkdownFile(file, checkOnly, cfg)
	default:
		return fmt.Errorf("unsupported file type: %s", ext)
	}
}

func formatGoFile(file string, checkOnly bool, cfg *config.Config) error {
	content, err := os.ReadFile(file)
	if err != nil {
		return err
	}

	// Get Go formatting configuration
	goConfig := cfg.GetGoConfig()

	// Apply gofmt options (currently only simplify is supported by go/format)
	if goConfig.Simplify {
		// Note: go/format doesn't directly support simplify, we'd need go/ast parsing
		// For now, we'll just use standard formatting
		logger.Debug(fmt.Sprintf("Go simplify option enabled for %s", file))
	}

	formatted, err := format.Source(content)
	if err != nil {
		return err
	}

	if bytes.Equal(content, formatted) {
		return nil // Already formatted
	}

	if checkOnly {
		return fmt.Errorf("needs formatting")
	}

	// Log what we're doing - this demonstrates information passthrough
	logger.Info(fmt.Sprintf("Applying Go formatting changes to %s", file))

	return os.WriteFile(file, formatted, 0644)
}

func formatYAMLFile(file string, checkOnly bool, cfg *config.Config) error {
	yamlConfig := cfg.GetYAMLConfig()

	// Build yamlfmt arguments
	args := []string{"-w", file}

	// Add configuration options
	if yamlConfig.Indent != 2 {
		args = append(args, fmt.Sprintf("-indent=%d", yamlConfig.Indent))
	}
	if yamlConfig.LineLength != 80 {
		args = append(args, fmt.Sprintf("-width=%d", yamlConfig.LineLength))
	}
	if yamlConfig.QuoteStyle == "single" {
		args = append(args, "-quote")
	}
	if !yamlConfig.TrailingNewline {
		args = append(args, "-no_trailing_newline")
	}

	logger.Debug(fmt.Sprintf("Running yamlfmt with args: %v", args))

	// Execute yamlfmt
	cmd := exec.Command("yamlfmt", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("yamlfmt failed: %v\nOutput: %s", err, string(output))
	}

	if len(output) > 0 {
		logger.Debug(fmt.Sprintf("yamlfmt output: %s", string(output)))
	}

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
		return nil // Already formatted
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
		return nil // Already formatted
	}

	if checkOnly {
		return fmt.Errorf("needs formatting")
	}

	logger.Info(fmt.Sprintf("Applying Markdown formatting changes to %s", file))
	return os.WriteFile(file, []byte(formatted), 0644)
}

// executeSequential executes work items sequentially
func executeSequential(files []string, checkOnly, quiet bool, cfg *config.Config) error {
	var formattedCount, errorCount int

	for _, file := range files {
		if err := processFile(file, checkOnly, quiet, cfg); err != nil {
			logger.Error(fmt.Sprintf("Failed to process %s", file), logger.Err(err))
			errorCount++
		} else {
			formattedCount++
			if !quiet && !checkOnly {
				logger.Info(fmt.Sprintf("Formatted %s", file))
			}
		}
	}

	if !quiet {
		if checkOnly {
			if errorCount > 0 {
				logger.Warn(fmt.Sprintf("Found %d files that need formatting", errorCount))
			} else {
				logger.Info("All files are properly formatted")
			}
		} else {
			logger.Info(fmt.Sprintf("Processed %d files (%d formatted, %d errors)", len(files), formattedCount, errorCount))
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
	processor := work.NewFormatProcessor(cfg)
	dispatcher := work.NewDispatcher(work.DispatcherConfig{
		MaxWorkers: runtime.NumCPU(),
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
		logger.Info(fmt.Sprintf("Parallel execution completed: %d successful, %d failed in %v",
			summary.Successful, summary.Failed, summary.TotalDuration))
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

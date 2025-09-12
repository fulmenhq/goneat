/*
Copyright © 2025 3 Leaps <info@3leaps.net>
*/
package cmd

import (
	"bufio"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/fulmenhq/goneat/internal/assets"
	"github.com/fulmenhq/goneat/pkg/logger"
	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize goneat configuration with language-aware .goneatignore",
	Long: `Initialize creates a .goneatignore file with patterns appropriate for your project's languages.
It automatically detects languages in your repository and generates relevant ignore patterns.

DESIGN PHILOSOPHY:
The .goneatignore file is comprehensive and independent of .gitignore. This ensures goneat
works reliably regardless of .gitignore configuration. While there may be some overlap with
typical .gitignore patterns, this approach prioritizes user experience and safety.

KEY BEHAVIORS:
• .goneatignore should be COMMITTED to git (not gitignored)
• Applies ONLY to goneat commands (assess, format, etc.)
• RESPECTS .gitignore - goneat will still process files that are gitignored
• Can FORCE inclusion of files by adding negative patterns (e.g., "!important-file.txt")
• Independent of .gitignore - works even if .gitignore doesn't exist

RECOMMENDED USAGE:
• Run 'goneat init' immediately after installing goneat
• Use --force to update patterns when adding new languages to your project
• Use --merge to preserve custom patterns when updating
• Review generated patterns and customize as needed for your workflow

Examples:
  goneat init                    # Auto-detect languages and create .goneatignore
  goneat init --force           # Replace existing .goneatignore
  goneat init --merge           # Merge with existing .goneatignore
  goneat init --languages go,rust  # Explicitly specify languages
  goneat init --add-languages python  # Add Python patterns to existing file
  goneat init --dry-run         # Show what would be generated without writing`,
	RunE: runInit,
}

var (
	initLanguages         []string
	initAddLanguages      []string
	initForce             bool
	initMerge             bool
	initDryRun            bool
	initInteractive       bool
	initNonInteractive    bool
	initOutput            string
	initQuiet             bool
	initVerbose           bool
	initUniversalOnly     bool
	initExcludeCategories []string
	initIncludePatterns   []string
)

func init() {
	initCmd.Flags().StringSliceVar(&initLanguages, "languages", nil, "Explicitly specify languages (comma-separated)")
	initCmd.Flags().StringSliceVar(&initAddLanguages, "add-languages", nil, "Add language patterns to existing .goneatignore")
	initCmd.Flags().BoolVar(&initForce, "force", false, "Force replace existing .goneatignore")
	initCmd.Flags().BoolVar(&initMerge, "merge", false, "Merge with existing .goneatignore")
	initCmd.Flags().BoolVar(&initDryRun, "dry-run", false, "Show what would be generated without writing")
	initCmd.Flags().BoolVar(&initInteractive, "interactive", false, "Interactive mode for customization")
	initCmd.Flags().BoolVar(&initNonInteractive, "non-interactive", false, "Non-interactive mode for CI/CD")
	initCmd.Flags().StringVar(&initOutput, "output", ".goneatignore", "Output file path")
	initCmd.Flags().BoolVar(&initQuiet, "quiet", false, "Quiet mode - minimal output")
	initCmd.Flags().BoolVar(&initVerbose, "verbose", false, "Verbose output")
	initCmd.Flags().BoolVar(&initUniversalOnly, "universal-only", false, "Include only universal patterns")
	initCmd.Flags().StringSliceVar(&initExcludeCategories, "exclude-categories", nil, "Exclude pattern categories (build,logs,temp)")
	initCmd.Flags().StringSliceVar(&initIncludePatterns, "include-patterns", nil, "Additional custom patterns to include")

	rootCmd.AddCommand(initCmd)
}

func runInit(cmd *cobra.Command, args []string) error {
	targetDir := "."
	if len(args) > 0 {
		targetDir = args[0]
	}

	// Determine languages to include
	var languages []string
	var err error

	if len(initAddLanguages) > 0 {
		// Adding to existing file
		languages = initAddLanguages
	} else if len(initLanguages) > 0 {
		// Explicitly specified
		languages = initLanguages
	} else if !initUniversalOnly {
		// Auto-detect
		languages, err = detectLanguages(targetDir)
		if err != nil {
			return fmt.Errorf("failed to detect languages: %w", err)
		}
	}

	// Generate patterns
	patterns, err := generatePatterns(languages)
	if err != nil {
		return fmt.Errorf("failed to generate patterns: %w", err)
	}

	// Add custom patterns
	patterns = append(patterns, initIncludePatterns...)

	// Handle existing file
	// Validate output path to prevent directory traversal
	outputPath := filepath.Join(targetDir, initOutput)
	if strings.Contains(outputPath, "..") {
		return fmt.Errorf("output path cannot contain directory traversal: %s", initOutput)
	}
	outputPath = filepath.Clean(outputPath)
	existingContent := ""
	fileExists := false

	if info, err := os.Stat(outputPath); err == nil && !info.IsDir() {
		fileExists = true
		if initMerge || (!initForce && !initDryRun) {
			content, err := os.ReadFile(outputPath)
			if err != nil {
				return fmt.Errorf("failed to read existing file: %w", err)
			}
			existingContent = string(content)
		}
	}

	// Interactive mode
	if initInteractive && !initNonInteractive && !initDryRun {
		patterns, err = interactiveCustomization(patterns, languages)
		if err != nil {
			return fmt.Errorf("interactive customization failed: %w", err)
		}
	}

	// Merge or replace logic
	finalContent := ""
	if initMerge && fileExists {
		finalContent = mergePatterns(existingContent, patterns)
	} else {
		finalContent = generateFileContent(patterns, languages)
	}

	// Dry run
	if initDryRun {
		fmt.Println("=== DRY RUN ===")
		fmt.Println("Would generate .goneatignore with the following content:")
		fmt.Println(finalContent)
		return nil
	}

	// Safety check for existing file
	if fileExists && !initForce && !initMerge {
		if initNonInteractive {
			return fmt.Errorf(".goneatignore already exists. Use --force to replace or --merge to merge")
		}

		fmt.Printf("⚠️  %s already exists\n", initOutput)
		fmt.Print("❓ Replace existing file? (y/N): ")

		reader := bufio.NewReader(os.Stdin)
		response, err := reader.ReadString('\n')
		if err != nil {
			return fmt.Errorf("failed to read input: %w", err)
		}

		response = strings.ToLower(strings.TrimSpace(response))
		if response != "y" && response != "yes" {
			fmt.Print("❓ Merge with existing file? (y/N): ")
			response, err := reader.ReadString('\n')
			if err != nil {
				return fmt.Errorf("failed to read input: %w", err)
			}

			response = strings.ToLower(strings.TrimSpace(response))
			if response == "y" || response == "yes" {
				initMerge = true
				finalContent = mergePatterns(existingContent, patterns)
			} else {
				fmt.Println("Operation cancelled.")
				return nil
			}
		}
	}

	// Write file with secure permissions
	err = os.WriteFile(outputPath, []byte(finalContent), 0600)
	if err != nil {
		return fmt.Errorf("failed to write %s: %w", initOutput, err)
	}

	// Output
	if !initQuiet {
		if len(initAddLanguages) > 0 {
			fmt.Printf("✅ Added %s patterns to %s\n", strings.Join(languages, ", "), initOutput)
		} else if initMerge {
			fmt.Printf("✅ Merged patterns into existing %s\n", initOutput)
		} else {
			fmt.Printf("✅ Generated %s with %d patterns\n", initOutput, len(patterns))
			if len(languages) > 0 {
				fmt.Printf("   Languages: %s\n", strings.Join(languages, ", "))
			}
		}

		if initVerbose {
			fmt.Println("\nGenerated patterns:")
			for _, pattern := range patterns {
				fmt.Printf("   - %s\n", pattern)
			}
		}
	}

	return nil
}

func detectLanguages(targetDir string) ([]string, error) {
	languages := []string{}

	// Check for language indicators
	checks := map[string][]string{
		"go":         {"go.mod", "*.go"},
		"typescript": {"package.json", "*.ts", "*.tsx", "*.js", "*.jsx"},
		"python":     {"requirements.txt", "setup.py", "pyproject.toml", "*.py"},
		"rust":       {"Cargo.toml", "*.rs"},
	}

	for lang, indicators := range checks {
		for _, indicator := range indicators {
			if strings.Contains(indicator, "*") {
				// Glob pattern
				matches, err := filepath.Glob(filepath.Join(targetDir, indicator))
				if err == nil && len(matches) > 0 {
					languages = append(languages, lang)
					break
				}
			} else {
				// File check
				if _, err := os.Stat(filepath.Join(targetDir, indicator)); err == nil {
					languages = append(languages, lang)
					break
				}
			}
		}
	}

	// Remove duplicates
	seen := make(map[string]bool)
	unique := []string{}
	for _, lang := range languages {
		if !seen[lang] {
			seen[lang] = true
			unique = append(unique, lang)
		}
	}

	return unique, nil
}

func generatePatterns(languages []string) ([]string, error) {
	patterns := []string{}

	// Always include universal patterns
	templatesFS := assets.GetTemplatesFS()
	universalContent, err := fs.ReadFile(templatesFS, "goneatignore/universal.txt")
	if err != nil {
		return nil, fmt.Errorf("failed to load universal template: %w", err)
	}

	universalPatterns := strings.Split(strings.TrimSpace(string(universalContent)), "\n")
	patterns = append(patterns, universalPatterns...)

	// Add language-specific patterns
	for _, lang := range languages {
		templatePath := fmt.Sprintf("goneatignore/%s.txt", lang)
		templatesFS := assets.GetTemplatesFS()
		content, err := fs.ReadFile(templatesFS, templatePath)
		if err != nil {
			logger.Warn(fmt.Sprintf("Template not found for language %s: %v", lang, err))
			continue
		}

		langPatterns := strings.Split(strings.TrimSpace(string(content)), "\n")
		patterns = append(patterns, langPatterns...)
	}

	// Remove duplicates and empty lines
	seen := make(map[string]bool)
	unique := []string{}
	for _, pattern := range patterns {
		pattern = strings.TrimSpace(pattern)
		if pattern != "" && !seen[pattern] && !strings.HasPrefix(pattern, "#") {
			seen[pattern] = true
			unique = append(unique, pattern)
		}
	}

	sort.Strings(unique)
	return unique, nil
}

func generateFileContent(patterns []string, languages []string) string {
	var builder strings.Builder

	// Header
	builder.WriteString("# .goneatignore\n")
	builder.WriteString("# Generated by goneat init\n")
	if len(languages) > 0 {
		builder.WriteString(fmt.Sprintf("# Languages: %s\n", strings.Join(languages, ", ")))
	}
	builder.WriteString("\n")

	// Patterns
	for _, pattern := range patterns {
		builder.WriteString(pattern)
		builder.WriteString("\n")
	}

	return builder.String()
}

func mergePatterns(existingContent string, newPatterns []string) string {
	existingLines := strings.Split(existingContent, "\n")
	existingPatterns := make(map[string]bool)

	// Extract existing patterns (skip comments and empty lines)
	for _, line := range existingLines {
		line = strings.TrimSpace(line)
		if line != "" && !strings.HasPrefix(line, "#") {
			existingPatterns[line] = true
		}
	}

	// Add new patterns that don't exist
	merged := existingLines
	added := false

	for _, pattern := range newPatterns {
		if !existingPatterns[pattern] {
			if !added {
				merged = append(merged, "")
				merged = append(merged, "# Added by goneat init")
				added = true
			}
			merged = append(merged, pattern)
		}
	}

	return strings.Join(merged, "\n")
}

func interactiveCustomization(patterns []string, languages []string) ([]string, error) {
	fmt.Println("=== Interactive Customization ===")
	fmt.Printf("Detected languages: %s\n", strings.Join(languages, ", "))
	fmt.Printf("Generated %d patterns\n", len(patterns))

	reader := bufio.NewReader(os.Stdin)

	// Ask about additional patterns
	fmt.Print("❓ Add custom patterns? (y/N): ")
	response, err := reader.ReadString('\n')
	if err != nil {
		return patterns, err
	}

	response = strings.ToLower(strings.TrimSpace(response))
	if response == "y" || response == "yes" {
		fmt.Println("Enter patterns (one per line, empty line to finish):")
		for {
			fmt.Print("> ")
			pattern, err := reader.ReadString('\n')
			if err != nil {
				return patterns, err
			}

			pattern = strings.TrimSpace(pattern)
			if pattern == "" {
				break
			}

			patterns = append(patterns, pattern)
		}
	}

	return patterns, nil
}

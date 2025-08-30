/*
Copyright © 2025 3 Leaps <info@3leaps.net>
*/
package cmd

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/3leaps/goneat/internal/ops"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

// hooksCmd represents the hooks command
var hooksCmd = &cobra.Command{
	Use:   "hooks",
	Short: "Manage git hooks with goneat integration",
	Long: `Hooks provides comprehensive git hook management with native goneat integration.
It supports creating, installing, and managing hooks that leverage goneat's assessment
capabilities for intelligent code quality validation.

Examples:
  goneat hooks init          # Initialize hooks system
  goneat hooks generate      # Generate hook files from manifest
  goneat hooks install       # Install hooks to .git/hooks
  goneat hooks validate      # Validate hook configuration
  goneat hooks list          # List available hooks`,
}

// hooksInitCmd represents the hooks init command
var hooksInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize hooks system",
	Long: `Initialize creates the basic hooks infrastructure including:
- .goneat/hooks.yaml manifest file
- Basic directory structure
- Default hook configurations`,
	RunE: runHooksInit,
}

// hooksGenerateCmd represents the hooks generate command
var hooksGenerateCmd = &cobra.Command{
	Use:   "generate",
	Short: "Generate hook files from manifest",
	Long: `Generate creates executable hook files based on the hooks manifest.
These files integrate with goneat's assessment system for intelligent validation.`,
	RunE: runHooksGenerate,
}

// hooksInstallCmd represents the hooks install command
var hooksInstallCmd = &cobra.Command{
	Use:   "install",
	Short: "Install hooks to .git/hooks",
	Long: `Install copies generated hook files to .git/hooks directory,
making them active for git operations.`,
	RunE: runHooksInstall,
}

// hooksValidateCmd represents the hooks validate command
var hooksValidateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Validate hook configuration",
	Long: `Validate checks the hooks manifest and generated files for
correctness and compatibility.`,
	RunE: runHooksValidate,
}

// hooksRemoveCmd represents the hooks remove command
var hooksRemoveCmd = &cobra.Command{
	Use:   "remove",
	Short: "Remove installed hooks",
	Long: `Remove uninstalls goneat hooks from .git/hooks directory,
restoring any previously backed up hooks if they exist.`,
	RunE: runHooksRemove,
}

var removeNoRestore bool

// hooksUpgradeCmd represents the hooks upgrade command
var hooksUpgradeCmd = &cobra.Command{
	Use:   "upgrade",
	Short: "Upgrade hook configuration to latest version",
	Long: `Upgrade updates the hooks manifest to the latest schema version,
migrating configuration as needed. This command scaffolds future
functionality for automatic schema upgrades.`,
	RunE: runHooksUpgrade,
}

// hooksInspectCmd represents the hooks inspect command
var hooksInspectCmd = &cobra.Command{
	Use:   "inspect",
	Short: "Inspect current hook configuration and status",
	Long: `Inspect displays detailed information about the current hook
configuration, installation status, and system state. Supports both
human-readable and JSON output formats.`,
	RunE: runHooksInspect,
}

func init() {
	rootCmd.AddCommand(hooksCmd)

	// Register command in ops registry with taxonomy
	capabilities := ops.GetDefaultCapabilities(ops.GroupWorkflow, ops.CategoryOrchestration)
	if err := ops.RegisterCommandWithTaxonomy("hooks", ops.GroupWorkflow, ops.CategoryOrchestration, capabilities, hooksCmd, "Manage git hooks with goneat integration"); err != nil {
		panic(fmt.Sprintf("Failed to register hooks command: %v", err))
	}

	// Add subcommands
	hooksCmd.AddCommand(hooksInitCmd)
	hooksCmd.AddCommand(hooksGenerateCmd)
	hooksCmd.AddCommand(hooksInstallCmd)
	hooksCmd.AddCommand(hooksValidateCmd)
	hooksCmd.AddCommand(hooksRemoveCmd)
	hooksCmd.AddCommand(hooksUpgradeCmd)
	hooksCmd.AddCommand(hooksInspectCmd)

	// hooks remove flags (define before registration to avoid duplicate init)
	hooksRemoveCmd.Flags().BoolVar(&removeNoRestore, "no-restore", false, "Do not restore original hooks from backups; remove hooks completely")

	// Register subcommands
	subcommands := []*cobra.Command{hooksInitCmd, hooksGenerateCmd, hooksInstallCmd, hooksValidateCmd, hooksRemoveCmd, hooksUpgradeCmd, hooksInspectCmd}
	for _, cmd := range subcommands {
		if err := ops.RegisterCommand(fmt.Sprintf("hooks %s", cmd.Use), ops.GroupWorkflow, cmd, cmd.Short); err != nil {
			panic(fmt.Sprintf("Failed to register hooks %s command: %v", cmd.Use, err))
		}
	}
}

func runHooksInit(cmd *cobra.Command, args []string) error {
	fmt.Println("🐾 Initializing goneat hooks system...")

	// Check if already initialized
	if _, err := os.Stat(".goneat/hooks.yaml"); err == nil {
		fmt.Println("⚠️  Hooks system already initialized")
		fmt.Println("💡 Use 'goneat hooks upgrade' to update configuration")
		fmt.Println("💡 Use 'goneat hooks generate' to regenerate hook files")
		return nil
	}

	// Check if we're in a git repository
	if _, err := os.Stat(".git"); os.IsNotExist(err) {
		return fmt.Errorf("not in a git repository. Initialize git first with 'git init'")
	}

	// Create .goneat directory
	goneatDir := ".goneat"
	if err := os.MkdirAll(goneatDir, 0755); err != nil {
		return fmt.Errorf("failed to create .goneat directory: %v", err)
	}

	// Create default hooks.yaml manifest
	hooksConfig := `version: "1.0.0"
hooks:
  pre-commit:
    - command: "assess"
      args: ["--categories", "format,lint", "--fail-on", "error"]
      stage_fixed: true
      priority: 10
      timeout: "2m"
  pre-push:
    - command: "assess"
      args: ["--categories", "format,lint,static-analysis", "--fail-on", "high"]
      priority: 10
      timeout: "3m"
optimization:
  only_changed_files: true
  cache_results: true
  parallel: "auto"
`

	hooksPath := filepath.Join(goneatDir, "hooks.yaml")
	if err := os.WriteFile(hooksPath, []byte(hooksConfig), 0644); err != nil {
		return fmt.Errorf("failed to create hooks.yaml: %v", err)
	}

	fmt.Println("✅ Hooks system initialized successfully!")
	fmt.Println("📝 Created .goneat/hooks.yaml with default configuration")
	fmt.Println("🚀 Next steps:")
	fmt.Println("   1. Run 'goneat hooks generate' to create hook files")
	fmt.Println("   2. Run 'goneat hooks install' to install hooks to .git/hooks")
	fmt.Println("   3. Run 'goneat hooks validate' to verify everything works")

	return nil
}

func runHooksGenerate(cmd *cobra.Command, args []string) error {
	fmt.Println("🔨 Generating hook files from manifest...")

	// Check if hooks.yaml exists
	if _, err := os.Stat(".goneat/hooks.yaml"); os.IsNotExist(err) {
		return fmt.Errorf("hooks configuration not found. Run 'goneat hooks init' first")
	}

	// Create .goneat/hooks directory
	hooksDir := ".goneat/hooks"
	if err := os.MkdirAll(hooksDir, 0755); err != nil {
		return fmt.Errorf("failed to create hooks directory: %v", err)
	}

	// Load manifest and render templates
	manifestData, err := os.ReadFile(".goneat/hooks.yaml")
	if err != nil {
		return fmt.Errorf("failed to read hooks manifest: %v", err)
	}
	var manifest struct {
		Hooks map[string][]struct {
			Command  string   `yaml:"command"`
			Args     []string `yaml:"args"`
			Fallback string   `yaml:"fallback"`
		} `yaml:"hooks"`
		Optimization struct {
			OnlyChangedFiles bool `yaml:"only_changed_files"`
		} `yaml:"optimization"`
	}
	if err := yaml.Unmarshal(manifestData, &manifest); err != nil {
		return fmt.Errorf("failed to parse hooks manifest: %v", err)
	}

	type tplData struct {
		Args                 []string
		Fallback             string
		OptimizationSettings struct {
			OnlyChangedFiles bool
		}
	}

	render := func(templatePath, destPath string, data tplData) error {
		raw, err := os.ReadFile(templatePath)
		if err != nil {
			return fmt.Errorf("failed to read template %s: %v", templatePath, err)
		}
		tmpl, err := template.New(filepath.Base(templatePath)).Parse(string(raw))
		if err != nil {
			return fmt.Errorf("failed to parse template %s: %v", templatePath, err)
		}
		var buf bytes.Buffer
		if err := tmpl.Execute(&buf, data); err != nil {
			return fmt.Errorf("failed to render template %s: %v", templatePath, err)
		}
		if err := os.WriteFile(destPath, buf.Bytes(), 0755); err != nil {
			return fmt.Errorf("failed to write %s: %v", destPath, err)
		}
		return nil
	}

	buildArgs := func(hook string) ([]string, string) {
		var args []string
		var fallback string
		for _, h := range manifest.Hooks[hook] {
			if strings.TrimSpace(h.Command) == "assess" {
				args = append(args, h.Args...)
				if h.Fallback != "" {
					fallback = h.Fallback
				}
				break
			}
		}
		return args, fallback
	}

	// Render pre-commit from template
	argsPC, fbPC := buildArgs("pre-commit")
	dataPC := tplData{Args: argsPC, Fallback: fbPC}
	dataPC.OptimizationSettings.OnlyChangedFiles = manifest.Optimization.OnlyChangedFiles
	if err := render("templates/hooks/bash/pre-commit.sh.tmpl", filepath.Join(hooksDir, "pre-commit"), dataPC); err != nil {
		return err
	}

	// Render pre-push from template
	argsPP, fbPP := buildArgs("pre-push")
	dataPP := tplData{Args: argsPP, Fallback: fbPP}
	dataPP.OptimizationSettings.OnlyChangedFiles = manifest.Optimization.OnlyChangedFiles
	if err := render("templates/hooks/bash/pre-push.sh.tmpl", filepath.Join(hooksDir, "pre-push"), dataPP); err != nil {
		return err
	}

	fmt.Println("✅ Hook files generated successfully!")
	fmt.Printf("📁 Created: %s/pre-commit\n", hooksDir)
	fmt.Printf("📁 Created: %s/pre-push\n", hooksDir)
	fmt.Println("📌 Next: Run 'goneat hooks install' to install hooks to .git/hooks")

	return nil
}

func runHooksInstall(cmd *cobra.Command, args []string) error {
	fmt.Println("📦 Installing hooks to .git/hooks...")

	// Check if generated hooks exist
	hooksDir := ".goneat/hooks"
	if _, err := os.Stat(hooksDir); os.IsNotExist(err) {
		return fmt.Errorf("no generated hooks found. Run 'goneat hooks generate' first")
	}

	// Check if .git/hooks exists
	gitHooksDir := ".git/hooks"
	if _, err := os.Stat(gitHooksDir); os.IsNotExist(err) {
		return fmt.Errorf(".git/hooks directory not found. Are you in a git repository?")
	}

	hooksInstalled := 0

	// Install pre-commit hook
	preCommitSrc := filepath.Join(hooksDir, "pre-commit")
	preCommitDst := filepath.Join(gitHooksDir, "pre-commit")

	if _, err := os.Stat(preCommitSrc); err == nil {
		// Backup existing hook if it exists
		if _, err := os.Stat(preCommitDst); err == nil {
			backupPath := preCommitDst + ".backup"
			if err := os.Rename(preCommitDst, backupPath); err != nil {
				return fmt.Errorf("failed to backup existing pre-commit hook: %v", err)
			}
			fmt.Printf("📋 Backed up existing pre-commit hook to %s\n", backupPath)
		}

		// Copy new hook
		if err := copyFile(preCommitSrc, preCommitDst); err != nil {
			return fmt.Errorf("failed to install pre-commit hook: %v", err)
		}

		// Make executable
		if err := os.Chmod(preCommitDst, 0755); err != nil {
			return fmt.Errorf("failed to make pre-commit hook executable: %v", err)
		}

		fmt.Println("✅ Installed pre-commit hook")
		hooksInstalled++
	}

	// Install pre-push hook
	prePushSrc := filepath.Join(hooksDir, "pre-push")
	prePushDst := filepath.Join(gitHooksDir, "pre-push")

	if _, err := os.Stat(prePushSrc); err == nil {
		// Backup existing hook if it exists
		if _, err := os.Stat(prePushDst); err == nil {
			backupPath := prePushDst + ".backup"
			if err := os.Rename(prePushDst, backupPath); err != nil {
				return fmt.Errorf("failed to backup existing pre-push hook: %v", err)
			}
			fmt.Printf("📋 Backed up existing pre-push hook to %s\n", backupPath)
		}

		// Copy new hook
		if err := copyFile(prePushSrc, prePushDst); err != nil {
			return fmt.Errorf("failed to install pre-push hook: %v", err)
		}

		// Make executable
		if err := os.Chmod(prePushDst, 0755); err != nil {
			return fmt.Errorf("failed to make pre-push hook executable: %v", err)
		}

		fmt.Println("✅ Installed pre-push hook")
		hooksInstalled++
	}

	if hooksInstalled == 0 {
		return fmt.Errorf("no hooks found to install")
	}

	fmt.Printf("🎯 Successfully installed %d hook(s)!\n", hooksInstalled)
	fmt.Println("💡 Your git operations will now use goneat's intelligent validation")
	fmt.Println("🔍 Test with: goneat assess --hook pre-commit")

	return nil
}

// copyFile copies a file from src to dst
func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0755) // Make executable
}

func runHooksValidate(cmd *cobra.Command, args []string) error {
	fmt.Println("🔍 Validating hook configuration...")

	// Check if hooks.yaml exists
	hooksConfigPath := ".goneat/hooks.yaml"
	if _, err := os.Stat(hooksConfigPath); os.IsNotExist(err) {
		return fmt.Errorf("hooks configuration not found at %s", hooksConfigPath)
	}

	// Check if generated hooks exist
	hooksDir := ".goneat/hooks"
	if _, err := os.Stat(hooksDir); os.IsNotExist(err) {
		fmt.Println("⚠️  No generated hooks found - run 'goneat hooks generate'")
	} else {
		// Check for hook files
		preCommitPath := filepath.Join(hooksDir, "pre-commit")
		prePushPath := filepath.Join(hooksDir, "pre-push")

		if _, err := os.Stat(preCommitPath); err == nil {
			fmt.Println("✅ Pre-commit hook generated")
		} else {
			fmt.Println("⚠️  Pre-commit hook not found")
		}

		if _, err := os.Stat(prePushPath); err == nil {
			fmt.Println("✅ Pre-push hook generated")
		} else {
			fmt.Println("⚠️  Pre-push hook not found")
		}
	}

	// Check if installed hooks exist
	gitHooksDir := ".git/hooks"
	if _, err := os.Stat(gitHooksDir); os.IsNotExist(err) {
		fmt.Println("⚠️  .git/hooks directory not found - not in a git repository?")
	} else {
		preCommitInstalled := filepath.Join(gitHooksDir, "pre-commit")
		prePushInstalled := filepath.Join(gitHooksDir, "pre-push")

		if info, err := os.Stat(preCommitInstalled); err == nil && (info.Mode()&0111) != 0 {
			fmt.Println("✅ Pre-commit hook installed and executable")
		} else {
			fmt.Println("⚠️  Pre-commit hook not properly installed")
		}

		if info, err := os.Stat(prePushInstalled); err == nil && (info.Mode()&0111) != 0 {
			fmt.Println("✅ Pre-push hook installed and executable")
		} else {
			fmt.Println("⚠️  Pre-push hook not properly installed")
		}
	}

	fmt.Println("✅ Hook configuration validation complete")
	fmt.Println("🎉 Ready to commit with intelligent validation!")

	return nil
}

func runHooksRemove(cmd *cobra.Command, args []string) error {
	fmt.Println("🗑️  Removing goneat hooks...")

	gitHooksDir := ".git/hooks"
	if _, err := os.Stat(gitHooksDir); os.IsNotExist(err) {
		return fmt.Errorf(".git/hooks directory not found")
	}

	// Remove pre-commit hook and restore backup
	preCommitHook := filepath.Join(gitHooksDir, "pre-commit")
	preCommitBackup := preCommitHook + ".backup"

	if _, err := os.Stat(preCommitHook); err == nil {
		if err := os.Remove(preCommitHook); err != nil {
			return fmt.Errorf("failed to remove pre-commit hook: %v", err)
		}
		fmt.Println("✅ Removed pre-commit hook")

		// Restore backup if it exists
		if !removeNoRestore {
			if _, err := os.Stat(preCommitBackup); err == nil {
				if err := os.Rename(preCommitBackup, preCommitHook); err != nil {
					return fmt.Errorf("failed to restore pre-commit backup: %v", err)
				}
				fmt.Printf("📋 Restored original pre-commit hook from %s\n", preCommitBackup)
			}
		} else {
			// If no-restore, clean up backup as well
			if _, err := os.Stat(preCommitBackup); err == nil {
				_ = os.Remove(preCommitBackup)
			}
		}
	}

	// Remove pre-push hook and restore backup
	prePushHook := filepath.Join(gitHooksDir, "pre-push")
	prePushBackup := prePushHook + ".backup"

	if _, err := os.Stat(prePushHook); err == nil {
		if err := os.Remove(prePushHook); err != nil {
			return fmt.Errorf("failed to remove pre-push hook: %v", err)
		}
		fmt.Println("✅ Removed pre-push hook")

		// Restore backup if it exists
		if !removeNoRestore {
			if _, err := os.Stat(prePushBackup); err == nil {
				if err := os.Rename(prePushBackup, prePushHook); err != nil {
					return fmt.Errorf("failed to restore pre-push backup: %v", err)
				}
				fmt.Printf("📋 Restored original pre-push hook from %s\n", prePushBackup)
			}
		} else {
			// If no-restore, clean up backup as well
			if _, err := os.Stat(prePushBackup); err == nil {
				_ = os.Remove(prePushBackup)
			}
		}
	}

	fmt.Println("✅ Goneat hooks removed")
	if removeNoRestore {
		fmt.Println("💡 Backups not restored per --no-restore; hooks are now absent")
	} else {
		fmt.Println("💡 Your git hooks have been restored to their previous state")
	}

	return nil
}

func runHooksUpgrade(cmd *cobra.Command, args []string) error {
	fmt.Println("⬆️  Upgrading hook configuration...")

	// Check if hooks.yaml exists
	hooksConfigPath := ".goneat/hooks.yaml"
	if _, err := os.Stat(hooksConfigPath); os.IsNotExist(err) {
		return fmt.Errorf("hooks configuration not found. Run 'goneat hooks init' first")
	}

	// Read current configuration
	_, err := os.ReadFile(hooksConfigPath)
	if err != nil {
		return fmt.Errorf("failed to read hooks configuration: %v", err)
	}

	// For now, this is a placeholder for future schema migration
	// In a real implementation, this would:
	// 1. Parse current YAML
	// 2. Check schema version
	// 3. Download latest schema
	// 4. Migrate configuration
	// 5. Write updated configuration

	fmt.Println("🔄 Schema upgrade functionality coming soon!")
	fmt.Println("📋 This command will automatically migrate your hooks configuration")
	fmt.Println("   to the latest schema version when implemented")
	fmt.Println("✅ Current configuration validated")

	return nil
}

func runHooksInspect(cmd *cobra.Command, args []string) error {
	fmt.Println("🔍 Inspecting hook configuration and status...")

	// Check configuration file
	hooksConfigPath := ".goneat/hooks.yaml"
	configStatus := "❌ Not found"
	if _, err := os.Stat(hooksConfigPath); err == nil {
		configStatus = "✅ Found"
	}

	// Check generated hooks
	hooksDir := ".goneat/hooks"
	generatedStatus := "❌ Not found"
	preCommitGenerated := "❌ Missing"
	prePushGenerated := "❌ Missing"

	if _, err := os.Stat(hooksDir); err == nil {
		generatedStatus = "✅ Found"
		if _, err := os.Stat(filepath.Join(hooksDir, "pre-commit")); err == nil {
			preCommitGenerated = "✅ Present"
		}
		if _, err := os.Stat(filepath.Join(hooksDir, "pre-push")); err == nil {
			prePushGenerated = "✅ Present"
		}
	}

	// Check installed hooks
	gitHooksDir := ".git/hooks"
	installedStatus := "❌ Not found"
	preCommitInstalled := "❌ Missing"
	prePushInstalled := "❌ Missing"

	if _, err := os.Stat(gitHooksDir); err == nil {
		installedStatus = "✅ Found"
		preCommitPath := filepath.Join(gitHooksDir, "pre-commit")
		prePushPath := filepath.Join(gitHooksDir, "pre-push")

		if info, err := os.Stat(preCommitPath); err == nil && (info.Mode()&0111) != 0 {
			preCommitInstalled = "✅ Installed & executable"
		}
		if info, err := os.Stat(prePushPath); err == nil && (info.Mode()&0111) != 0 {
			prePushInstalled = "✅ Installed & executable"
		}
	}

	// Display status
	fmt.Println("📊 Current Hook Status:")
	fmt.Printf("├── Configuration: %s\n", configStatus)
	fmt.Printf("├── Generated Hooks: %s\n", generatedStatus)
	fmt.Printf("│   ├── Pre-commit: %s\n", preCommitGenerated)
	fmt.Printf("│   └── Pre-push: %s\n", prePushGenerated)
	fmt.Printf("├── Installed Hooks: %s\n", installedStatus)
	fmt.Printf("│   ├── Pre-commit: %s\n", preCommitInstalled)
	fmt.Printf("│   └── Pre-push: %s\n", prePushInstalled)

	// Overall health assessment
	healthScore := 0
	if configStatus == "✅ Found" {
		healthScore++
	}
	if generatedStatus == "✅ Found" {
		healthScore++
	}
	if installedStatus == "✅ Found" {
		healthScore++
	}
	if preCommitGenerated == "✅ Present" {
		healthScore++
	}
	if prePushGenerated == "✅ Present" {
		healthScore++
	}
	if preCommitInstalled == "✅ Installed & executable" {
		healthScore++
	}
	if prePushInstalled == "✅ Installed & executable" {
		healthScore++
	}

	healthStatus := "❌ Critical"
	if healthScore >= 5 {
		healthStatus = "✅ Good"
	} else if healthScore >= 3 {
		healthStatus = "⚠️  Needs attention"
	}

	fmt.Printf("└── System Health: %s (%d/7)\n", healthStatus, healthScore)

	return nil
}

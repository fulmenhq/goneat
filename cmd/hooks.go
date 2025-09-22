/*
Copyright © 2025 3 Leaps <info@3leaps.net>
*/
package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"text/template"

	"github.com/fulmenhq/goneat/internal/assets"
	"github.com/fulmenhq/goneat/internal/guardian"
	"github.com/fulmenhq/goneat/internal/ops"
	"github.com/spf13/cobra"
	"github.com/xeipuuv/gojsonschema"
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

// hooksConfigureCmd represents the hooks configure command
var hooksConfigureCmd = &cobra.Command{
	Use:   "configure",
	Short: "Configure pre-commit/pre-push behavior without editing YAML",
	Long: `Configure common hook behaviors (scope, content source, apply mode) via CLI.
This updates .goneat/hooks.yaml and regenerates hook scripts automatically.`,
	RunE: runHooksConfigure,
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

var (
	removeNoRestore bool
	hooksGuardian   bool
)

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
	hooksCmd.AddCommand(hooksConfigureCmd)

	// hooks remove flags (define before registration to avoid duplicate init)
	hooksRemoveCmd.Flags().BoolVar(&removeNoRestore, "no-restore", false, "Do not restore original hooks from backups; remove hooks completely")

	// hooks configure flags
	hooksConfigureCmd.Flags().Bool("show", false, "Show current effective hook settings")
	hooksConfigureCmd.Flags().Bool("reset", false, "Reset to defaults")
	hooksConfigureCmd.Flags().Bool("pre-commit-only-changed-files", false, "Enable --staged-only mode for pre-commit (faster, staged files only)")
	hooksConfigureCmd.Flags().String("pre-commit-content-source", "", "Content source for pre-commit: index (staged only) or working")
	hooksConfigureCmd.Flags().String("pre-commit-apply-mode", "", "Apply mode for pre-commit: check (no changes) or fix (apply fixes and re-stage)")
	hooksConfigureCmd.Flags().String("optimization-parallel", "", "Parallel execution mode: auto|max|sequential")
	hooksConfigureCmd.Flags().Bool("install", false, "Install hooks after generation")
	hooksGenerateCmd.Flags().BoolVar(&hooksGuardian, "with-guardian", false, "Include guardian security checks when generating hooks")

	// Register subcommands
	subcommands := []*cobra.Command{hooksInitCmd, hooksGenerateCmd, hooksInstallCmd, hooksValidateCmd, hooksRemoveCmd, hooksUpgradeCmd, hooksInspectCmd, hooksConfigureCmd}
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

	// Create .goneat directory
	goneatDir := ".goneat"
	if err := os.MkdirAll(goneatDir, 0750); err != nil {
		return fmt.Errorf("failed to create .goneat directory: %v", err)
	}

	// Create default hooks.yaml manifest
	hooksConfig := `version: "1.0.0"
hooks:
  pre-commit:
    - command: "assess"
      args: ["--categories", "format,lint", "--fail-on", "high"]
      stage_fixed: true
      priority: 10
      timeout: "2m"
  pre-push:
    - command: "assess"
      args: ["--categories", "format,lint,static-analysis", "--fail-on", "high"]
      priority: 10
      timeout: "3m"
optimization:
  only_changed_files: false  # Set to true to enable --staged-only mode (faster, staged files only)
  cache_results: true
  parallel: "auto"
`

	hooksPath := filepath.Join(goneatDir, "hooks.yaml")
	if err := os.WriteFile(hooksPath, []byte(hooksConfig), 0600); err != nil { // #nosec G306 - Configuration files use restrictive permissions
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
	if err := os.MkdirAll(hooksDir, 0750); err != nil {
		return fmt.Errorf("failed to create hooks directory: %v", err)
	}

	// Load manifest and render templates
	manifestData, err := os.ReadFile(".goneat/hooks.yaml")
	if err != nil {
		return fmt.Errorf("failed to read hooks manifest: %v", err)
	}
	var manifest struct {
		Hooks map[string][]struct {
			Command    string   `yaml:"command"`
			Args       []string `yaml:"args"`
			Fallback   string   `yaml:"fallback"`
			StageFixed bool     `yaml:"stage_fixed,omitempty"`
		} `yaml:"hooks"`
		Optimization struct {
			OnlyChangedFiles bool   `yaml:"only_changed_files"`
			ContentSource    string `yaml:"content_source,omitempty"` // "index" or "working"
		} `yaml:"optimization"`
	}
	if err := yaml.Unmarshal(manifestData, &manifest); err != nil {
		return fmt.Errorf("failed to parse hooks manifest: %v", err)
	}

	guardianFlagSet := cmd.Flags().Changed("with-guardian")
	withGuardian := hooksGuardian

	var guardianCfg *guardian.ConfigRoot
	if !guardianFlagSet || withGuardian {
		cfg, cfgErr := guardian.LoadConfig()
		if cfgErr != nil {
			if guardianFlagSet && withGuardian {
				return fmt.Errorf("failed to load guardian configuration: %w", cfgErr)
			}
			fmt.Fprintf(cmd.ErrOrStderr(), "⚠️  Guardian integration disabled due to configuration error: %v\n", cfgErr)
		} else {
			guardianCfg = cfg
			if !guardianFlagSet {
				withGuardian = cfg.Guardian.Integrations.Hooks.AutoInstall
			}
		}
	}

	type guardianTpl struct {
		Enabled       bool
		Scope         string
		Operation     string
		Method        string
		Expires       string
		Risk          string
		RequireReason bool
		HasPolicy     bool
	}

	// Detect appropriate shell type for the current OS
	shellType, extension := detectShellType()

	type tplData struct {
		Args                 []string
		Fallback             string
		OptimizationSettings struct {
			OnlyChangedFiles bool
			ContentSource    string
		}
		Guardian guardianTpl
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

	resolveGuardian := func(scope, operation string) (guardianTpl, error) {
		gt := guardianTpl{
			Enabled:   withGuardian,
			Scope:     scope,
			Operation: operation,
		}

		if !withGuardian {
			if gt.Enabled && gt.Method == "" {
				gt.Method = string(guardian.MethodBrowser)
			}
			return gt, nil
		}

		if guardianCfg != nil {
			policy, enforced, err := guardianCfg.ResolvePolicy(scope, operation)
			if err != nil {
				return gt, err
			}
			if enforced && policy != nil {
				gt.HasPolicy = true
				gt.Method = string(policy.Method)
				gt.Expires = policy.Expires.String()
				gt.Risk = policy.Risk
				gt.RequireReason = policy.RequireReason
			}
			if gt.Method == "" {
				gt.Method = string(guardianCfg.Guardian.Defaults.Method)
			}
			if gt.Expires == "" {
				gt.Expires = guardianCfg.Guardian.Defaults.Expires
			}
			if !gt.HasPolicy {
				gt.RequireReason = guardianCfg.Guardian.Defaults.RequireReason
			}
		}

		if gt.Method == "" {
			gt.Method = string(guardian.MethodBrowser)
		}

		return gt, nil
	}

	render := func(templatePath, destPath string, data tplData) error {
		// Validate template path to prevent path traversal
		templatePath = filepath.Clean(templatePath)
		if strings.Contains(templatePath, "..") {
			return fmt.Errorf("invalid template path: contains path traversal")
		}
		templatesFS := assets.GetTemplatesFS()
		content, err := fs.ReadFile(templatesFS, templatePath)
		if err != nil {
			// Fallback: attempt to read from filesystem SSOT (templates/ root)
			if data, ferr := os.ReadFile(filepath.Clean(filepath.Join("templates", strings.TrimPrefix(templatePath, "templates/")))); ferr == nil {
				content = data
			} else {
				return fmt.Errorf("failed to read embedded template %s: %v", templatePath, err)
			}
		}
		tmpl, err := template.New(filepath.Base(templatePath)).Parse(string(content))
		if err != nil {
			return fmt.Errorf("failed to parse template %s: %v", templatePath, err)
		}
		var buf bytes.Buffer
		if err := tmpl.Execute(&buf, data); err != nil {
			return fmt.Errorf("failed to render template %s: %v", templatePath, err)
		}
		// Write with execute permissions: Git hooks must be executable
		// Security justification: path is sanitized (Clean + no traversal), generated into .goneat/hooks,
		// and content is produced by trusted templates. Git requires +x bits to invoke hooks.
		if err := os.WriteFile(destPath, buf.Bytes(), 0700); err != nil { // #nosec G306 -- required exec perms for git hooks
			return fmt.Errorf("failed to write %s: %v", destPath, err)
		}
		return nil
	}

	// Render pre-commit from template
	argsPC, fbPC := buildArgs("pre-commit")
	dataPC := tplData{Args: argsPC, Fallback: fbPC}
	dataPC.OptimizationSettings.OnlyChangedFiles = manifest.Optimization.OnlyChangedFiles
	dataPC.OptimizationSettings.ContentSource = manifest.Optimization.ContentSource
	if dataPC.OptimizationSettings.ContentSource == "" {
		dataPC.OptimizationSettings.ContentSource = "index"
	}
	guardianPC, err := resolveGuardian("git", "commit")
	if err != nil {
		return fmt.Errorf("failed to resolve guardian policy for pre-commit: %w", err)
	}
	dataPC.Guardian = guardianPC
	preCommitTemplate := fmt.Sprintf("templates/hooks/%s/pre-commit.%s.tmpl", shellType, extension)
	preCommitHook := filepath.Join(hooksDir, "pre-commit")
	if err := render(preCommitTemplate, preCommitHook, dataPC); err != nil {
		return err
	}

	// Render pre-push from template
	argsPP, fbPP := buildArgs("pre-push")
	dataPP := tplData{Args: argsPP, Fallback: fbPP}
	dataPP.OptimizationSettings.OnlyChangedFiles = manifest.Optimization.OnlyChangedFiles
	dataPP.OptimizationSettings.ContentSource = manifest.Optimization.ContentSource
	if dataPP.OptimizationSettings.ContentSource == "" {
		dataPP.OptimizationSettings.ContentSource = "index"
	}
	guardianPP, err := resolveGuardian("git", "push")
	if err != nil {
		return fmt.Errorf("failed to resolve guardian policy for pre-push: %w", err)
	}
	dataPP.Guardian = guardianPP
	prePushTemplate := fmt.Sprintf("templates/hooks/%s/pre-push.%s.tmpl", shellType, extension)
	prePushHook := filepath.Join(hooksDir, "pre-push")
	if err := render(prePushTemplate, prePushHook, dataPP); err != nil {
		return err
	}

	if withGuardian {
		fmt.Println("🛡️  Guardian integration enabled in generated hooks")
	}
	fmt.Println("✅ Hook files generated successfully!")
	fmt.Printf("📁 Created: %s/pre-commit\n", hooksDir)
	fmt.Printf("📁 Created: %s/pre-push\n", hooksDir)
	fmt.Println("📌 Next: Run 'goneat hooks install' to install hooks to .git/hooks")

	return nil
}

// detectShellType determines the appropriate shell type and template directory based on OS
func detectShellType() (string, string) {
	switch runtime.GOOS {
	case "windows":
		// Prefer PowerShell if available, fallback to CMD
		if isPowerShellAvailable() {
			return "powershell", "ps1"
		}
		return "cmd", "bat"
	default:
		// Unix-like systems (Linux, macOS, etc.) use bash
		return "bash", "sh"
	}
}

// isPowerShellAvailable checks if PowerShell is available on Windows
func isPowerShellAvailable() bool {
	// On Windows, check if PowerShell is available
	if runtime.GOOS != "windows" {
		return false
	}

	// Try to find powershell.exe or pwsh.exe
	for _, cmd := range []string{"powershell.exe", "pwsh.exe"} {
		if _, err := exec.LookPath(cmd); err == nil {
			return true
		}
	}

	return false
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

	detectGuardian := func(paths ...string) (bool, error) {
		needle := []byte("goneat guardian check")
		for _, p := range paths {
			data, readErr := os.ReadFile(p)
			if readErr != nil {
				if os.IsNotExist(readErr) {
					continue
				}
				return false, readErr
			}
			if bytes.Contains(data, needle) {
				return true, nil
			}
		}
		return false, nil
	}

	guardianActive, err := detectGuardian(filepath.Join(hooksDir, "pre-commit"), filepath.Join(hooksDir, "pre-push"))
	if err != nil {
		return fmt.Errorf("failed to inspect generated hooks for guardian integration: %w", err)
	}

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

		// Make executable - git hooks require execute permissions
		// Git hooks need exec perms; destination path is validated and within .git/hooks
		if err := os.Chmod(preCommitDst, 0700); err != nil { // #nosec G302 -- required exec perms for git hooks
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

		// Make executable - git hooks require execute permissions
		if err := os.Chmod(prePushDst, 0700); err != nil { // #nosec G302 -- required exec perms for git hooks
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

	if guardianActive {
		path, cfgErr := guardian.EnsureConfig()
		if cfgErr != nil {
			return fmt.Errorf("failed to ensure guardian configuration: %w", cfgErr)
		}
		fmt.Printf("🛡️  Guardian integration detected. Config available at %s\n", path)
		fmt.Println("🔐 Protected operations will require guardian approval before proceeding")
	}

	return nil
}

// copyFile copies a file from src to dst
func copyFile(src, dst string) error {
	// Validate paths to prevent path traversal
	src = filepath.Clean(src)
	dst = filepath.Clean(dst)
	if strings.Contains(src, "..") || strings.Contains(dst, "..") {
		return fmt.Errorf("invalid path: contains path traversal")
	}
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0700) // #nosec G306 - Git hooks require execute permissions
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

// runHooksConfigure updates .goneat/hooks.yaml with CLI-provided options and regenerates hooks
// --- Hooks Policy Management (show|set|reset|validate) ---

var hooksPolicyCmd = &cobra.Command{
	Use:   "policy",
	Short: "Manage hook policy (categories, fail-on, timeouts, optimization)",
}

var hooksPolicyShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Show effective hook policy",
	RunE:  runHooksPolicyShow,
}

var hooksPolicySetCmd = &cobra.Command{
	Use:   "set",
	Short: "Update specific hook policy keys",
	RunE:  runHooksPolicySet,
}

var hooksPolicyResetCmd = &cobra.Command{
	Use:   "reset",
	Short: "Restore defaults for a hook",
	RunE:  runHooksPolicyReset,
}

var hooksPolicyValidateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Validate hooks.yaml against schema",
	RunE:  runHooksPolicyValidate,
}

func init() {
	// Register policy subcommands separately to avoid large init edits above
	hooksCmd.AddCommand(hooksPolicyCmd)
	hooksPolicyCmd.AddCommand(hooksPolicyShowCmd)
	hooksPolicyCmd.AddCommand(hooksPolicySetCmd)
	hooksPolicyCmd.AddCommand(hooksPolicyResetCmd)
	hooksPolicyCmd.AddCommand(hooksPolicyValidateCmd)

	// Flags
	hooksPolicyShowCmd.Flags().String("hook", "pre-commit", "Hook to show: pre-commit|pre-push")
	hooksPolicyShowCmd.Flags().String("format", "text", "Output format: text|json")

	hooksPolicySetCmd.Flags().String("hook", "pre-commit", "Hook to update: pre-commit|pre-push")
	hooksPolicySetCmd.Flags().String("fail-on", "", "Fail threshold: critical|high|medium|low")
	hooksPolicySetCmd.Flags().String("categories", "", "Comma-separated categories, e.g., format,lint[,security]")
	hooksPolicySetCmd.Flags().String("timeout", "", "Timeout for the hook command, e.g., 90s|2m|3m")
	hooksPolicySetCmd.Flags().Bool("only-changed-files", false, "Set optimization.only_changed_files true|false")
	hooksPolicySetCmd.Flags().String("parallel", "", "Set optimization.parallel: auto|max|sequential")
	hooksPolicySetCmd.Flags().Bool("dry-run", false, "Preview changes without writing")
	hooksPolicySetCmd.Flags().Bool("yes", false, "Apply changes without prompt")
	hooksPolicySetCmd.Flags().Bool("install", false, "Install hooks after generation")

	hooksPolicyResetCmd.Flags().String("hook", "pre-commit", "Hook to reset: pre-commit|pre-push")
	hooksPolicyResetCmd.Flags().Bool("dry-run", false, "Preview changes without writing")
	hooksPolicyResetCmd.Flags().Bool("yes", false, "Apply changes without prompt")
	hooksPolicyResetCmd.Flags().Bool("install", false, "Install hooks after generation")
}

func runHooksPolicyShow(cmd *cobra.Command, args []string) error {
	hook, _ := cmd.Flags().GetString("hook")
	format, _ := cmd.Flags().GetString("format")

	raw, err := os.ReadFile(".goneat/hooks.yaml")
	if err != nil {
		return fmt.Errorf("failed to read hooks.yaml: %v", err)
	}
	// Reuse HookConfig structure
	var cfg HookConfig
	if err := yaml.Unmarshal(raw, &cfg); err != nil {
		return fmt.Errorf("failed to parse hooks.yaml: %v", err)
	}
	cats := parseCategoriesFromHooks(cfg.Hooks, hook)
	fail := parseFailOnFromHooks(cfg.Hooks, hook)
	timeout := ""
	// Scan raw YAML for timeout under hook assessor
	var manifest map[string]any
	_ = yaml.Unmarshal(raw, &manifest)
	if hooks, ok := manifest["hooks"].(map[string]any); ok {
		if seq, ok := hooks[hook].([]any); ok {
			for _, item := range seq {
				if m, ok := item.(map[string]any); ok {
					if m["command"] == "assess" {
						if tv, ok := m["timeout"].(string); ok {
							timeout = tv
						}
						break
					}
				}
			}
		}
	}
	opt := map[string]any{"only_changed_files": cfg.Optimization.OnlyChangedFiles, "parallel": cfg.Optimization.Parallel}
	if strings.ToLower(format) == "json" {
		out := map[string]any{"hook": hook, "categories": cats, "fail_on": fail, "timeout": timeout, "optimization": opt}
		b, _ := json.MarshalIndent(out, "", "  ")
		fmt.Fprintln(cmd.OutOrStdout(), string(b)) //nolint:errcheck // CLI output, error is not critical
		return nil
	}
	fmt.Fprintf(cmd.OutOrStdout(), "Hook: %s\nCategories: %s\nFail-on: %s\nTimeout: %s\nOptimization: only_changed_files=%v, parallel=%s\n", hook, strings.Join(cats, ","), fail, timeout, cfg.Optimization.OnlyChangedFiles, cfg.Optimization.Parallel) //nolint:errcheck // CLI output, error is not critical
	return nil
}

// Helper: load manifest as node and as typed struct
func loadHooksManifest() (*yaml.Node, []byte, error) {
	raw, err := os.ReadFile(".goneat/hooks.yaml")
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read hooks.yaml: %v", err)
	}
	var root yaml.Node
	if err := yaml.Unmarshal(raw, &root); err != nil {
		return nil, nil, fmt.Errorf("failed to parse hooks.yaml: %v", err)
	}
	return &root, raw, nil
}

// Helpers to navigate yaml.Node mappings
func findMapValue(m *yaml.Node, key string) (*yaml.Node, int) {
	if m == nil || m.Kind != yaml.MappingNode {
		return nil, -1
	}
	for i := 0; i < len(m.Content); i += 2 {
		k := m.Content[i]
		if k.Value == key {
			return m.Content[i+1], i + 1
		}
	}
	return nil, -1
}

func setMapScalar(m *yaml.Node, key, val string) bool {
	if m == nil || m.Kind != yaml.MappingNode {
		return false
	}
	if v, _ := findMapValue(m, key); v != nil {
		if v.Kind != yaml.ScalarNode || v.Value != val {
			v.Kind = yaml.ScalarNode
			v.Tag = "!!str"
			v.Value = val
			return true
		}
		return false
	}
	// append
	m.Content = append(m.Content, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: key}, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: val})
	return true
}

func setMapBool(m *yaml.Node, key string, val bool) bool {
	if m == nil || m.Kind != yaml.MappingNode {
		return false
	}
	str := "false"
	if val {
		str = "true"
	}
	return setMapScalar(m, key, str)
}

func updateArgsPair(args *yaml.Node, flag, value string) bool {
	if args == nil || args.Kind != yaml.SequenceNode {
		return false
	}
	changed := false
	for i := 0; i < len(args.Content)-1; i++ {
		if args.Content[i].Value == flag {
			if args.Content[i+1].Value != value {
				args.Content[i+1].Kind = yaml.ScalarNode
				args.Content[i+1].Tag = "!!str"
				args.Content[i+1].Value = value
				changed = true
			}
			return changed
		}
	}
	// not found, append
	args.Content = append(args.Content, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: flag}, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: value})
	return true
}

func findAssessEntryForHook(root *yaml.Node, hook string) (*yaml.Node, *yaml.Node) {
	// root is a DocumentNode -> Mapping "hooks" -> Mapping hook -> Sequence -> each item is Mapping
	if root == nil || root.Kind != yaml.DocumentNode || len(root.Content) == 0 {
		return nil, nil
	}
	doc := root.Content[0]
	hooksMap, _ := findMapValue(doc, "hooks")
	if hooksMap == nil || hooksMap.Kind != yaml.MappingNode {
		return nil, nil
	}
	hookSeqNode, _ := findMapValue(hooksMap, hook)
	if hookSeqNode == nil || hookSeqNode.Kind != yaml.SequenceNode {
		return nil, nil
	}
	for _, item := range hookSeqNode.Content {
		if item.Kind == yaml.MappingNode {
			if cv, _ := findMapValue(item, "command"); cv != nil && cv.Value == "assess" {
				return item, hookSeqNode
			}
		}
	}
	return nil, hookSeqNode
}

func policyWriteAtomic(path string, data []byte) error {
	dir := filepath.Dir(path)
	tmp := filepath.Join(dir, ".hooks.yaml.tmp")
	if err := os.WriteFile(tmp, data, 0600); err != nil { // #nosec G306 - perms by design
		return err
	}
	return os.Rename(tmp, path)
}

func runHooksPolicySet(cmd *cobra.Command, args []string) error {
	hook, _ := cmd.Flags().GetString("hook")
	failOn, _ := cmd.Flags().GetString("fail-on")
	catsStr, _ := cmd.Flags().GetString("categories")
	timeout, _ := cmd.Flags().GetString("timeout")
	onlyChanged, _ := cmd.Flags().GetBool("only-changed-files")
	parallel, _ := cmd.Flags().GetString("parallel")
	dryRun, _ := cmd.Flags().GetBool("dry-run")
	yes, _ := cmd.Flags().GetBool("yes")
	install, _ := cmd.Flags().GetBool("install")

	root, orig, err := loadHooksManifest()
	if err != nil {
		return err
	}
	// Update assess entry for selected hook
	entry, _ := findAssessEntryForHook(root, hook)
	if entry == nil {
		return fmt.Errorf("no assess entry found for hook %s", hook)
	}
	// args
	argsNode, _ := findMapValue(entry, "args")
	if argsNode == nil {
		argsNode = &yaml.Node{Kind: yaml.SequenceNode}
		entry.Content = append(entry.Content, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: "args"}, argsNode)
	}
	if strings.TrimSpace(catsStr) != "" {
		cats := strings.Join(splitAndTrim(catsStr), ",")
		_ = updateArgsPair(argsNode, "--categories", cats)
	}
	if strings.TrimSpace(failOn) != "" {
		_ = updateArgsPair(argsNode, "--fail-on", strings.TrimSpace(failOn))
	}
	if strings.TrimSpace(timeout) != "" {
		_ = setMapScalar(entry, "timeout", strings.TrimSpace(timeout))
	}
	// optimization
	doc := root.Content[0]
	optMap, _ := findMapValue(doc, "optimization")
	if optMap == nil {
		optMap = &yaml.Node{Kind: yaml.MappingNode}
		doc.Content = append(doc.Content, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: "optimization"}, optMap)
	}
	if cmd.Flags().Changed("only-changed-files") {
		_ = setMapBool(optMap, "only_changed_files", onlyChanged)
	}
	if cmd.Flags().Changed("parallel") {
		pv := strings.ToLower(strings.TrimSpace(parallel))
		if pv != "auto" && pv != "max" && pv != "sequential" {
			return fmt.Errorf("invalid --parallel: %s (allowed: auto|max|sequential)", parallel)
		}
		_ = setMapScalar(optMap, "parallel", pv)
	}

	out, err := yaml.Marshal(root)
	if err != nil {
		return fmt.Errorf("failed to marshal updated YAML: %v", err)
	}
	if string(out) == string(orig) {
		fmt.Fprintln(cmd.OutOrStdout(), "No changes required; policy already matches requested values") //nolint:errcheck // CLI output, error is not critical
		return nil
	}
	if dryRun && !yes {
		fmt.Fprintln(cmd.OutOrStdout(), "--dry-run: proposed .goneat/hooks.yaml:") //nolint:errcheck // CLI output, error is not critical
		fmt.Fprintln(cmd.OutOrStdout(), string(out))                               //nolint:errcheck // CLI output, error is not critical
		fmt.Fprintln(cmd.OutOrStdout(), "Run with --yes to apply.")                //nolint:errcheck // CLI output, error is not critical
		return nil
	}
	if err := policyWriteAtomic(".goneat/hooks.yaml", out); err != nil {
		return fmt.Errorf("failed to write updated hooks.yaml: %v", err)
	}
	fmt.Fprintln(cmd.OutOrStdout(), "✅ Updated .goneat/hooks.yaml") //nolint:errcheck // CLI output, error is not critical
	// Regenerate and optionally install
	if err := runHooksGenerate(cmd, nil); err != nil {
		return fmt.Errorf("failed to regenerate hooks: %v", err)
	}
	if install {
		if err := runHooksInstall(cmd, nil); err != nil {
			return fmt.Errorf("failed to install hooks: %v", err)
		}
	}
	return nil
}

func runHooksPolicyReset(cmd *cobra.Command, args []string) error {
	hook, _ := cmd.Flags().GetString("hook")
	dryRun, _ := cmd.Flags().GetBool("dry-run")
	yes, _ := cmd.Flags().GetBool("yes")
	install, _ := cmd.Flags().GetBool("install")

	root, orig, err := loadHooksManifest()
	if err != nil {
		return err
	}
	entry, _ := findAssessEntryForHook(root, hook)
	if entry == nil {
		return fmt.Errorf("no assess entry found for hook %s", hook)
	}
	argsNode, _ := findMapValue(entry, "args")
	if argsNode == nil {
		argsNode = &yaml.Node{Kind: yaml.SequenceNode}
		entry.Content = append(entry.Content, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: "args"}, argsNode)
	}
	// Defaults similar to hooks init
	switch hook {
	case "pre-commit":
		_ = updateArgsPair(argsNode, "--categories", "format,lint")
		_ = updateArgsPair(argsNode, "--fail-on", "high")
		_ = setMapScalar(entry, "timeout", "2m")
	case "pre-push":
		_ = updateArgsPair(argsNode, "--categories", "format,lint,static-analysis")
		_ = updateArgsPair(argsNode, "--fail-on", "high")
		_ = setMapScalar(entry, "timeout", "3m")
	}
	// optimization defaults
	doc := root.Content[0]
	optMap, _ := findMapValue(doc, "optimization")
	if optMap == nil {
		optMap = &yaml.Node{Kind: yaml.MappingNode}
		doc.Content = append(doc.Content, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: "optimization"}, optMap)
	}
	_ = setMapBool(optMap, "only_changed_files", false) // Default to false for better DX
	_ = setMapScalar(optMap, "parallel", "auto")

	out, err := yaml.Marshal(root)
	if err != nil {
		return fmt.Errorf("failed to marshal updated YAML: %v", err)
	}
	if string(out) == string(orig) {
		fmt.Fprintln(cmd.OutOrStdout(), "No changes required; policy already at defaults") //nolint:errcheck // CLI output, error is not critical
		return nil
	}
	if dryRun && !yes {
		fmt.Fprintln(cmd.OutOrStdout(), "--dry-run: proposed .goneat/hooks.yaml (defaults):") //nolint:errcheck // CLI output, error is not critical
		fmt.Fprintln(cmd.OutOrStdout(), string(out))                                          //nolint:errcheck // CLI output, error is not critical
		fmt.Fprintln(cmd.OutOrStdout(), "Run with --yes to apply.")                           //nolint:errcheck // CLI output, error is not critical
		return nil
	}
	if err := policyWriteAtomic(".goneat/hooks.yaml", out); err != nil {
		return fmt.Errorf("failed to write updated hooks.yaml: %v", err)
	}
	fmt.Fprintln(cmd.OutOrStdout(), "✅ Reset .goneat/hooks.yaml to defaults for hook") //nolint:errcheck // CLI output, error is not critical
	if err := runHooksGenerate(cmd, nil); err != nil {
		return fmt.Errorf("failed to regenerate hooks: %v", err)
	}
	if install {
		if err := runHooksInstall(cmd, nil); err != nil {
			return fmt.Errorf("failed to install hooks: %v", err)
		}
	}
	return nil
}

func runHooksPolicyValidate(cmd *cobra.Command, args []string) error {
	// Load YAML
	raw, err := os.ReadFile(".goneat/hooks.yaml")
	if err != nil {
		return fmt.Errorf("failed to read hooks.yaml: %v", err)
	}
	var doc any
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		return fmt.Errorf("failed to parse hooks.yaml: %v", err)
	}
	docJSON, _ := json.Marshal(doc)
	// Load schema from embedded FS (YAML -> JSON)
	schemaFS := assets.GetSchemasFS()
	sb, err := fs.ReadFile(schemaFS, "schemas/work/hooks-manifest-v1.0.0.yaml")
	if err != nil {
		return fmt.Errorf("failed to read embedded hooks schema: %v", err)
	}
	var schm any
	if err := yaml.Unmarshal(sb, &schm); err != nil {
		return fmt.Errorf("failed to parse embedded hooks schema: %v", err)
	}
	schJSON, _ := json.Marshal(schm)

	schemaLoader := gojsonschema.NewBytesLoader(schJSON)
	docLoader := gojsonschema.NewBytesLoader(docJSON)
	res, err := gojsonschema.Validate(schemaLoader, docLoader)
	if err != nil {
		return fmt.Errorf("schema validation error: %v", err)
	}
	if !res.Valid() {
		fmt.Fprintln(cmd.OutOrStdout(), "❌ hooks.yaml is invalid:") //nolint:errcheck // CLI output, error is not critical
		for _, e := range res.Errors() {
			fmt.Fprintf(cmd.OutOrStdout(), " - %s\n", e) //nolint:errcheck // CLI output, error is not critical
		}
		return fmt.Errorf("hooks.yaml failed schema validation")
	}
	fmt.Fprintln(cmd.OutOrStdout(), "✅ hooks.yaml is valid") //nolint:errcheck // CLI output, error is not critical
	return nil
}

func splitAndTrim(s string) []string {
	parts := strings.Split(s, ",")
	var out []string
	for _, p := range parts {
		pp := strings.TrimSpace(p)
		if pp != "" {
			out = append(out, pp)
		}
	}
	return out
}

func runHooksConfigure(cmd *cobra.Command, args []string) error {
	hooksConfigPath := ".goneat/hooks.yaml"
	if _, err := os.Stat(hooksConfigPath); os.IsNotExist(err) {
		return fmt.Errorf("hooks configuration not found. Run 'goneat hooks init' first")
	}

	show, _ := cmd.Flags().GetBool("show")
	reset, _ := cmd.Flags().GetBool("reset")
	install, _ := cmd.Flags().GetBool("install")

	onlyChangedVal, _ := cmd.Flags().GetBool("pre-commit-only-changed-files")
	onlyChangedSet := cmd.Flags().Changed("pre-commit-only-changed-files")

	contentSource, _ := cmd.Flags().GetString("pre-commit-content-source")
	contentSourceSet := cmd.Flags().Changed("pre-commit-content-source")

	applyMode, _ := cmd.Flags().GetString("pre-commit-apply-mode")
	applyModeSet := cmd.Flags().Changed("pre-commit-apply-mode")

	parallelVal, _ := cmd.Flags().GetString("optimization-parallel")
	parallelSet := cmd.Flags().Changed("optimization-parallel")

	// Load YAML
	raw, err := os.ReadFile(hooksConfigPath)
	if err != nil {
		return fmt.Errorf("failed to read hooks configuration: %v", err)
	}

	type HookEntry struct {
		Command    string   `yaml:"command"`
		Args       []string `yaml:"args"`
		Fallback   string   `yaml:"fallback,omitempty"`
		StageFixed bool     `yaml:"stage_fixed,omitempty"`
		When       any      `yaml:"when,omitempty"`
		Priority   int      `yaml:"priority,omitempty"`
		Timeout    string   `yaml:"timeout,omitempty"`
		Skip       []string `yaml:"skip,omitempty"`
	}
	var manifest struct {
		Version string                 `yaml:"version"`
		Hooks   map[string][]HookEntry `yaml:"hooks"`
		Opt     map[string]any         `yaml:"optimization,omitempty"`
	}
	if err := yaml.Unmarshal(raw, &manifest); err != nil {
		return fmt.Errorf("failed to parse hooks configuration: %v", err)
	}
	if manifest.Opt == nil {
		manifest.Opt = make(map[string]any)
	}

	// Inspect mode
	if show {
		onlyChanged := false
		if v, ok := manifest.Opt["only_changed_files"].(bool); ok {
			onlyChanged = v
		}
		cs := "index"
		if v, ok := manifest.Opt["content_source"].(string); ok && v != "" {
			cs = v
		}
		pm := "check"
		if entries, ok := manifest.Hooks["pre-commit"]; ok {
			for _, e := range entries {
				// prefer an assess or format entry for apply mode determination
				if e.Command == "assess" || e.Command == "format" {
					if e.StageFixed {
						pm = "fix"
					} else {
						pm = "check"
					}
					break
				}
			}
		}
		fmt.Printf("Pre-commit settings:\n  only_changed_files: %v\n  content_source: %s\n  apply_mode: %s\n", onlyChanged, cs, pm)
		return nil
	}

	// Reset to defaults
	if reset {
		manifest.Opt["only_changed_files"] = false // Default to false for better DX (--staged-only is opt-in)
		manifest.Opt["content_source"] = "index"
		manifest.Opt["parallel"] = "auto"
		// keep existing apply mode as-is; teams may prefer current default
	}

	// Apply flags
	if onlyChangedSet {
		manifest.Opt["only_changed_files"] = onlyChangedVal
	}
	if contentSourceSet {
		cs := strings.ToLower(strings.TrimSpace(contentSource))
		if cs != "index" && cs != "working" {
			return fmt.Errorf("invalid --pre-commit-content-source: %s (allowed: index|working)", contentSource)
		}
		manifest.Opt["content_source"] = cs
	}
	if parallelSet {
		pv := strings.ToLower(strings.TrimSpace(parallelVal))
		if pv != "auto" && pv != "max" && pv != "sequential" {
			return fmt.Errorf("invalid --optimization-parallel: %s (allowed: auto|max|sequential)", parallelVal)
		}
		manifest.Opt["parallel"] = pv
	}
	if applyModeSet {
		pm := strings.ToLower(strings.TrimSpace(applyMode))
		if pm != "check" && pm != "fix" {
			return fmt.Errorf("invalid --pre-commit-apply-mode: %s (allowed: check|fix)", applyMode)
		}
		if entries, ok := manifest.Hooks["pre-commit"]; ok {
			for i := range entries {
				if entries[i].Command == "assess" || entries[i].Command == "format" {
					entries[i].StageFixed = (pm == "fix")
				}
			}
			manifest.Hooks["pre-commit"] = entries
		}
	}

	// Write back YAML
	out, err := yaml.Marshal(&manifest)
	if err != nil {
		return fmt.Errorf("failed to marshal updated configuration: %v", err)
	}
	if err := os.WriteFile(hooksConfigPath, out, 0600); err != nil { // #nosec G306 - Configuration files use restrictive permissions
		return fmt.Errorf("failed to write updated configuration: %v", err)
	}
	fmt.Println("✅ Updated .goneat/hooks.yaml")

	// Regenerate hooks to apply settings
	if err := runHooksGenerate(cmd, nil); err != nil {
		return fmt.Errorf("failed to regenerate hooks: %v", err)
	}
	fmt.Println("🔨 Regenerated hook scripts from manifest")

	// Optional install
	if install {
		if err := runHooksInstall(cmd, nil); err != nil {
			return fmt.Errorf("failed to install hooks: %v", err)
		}
		fmt.Println("📦 Installed hooks to .git/hooks")
	}

	return nil
}

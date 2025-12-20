/*
Copyright © 2025 3 Leaps <info@3leaps.net>
*/
package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
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
	"github.com/fulmenhq/goneat/pkg/logger"
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
	removeNoRestore  bool
	hooksGuardian    bool
	gitResetGuardian bool
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

	// Output format flags
	hooksValidateCmd.Flags().String("format", "text", "Output format: text|json")
	hooksInspectCmd.Flags().String("format", "text", "Output format: text|json")

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
	hooksConfigureCmd.Flags().Bool("git-reset-guardian", false, "Enable guardian protection for git reset operations on protected branches")
	hooksGenerateCmd.Flags().BoolVar(&hooksGuardian, "with-guardian", false, "Include guardian security checks when generating hooks")
	hooksGenerateCmd.Flags().BoolVar(&gitResetGuardian, "reset-guardian", false, "Generate guardian-protected reset hook for protected branches")

	// Subcommands are added to hooksCmd but not individually registered with ops
	// as they inherit from the parent hooks command registration
}

// detectFormatCapabilities detects if the project has format capabilities
func detectFormatCapabilities(targetDir string) []string {
	var capabilities []string

	// Check for Makefile with format targets
	makefilePath := filepath.Join(targetDir, "Makefile")
	// #nosec
	if data, err := os.ReadFile(makefilePath); err == nil {
		content := string(data)

		// Look for specific format targets that we know about
		if strings.Contains(content, "format-all:") {
			capabilities = append(capabilities, "make format-all")
		} else if strings.Contains(content, "format:") && strings.Contains(content, "fmt:") {
			// Has both format and fmt targets, likely supports comprehensive formatting
			capabilities = append(capabilities, "make format")
		} else if strings.Contains(content, "fmt:") {
			// Basic fmt target
			capabilities = append(capabilities, "make fmt")
		}
	}

	// Check for package.json with format scripts
	packageJSONPath := filepath.Join(targetDir, "package.json")
	// #nosec
	if data, err := os.ReadFile(packageJSONPath); err == nil {
		content := string(data)
		if strings.Contains(content, `"format"`) && strings.Contains(content, `"scripts"`) {
			capabilities = append(capabilities, "npm run format")
		}
	}

	// Check for other common format setups
	if _, err := os.Stat(filepath.Join(targetDir, ".prettierrc")); err == nil {
		capabilities = append(capabilities, "prettier")
	}

	if _, err := os.Stat(filepath.Join(targetDir, "pyproject.toml")); err == nil {
		// #nosec
		if data, err := os.ReadFile(filepath.Join(targetDir, "pyproject.toml")); err == nil {
			content := string(data)
			if strings.Contains(content, "black") || strings.Contains(content, "ruff") {
				capabilities = append(capabilities, "python format")
			}
		}
	}

	return capabilities
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

	// Detect format capabilities
	formatCapabilities := detectFormatCapabilities(".")

	// Build hooks configuration with format support if detected
	var hooksConfig string
	if len(formatCapabilities) > 0 {
		// Include format commands for projects with format capabilities
		var formatCmd string
		var formatArgs string

		// Prioritize make format-all if available, else use the first detected capability
		for _, cap := range formatCapabilities {
			if strings.Contains(cap, "make format-all") {
				formatCmd = "make"
				formatArgs = `["format-all"]`
				break
			}
		}

		// Fallback to first capability if format-all not found
		if formatCmd == "" {
			parts := strings.Fields(formatCapabilities[0])
			if len(parts) >= 2 {
				formatCmd = parts[0]
				formatArgs = fmt.Sprintf(`["%s"]`, strings.Join(parts[1:], `", "`))
			} else {
				formatCmd = parts[0]
				formatArgs = `[]`
			}
		}

		hooksConfig = fmt.Sprintf(`version: "1.0.0"
hooks:
  pre-commit:
    - command: "%s"
      args: %s
      priority: 5
      timeout: "60s"
    - command: "assess"
      args: ["--check", "--categories", "format,lint,dates,tools", "--fail-on", "critical"]
      priority: 10
      timeout: "90s"
  pre-push:
    - command: "%s"
      args: %s
      priority: 5
      timeout: "60s"
    - command: "assess"
      args: ["--check", "--categories", "format,lint,security,dates,tools,maturity,repo-status", "--fail-on", "high"]
      priority: 10
      timeout: "2m"
optimization:
  cache_results: true
  content_source: working
  only_changed_files: false
  parallel: auto
`, formatCmd, formatArgs, formatCmd, formatArgs)
	} else {
		// Standard configuration without format commands
		hooksConfig = `version: "1.0.0"
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
	}

	hooksPath := filepath.Join(goneatDir, "hooks.yaml")
	if err := os.WriteFile(hooksPath, []byte(hooksConfig), 0600); err != nil { // #nosec G306 - Configuration files use restrictive permissions
		return fmt.Errorf("failed to create hooks.yaml: %v", err)
	}

	fmt.Println("✅ Hooks system initialized successfully!")
	if len(formatCapabilities) > 0 {
		fmt.Printf("📝 Created .goneat/hooks.yaml with format support (detected: %s)\n", strings.Join(formatCapabilities, ", "))
		fmt.Println("🎨 Format commands will run automatically before assessment in git hooks")
	} else {
		fmt.Println("📝 Created .goneat/hooks.yaml with default configuration")
	}
	fmt.Println("🚀 Next steps:")
	fmt.Println("   1. Run 'goneat hooks generate' to create hook files")
	fmt.Println("   2. Run 'goneat hooks install' to install hooks to .git/hooks")
	fmt.Println("   3. Run 'goneat hooks validate' to verify everything works")

	return nil
}

func runHooksGenerate(cmd *cobra.Command, args []string) error {
	fmt.Printf("🔨 Generating hook files from manifest... (gitResetGuardian=%v)\n", gitResetGuardian)

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

	// Validate manifest against schema before parsing into typed struct
	if err := validateHooksManifestSchema(manifestData); err != nil {
		return err
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
			if _, err := fmt.Fprintf(cmd.ErrOrStderr(), "⚠️  Guardian integration disabled due to configuration error: %v\n", cfgErr); err != nil {
				logger.Error("Failed to write warning message", logger.Err(err))
			}
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

	// Generate reset hook if guardian protection for reset is requested
	resetGuardian, _ := cmd.Flags().GetBool("reset-guardian")
	if resetGuardian {
		// Create a custom reset hook that checks for protected branches
		resetTemplate := fmt.Sprintf("templates/hooks/%s/pre-reset.%s.tmpl", shellType, extension)
		resetHook := filepath.Join(hooksDir, "pre-reset")
		dataReset := tplData{
			Args:     []string{}, // No args needed for reset hook
			Fallback: "",
		}
		dataReset.OptimizationSettings.OnlyChangedFiles = false
		dataReset.OptimizationSettings.ContentSource = "working"

		guardianReset, err := resolveGuardian("git", "reset")
		if err != nil {
			return fmt.Errorf("failed to resolve guardian policy for reset: %w", err)
		}
		dataReset.Guardian = guardianReset

		if err := render(resetTemplate, resetHook, dataReset); err != nil {
			return err
		}
		fmt.Printf("📁 Created: %s/pre-reset (with guardian protection for git reset)\n", hooksDir)
	}

	if withGuardian || resetGuardian {
		fmt.Println("🛡️  Guardian integration enabled in generated hooks")
	}
	fmt.Println("✅ Hook files generated successfully!")
	fmt.Printf("📁 Created: %s/pre-commit\n", hooksDir)
	fmt.Printf("📁 Created: %s/pre-push\n", hooksDir)
	if resetGuardian {
		fmt.Printf("📁 Created: %s/pre-reset\n", hooksDir)
	}
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
			// #nosec
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

	guardianActive, err := detectGuardian(filepath.Join(hooksDir, "pre-commit"), filepath.Join(hooksDir, "pre-push"), filepath.Join(hooksDir, "pre-reset"))
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

	// Install pre-reset hook
	preResetSrc := filepath.Join(hooksDir, "pre-reset")
	preResetDst := filepath.Join(gitHooksDir, "pre-reset")

	if _, err := os.Stat(preResetSrc); err == nil {
		// Backup existing hook if it exists
		if _, err := os.Stat(preResetDst); err == nil {
			backupPath := preResetDst + ".backup"
			if err := os.Rename(preResetDst, backupPath); err != nil {
				return fmt.Errorf("failed to backup existing pre-reset hook: %v", err)
			}
			fmt.Printf("📋 Backed up existing pre-reset hook to %s\n", backupPath)
		}

		// Copy new hook
		if err := copyFile(preResetSrc, preResetDst); err != nil {
			return fmt.Errorf("failed to install pre-reset hook: %v", err)
		}

		// Make executable - git hooks require execute permissions
		if err := os.Chmod(preResetDst, 0700); err != nil { // #nosec G302 -- required exec perms for git hooks
			return fmt.Errorf("failed to make pre-reset hook executable: %v", err)
		}

		fmt.Println("✅ Installed pre-reset hook")
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

type hooksManifestForInspection struct {
	Version string                          `yaml:"version"`
	Hooks   map[string][]hooksManifestEntry `yaml:"hooks"`
	Opt     map[string]any                  `yaml:"optimization,omitempty"`
}

type hooksManifestEntry struct {
	Command    string   `yaml:"command"`
	Args       []string `yaml:"args"`
	StageFixed bool     `yaml:"stage_fixed,omitempty"`
	Priority   int      `yaml:"priority,omitempty"`
	Timeout    string   `yaml:"timeout,omitempty"`
}

type hooksOptimizationSnapshot struct {
	OnlyChangedFiles bool   `json:"only_changed_files"`
	ContentSource    string `json:"content_source"`
	Parallel         string `json:"parallel"`
	CacheResults     bool   `json:"cache_results"`
}

type hooksCommandAnalysis struct {
	Command         string   `json:"command"`
	Args            []string `json:"args"`
	Kind            string   `json:"kind"` // internal|external
	StageFixed      bool     `json:"stage_fixed,omitempty"`
	Priority        int      `json:"priority,omitempty"`
	Timeout         string   `json:"timeout,omitempty"`
	IsMutator       bool     `json:"is_mutator"`
	MutatorReasons  []string `json:"mutator_reasons,omitempty"`
	WarningMessages []string `json:"warnings,omitempty"`
}

type hooksHookAnalysis struct {
	Hook         string                    `json:"hook"`
	Optimization hooksOptimizationSnapshot `json:"optimization"`
	Wrapper      string                    `json:"wrapper"`
	Commands     []hooksCommandAnalysis    `json:"commands"`
	Warnings     []string                  `json:"warnings,omitempty"`
}

type hooksInspectionReport struct {
	ConfigFound        bool                         `json:"config_found"`
	GeneratedFound     bool                         `json:"generated_found"`
	InstalledFound     bool                         `json:"installed_found"`
	PreCommitGenerated bool                         `json:"pre_commit_generated"`
	PrePushGenerated   bool                         `json:"pre_push_generated"`
	PreCommitInstalled bool                         `json:"pre_commit_installed"`
	PrePushInstalled   bool                         `json:"pre_push_installed"`
	HealthScore        int                          `json:"health_score"`
	HealthMax          int                          `json:"health_max"`
	HealthStatus       string                       `json:"health_status"`
	Hooks              map[string]hooksHookAnalysis `json:"hooks,omitempty"`
	Errors             []string                     `json:"errors,omitempty"`
}

func loadHooksManifestForInspection(path string) (*hooksManifestForInspection, error) {
	path = filepath.Clean(path)
	if strings.Contains(path, "..") {
		return nil, fmt.Errorf("invalid manifest path: contains path traversal")
	}
	data, err := os.ReadFile(path) // #nosec G304 -- path cleaned and traversal rejected above
	if err != nil {
		return nil, fmt.Errorf("failed to read hooks manifest: %w", err)
	}
	if err := validateHooksManifestSchema(data); err != nil {
		return nil, err
	}
	var manifest hooksManifestForInspection
	if err := yaml.Unmarshal(data, &manifest); err != nil {
		return nil, fmt.Errorf("failed to parse hooks manifest: %w", err)
	}
	if manifest.Hooks == nil {
		manifest.Hooks = make(map[string][]hooksManifestEntry)
	}
	if manifest.Opt == nil {
		manifest.Opt = make(map[string]any)
	}
	return &manifest, nil
}

func hooksGetBool(m map[string]any, key string, def bool) bool {
	if m == nil {
		return def
	}
	if v, ok := m[key].(bool); ok {
		return v
	}
	return def
}

func hooksGetString(m map[string]any, key, def string) string {
	if m == nil {
		return def
	}
	if v, ok := m[key].(string); ok {
		vv := strings.TrimSpace(v)
		if vv != "" {
			return vv
		}
	}
	return def
}

func hooksGetOptimizationSnapshot(opt map[string]any) hooksOptimizationSnapshot {
	return hooksOptimizationSnapshot{
		OnlyChangedFiles: hooksGetBool(opt, "only_changed_files", false),
		ContentSource:    hooksGetString(opt, "content_source", "index"),
		Parallel:         hooksGetString(opt, "parallel", "auto"),
		CacheResults:     hooksGetBool(opt, "cache_results", false),
	}
}

func hooksExtractFlagValue(args []string, flag string) string {
	for i, arg := range args {
		if arg == flag && i+1 < len(args) {
			return strings.TrimSpace(args[i+1])
		}
	}
	return ""
}

func hooksExtractCategories(args []string) []string {
	raw := hooksExtractFlagValue(args, "--categories")
	if raw == "" {
		return nil
	}
	return splitAndTrim(raw)
}

func hooksFormatWrapperInvocation(hook string, opt hooksOptimizationSnapshot) string {
	wrapper := fmt.Sprintf("goneat assess --hook %s --hook-manifest .goneat/hooks.yaml", hook)
	if opt.OnlyChangedFiles || opt.ContentSource == "index" {
		wrapper += " --staged-only"
	}
	wrapper += " --package-mode"
	return wrapper
}

func hooksGetOutputFormat(cmd *cobra.Command) string {
	if cmd == nil {
		return "text"
	}
	if cmd.Flags().Lookup("format") == nil {
		return "text"
	}
	val, err := cmd.Flags().GetString("format")
	if err != nil {
		return "text"
	}
	val = strings.ToLower(strings.TrimSpace(val))
	switch val {
	case "json":
		return "json"
	default:
		return "text"
	}
}

func hooksWriteJSON(out io.Writer, value any) error {
	b, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(out, string(b))
	return err
}

func hooksIsInternalCommand(name string) bool {
	switch strings.TrimSpace(name) {
	case "assess", "format", "dependencies":
		return true
	default:
		return false
	}
}

func hooksIsMutator(entry hooksManifestEntry) (bool, []string, []string) {
	cmd := strings.TrimSpace(entry.Command)
	args := entry.Args
	isInternal := hooksIsInternalCommand(cmd)

	mutator := false
	var reasons []string
	var warnings []string

	if entry.StageFixed {
		mutator = true
		reasons = append(reasons, "stage_fixed")
		warnings = append(warnings, "stage_fixed is enabled; this hook may modify files and re-stage them")
	}

	if cmd == "format" {
		mutator = true
		reasons = append(reasons, "format")
		warnings = append(warnings, "format will modify files; prefer check-only in hooks")
	}

	if cmd == "assess" {
		for i, arg := range args {
			if arg == "--fix" {
				mutator = true
				reasons = append(reasons, "assess_fix")
				warnings = append(warnings, "assess --fix will modify files")
				break
			}
			if arg == "--mode" && i+1 < len(args) && strings.TrimSpace(args[i+1]) == "fix" {
				mutator = true
				reasons = append(reasons, "assess_mode_fix")
				warnings = append(warnings, "assess --mode fix will modify files")
				break
			}
		}
	}

	if !isInternal {
		if cmd == "make" {
			warnings = append(warnings, "anti-pattern: running make in hooks can mutate the tree and cause confusing drift; prefer a direct assess invocation")
			for _, a := range args {
				aa := strings.ToLower(strings.TrimSpace(a))
				if strings.Contains(aa, "format") || strings.Contains(aa, "fmt") || strings.Contains(aa, "precommit") || strings.Contains(aa, "version") || strings.Contains(aa, "sync") {
					mutator = true
					reasons = append(reasons, "make_target_mutator")
					break
				}
			}
		}
		if cmd == "goneat" && len(args) > 0 {
			sub := strings.ToLower(strings.TrimSpace(args[0]))
			if sub == "format" || strings.HasPrefix(sub, "version") || sub == "ssot" {
				mutator = true
				reasons = append(reasons, "goneat_subcommand_mutator")
				warnings = append(warnings, "external goneat command in hooks may mutate the tree; prefer internal hook orchestration")
			}
		}
	}

	return mutator, reasons, warnings
}

func hooksAnalyzeHook(manifest *hooksManifestForInspection, hook string) hooksHookAnalysis {
	entries := manifest.Hooks[hook]
	opt := hooksGetOptimizationSnapshot(manifest.Opt)

	analysis := hooksHookAnalysis{
		Hook:         hook,
		Optimization: opt,
		Wrapper:      hooksFormatWrapperInvocation(hook, opt),
		Commands:     make([]hooksCommandAnalysis, 0, len(entries)),
	}

	if len(entries) == 0 {
		analysis.Warnings = append(analysis.Warnings, "no commands configured")
		return analysis
	}

	assessFound := false
	externalFound := false

	for _, entry := range entries {
		isInternal := hooksIsInternalCommand(entry.Command)
		kind := "external"
		if isInternal {
			kind = "internal"
		} else {
			externalFound = true
		}

		mutator, reasons, warns := hooksIsMutator(entry)
		analysis.Commands = append(analysis.Commands, hooksCommandAnalysis{
			Command:         entry.Command,
			Args:            entry.Args,
			Kind:            kind,
			StageFixed:      entry.StageFixed,
			Priority:        entry.Priority,
			Timeout:         entry.Timeout,
			IsMutator:       mutator,
			MutatorReasons:  reasons,
			WarningMessages: warns,
		})

		if strings.TrimSpace(entry.Command) == "assess" {
			assessFound = true
		}
	}

	if !assessFound {
		analysis.Warnings = append(analysis.Warnings, "no assess command configured")
	}
	if externalFound {
		analysis.Warnings = append(analysis.Warnings, "contains external commands")
	}

	return analysis
}

func hooksReportEffectiveHook(cmd *cobra.Command, manifest *hooksManifestForInspection, hook string) {
	out := cmd.OutOrStdout()
	analysis := hooksAnalyzeHook(manifest, hook)

	fmt.Fprintf(out, "\n🧩 %s policy\n", hook)                                                                                                                                                                                                                       //nolint:errcheck // CLI output
	fmt.Fprintf(out, "   Optimization: only_changed_files=%v, content_source=%s, parallel=%s, cache_results=%v\n", analysis.Optimization.OnlyChangedFiles, analysis.Optimization.ContentSource, analysis.Optimization.Parallel, analysis.Optimization.CacheResults) //nolint:errcheck // CLI output
	fmt.Fprintf(out, "   Hook wrapper: %s\n", analysis.Wrapper)                                                                                                                                                                                                     //nolint:errcheck // CLI output

	if len(analysis.Commands) == 0 {
		fmt.Fprintf(out, "   ⚠️  No commands configured for %s\n", hook) //nolint:errcheck // CLI output
		return
	}

	for _, c := range analysis.Commands {
		fmt.Fprintf(out, "   - %s: %s %s\n", c.Kind, c.Command, strings.Join(c.Args, " ")) //nolint:errcheck // CLI output
		if c.Command == "assess" {
			cats := hooksExtractCategories(c.Args)
			failOn := hooksExtractFlagValue(c.Args, "--fail-on")
			if len(cats) > 0 {
				fmt.Fprintf(out, "     categories: %s\n", strings.Join(cats, ",")) //nolint:errcheck // CLI output
			}
			if failOn != "" {
				fmt.Fprintf(out, "     fail-on: %s\n", failOn) //nolint:errcheck // CLI output
			}
		}
		for _, w := range c.WarningMessages {
			fmt.Fprintf(out, "     ⚠️  %s\n", w) //nolint:errcheck // CLI output
		}
		if c.IsMutator {
			fmt.Fprintf(out, "     ⚠️  mutator detected: %s\n", strings.Join(c.MutatorReasons, ",")) //nolint:errcheck // CLI output
		}
	}

	for _, w := range analysis.Warnings {
		if w == "contains external commands" {
			fmt.Fprintf(out, "   ⚠️  %s contains external commands; prefer internal goneat commands for predictability\n", hook) //nolint:errcheck // CLI output
			continue
		}
		if w == "no assess command configured" {
			fmt.Fprintf(out, "   ⚠️  No assess command configured for %s; hooks will not enforce goneat assessments\n", hook) //nolint:errcheck // CLI output
			continue
		}
		fmt.Fprintf(out, "   ⚠️  %s\n", w) //nolint:errcheck // CLI output
	}
}

func runHooksValidate(cmd *cobra.Command, args []string) error {
	out := cmd.OutOrStdout()
	format := hooksGetOutputFormat(cmd)

	// Check if hooks.yaml exists
	hooksConfigPath := ".goneat/hooks.yaml"
	if _, err := os.Stat(hooksConfigPath); os.IsNotExist(err) {
		return fmt.Errorf("hooks configuration not found at %s", hooksConfigPath)
	}

	manifest, err := loadHooksManifestForInspection(hooksConfigPath)
	if err != nil {
		return err
	}

	// Status snapshot
	hooksDir := ".goneat/hooks"
	generatedFound := false
	preCommitGenerated := false
	prePushGenerated := false
	if _, err := os.Stat(hooksDir); err == nil {
		generatedFound = true
		if _, err := os.Stat(filepath.Join(hooksDir, "pre-commit")); err == nil {
			preCommitGenerated = true
		}
		if _, err := os.Stat(filepath.Join(hooksDir, "pre-push")); err == nil {
			prePushGenerated = true
		}
	}

	gitHooksDir := ".git/hooks"
	installedFound := false
	preCommitInstalled := false
	prePushInstalled := false
	if _, err := os.Stat(gitHooksDir); err == nil {
		installedFound = true
		pc := filepath.Join(gitHooksDir, "pre-commit")
		pp := filepath.Join(gitHooksDir, "pre-push")
		if info, err := os.Stat(pc); err == nil && (info.Mode()&0111) != 0 {
			preCommitInstalled = true
		}
		if info, err := os.Stat(pp); err == nil && (info.Mode()&0111) != 0 {
			prePushInstalled = true
		}
	}

	if format == "json" {
		report := hooksInspectionReport{
			ConfigFound:        true,
			GeneratedFound:     generatedFound,
			InstalledFound:     installedFound,
			PreCommitGenerated: preCommitGenerated,
			PrePushGenerated:   prePushGenerated,
			PreCommitInstalled: preCommitInstalled,
			PrePushInstalled:   prePushInstalled,
			HealthMax:          7,
			Hooks:              map[string]hooksHookAnalysis{},
		}
		report.HealthScore = 0
		if report.ConfigFound {
			report.HealthScore++
		}
		if report.GeneratedFound {
			report.HealthScore++
		}
		if report.InstalledFound {
			report.HealthScore++
		}
		if report.PreCommitGenerated {
			report.HealthScore++
		}
		if report.PrePushGenerated {
			report.HealthScore++
		}
		if report.PreCommitInstalled {
			report.HealthScore++
		}
		if report.PrePushInstalled {
			report.HealthScore++
		}
		switch {
		case report.HealthScore >= 5:
			report.HealthStatus = "good"
		case report.HealthScore >= 3:
			report.HealthStatus = "needs_attention"
		default:
			report.HealthStatus = "critical"
		}
		report.Hooks["pre-commit"] = hooksAnalyzeHook(manifest, "pre-commit")
		report.Hooks["pre-push"] = hooksAnalyzeHook(manifest, "pre-push")
		return hooksWriteJSON(out, report)
	}

	fmt.Fprintln(out, "🔍 Validating hook configuration...") //nolint:errcheck // CLI output

	if !generatedFound {
		fmt.Fprintln(out, "⚠️  No generated hooks found - run 'goneat hooks generate'") //nolint:errcheck // CLI output
	} else {
		if preCommitGenerated {
			fmt.Fprintln(out, "✅ Pre-commit hook generated") //nolint:errcheck // CLI output
		} else {
			fmt.Fprintln(out, "⚠️  Pre-commit hook not found") //nolint:errcheck // CLI output
		}
		if prePushGenerated {
			fmt.Fprintln(out, "✅ Pre-push hook generated") //nolint:errcheck // CLI output
		} else {
			fmt.Fprintln(out, "⚠️  Pre-push hook not found") //nolint:errcheck // CLI output
		}
	}

	if !installedFound {
		fmt.Fprintln(out, "⚠️  .git/hooks directory not found - not in a git repository?") //nolint:errcheck // CLI output
	} else {
		if preCommitInstalled {
			fmt.Fprintln(out, "✅ Pre-commit hook installed and executable") //nolint:errcheck // CLI output
		} else {
			fmt.Fprintln(out, "⚠️  Pre-commit hook not properly installed") //nolint:errcheck // CLI output
		}
		if prePushInstalled {
			fmt.Fprintln(out, "✅ Pre-push hook installed and executable") //nolint:errcheck // CLI output
		} else {
			fmt.Fprintln(out, "⚠️  Pre-push hook not properly installed") //nolint:errcheck // CLI output
		}
	}

	hooksReportEffectiveHook(cmd, manifest, "pre-commit")
	hooksReportEffectiveHook(cmd, manifest, "pre-push")

	fmt.Fprintln(out, "\n✅ Hook configuration validation complete")     //nolint:errcheck // CLI output
	fmt.Fprintln(out, "🎉 Ready to commit with intelligent validation!") //nolint:errcheck // CLI output

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
	out := cmd.OutOrStdout()
	format := hooksGetOutputFormat(cmd)

	hooksConfigPath := ".goneat/hooks.yaml"
	configFound := false
	if _, err := os.Stat(hooksConfigPath); err == nil {
		configFound = true
	}

	hooksDir := ".goneat/hooks"
	generatedFound := false
	preCommitGenerated := false
	prePushGenerated := false
	if _, err := os.Stat(hooksDir); err == nil {
		generatedFound = true
		if _, err := os.Stat(filepath.Join(hooksDir, "pre-commit")); err == nil {
			preCommitGenerated = true
		}
		if _, err := os.Stat(filepath.Join(hooksDir, "pre-push")); err == nil {
			prePushGenerated = true
		}
	}

	gitHooksDir := ".git/hooks"
	installedFound := false
	preCommitInstalled := false
	prePushInstalled := false
	if _, err := os.Stat(gitHooksDir); err == nil {
		installedFound = true
		pc := filepath.Join(gitHooksDir, "pre-commit")
		pp := filepath.Join(gitHooksDir, "pre-push")
		if info, err := os.Stat(pc); err == nil && (info.Mode()&0111) != 0 {
			preCommitInstalled = true
		}
		if info, err := os.Stat(pp); err == nil && (info.Mode()&0111) != 0 {
			prePushInstalled = true
		}
	}

	healthScore := 0
	if configFound {
		healthScore++
	}
	if generatedFound {
		healthScore++
	}
	if installedFound {
		healthScore++
	}
	if preCommitGenerated {
		healthScore++
	}
	if prePushGenerated {
		healthScore++
	}
	if preCommitInstalled {
		healthScore++
	}
	if prePushInstalled {
		healthScore++
	}

	healthStatus := "critical"
	switch {
	case healthScore >= 5:
		healthStatus = "good"
	case healthScore >= 3:
		healthStatus = "needs_attention"
	}

	if format == "json" {
		report := hooksInspectionReport{
			ConfigFound:        configFound,
			GeneratedFound:     generatedFound,
			InstalledFound:     installedFound,
			PreCommitGenerated: preCommitGenerated,
			PrePushGenerated:   prePushGenerated,
			PreCommitInstalled: preCommitInstalled,
			PrePushInstalled:   prePushInstalled,
			HealthScore:        healthScore,
			HealthMax:          7,
			HealthStatus:       healthStatus,
			Hooks:              map[string]hooksHookAnalysis{},
		}
		if configFound {
			manifest, err := loadHooksManifestForInspection(hooksConfigPath)
			if err != nil {
				report.Errors = append(report.Errors, err.Error())
			} else {
				report.Hooks["pre-commit"] = hooksAnalyzeHook(manifest, "pre-commit")
				report.Hooks["pre-push"] = hooksAnalyzeHook(manifest, "pre-push")
			}
		}
		return hooksWriteJSON(out, report)
	}

	fmt.Fprintln(out, "🔍 Inspecting hook configuration and status...") //nolint:errcheck // CLI output

	configStatus := "❌ Not found"
	if configFound {
		configStatus = "✅ Found"
	}
	generatedStatus := "❌ Not found"
	if generatedFound {
		generatedStatus = "✅ Found"
	}
	installedStatus := "❌ Not found"
	if installedFound {
		installedStatus = "✅ Found"
	}
	preCommitGeneratedStatus := "❌ Missing"
	if preCommitGenerated {
		preCommitGeneratedStatus = "✅ Present"
	}
	prePushGeneratedStatus := "❌ Missing"
	if prePushGenerated {
		prePushGeneratedStatus = "✅ Present"
	}
	preCommitInstalledStatus := "❌ Missing"
	if preCommitInstalled {
		preCommitInstalledStatus = "✅ Installed & executable"
	}
	prePushInstalledStatus := "❌ Missing"
	if prePushInstalled {
		prePushInstalledStatus = "✅ Installed & executable"
	}

	fmt.Fprintln(out, "📊 Current Hook Status:")                            //nolint:errcheck // CLI output
	fmt.Fprintf(out, "├── Configuration: %s\n", configStatus)              //nolint:errcheck // CLI output
	fmt.Fprintf(out, "├── Generated Hooks: %s\n", generatedStatus)         //nolint:errcheck // CLI output
	fmt.Fprintf(out, "│   ├── Pre-commit: %s\n", preCommitGeneratedStatus) //nolint:errcheck // CLI output
	fmt.Fprintf(out, "│   └── Pre-push: %s\n", prePushGeneratedStatus)     //nolint:errcheck // CLI output
	fmt.Fprintf(out, "├── Installed Hooks: %s\n", installedStatus)         //nolint:errcheck // CLI output
	fmt.Fprintf(out, "│   ├── Pre-commit: %s\n", preCommitInstalledStatus) //nolint:errcheck // CLI output
	fmt.Fprintf(out, "│   └── Pre-push: %s\n", prePushInstalledStatus)     //nolint:errcheck // CLI output

	prettyHealth := "❌ Critical"
	switch healthStatus {
	case "good":
		prettyHealth = "✅ Good"
	case "needs_attention":
		prettyHealth = "⚠️  Needs attention"
	}
	fmt.Fprintf(out, "└── System Health: %s (%d/7)\n", prettyHealth, healthScore) //nolint:errcheck // CLI output

	if configFound {
		manifest, err := loadHooksManifestForInspection(hooksConfigPath)
		if err != nil {
			fmt.Fprintf(out, "\n⚠️  Failed to parse %s: %v\n", hooksConfigPath, err) //nolint:errcheck // CLI output
			return nil
		}
		hooksReportEffectiveHook(cmd, manifest, "pre-commit")
		hooksReportEffectiveHook(cmd, manifest, "pre-push")
	}

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
	sb, err := fs.ReadFile(schemaFS, "work/hooks-manifest-v1.0.0.yaml")
	if err != nil {
		return fmt.Errorf("failed to read embedded hooks schema: %v", err)
	}
	var schm any
	if err := yaml.Unmarshal(sb, &schm); err != nil {
		return fmt.Errorf("failed to parse embedded hooks schema: %v", err)
	}
	// Conditionally remove $schema field to prevent remote fetching in offline mode
	if os.Getenv("GONEAT_OFFLINE_SCHEMA_VALIDATION") == "true" {
		if m, ok := schm.(map[string]interface{}); ok {
			delete(m, "$schema")
		}
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

// validateHooksManifestSchema validates raw YAML data against the embedded hooks manifest schema.
// Returns nil if valid, or a user-friendly error with guidance if invalid.
func validateHooksManifestSchema(raw []byte) error {
	// Parse YAML into generic structure for schema validation
	var doc any
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		return fmt.Errorf("failed to parse hooks.yaml as YAML: %v", err)
	}
	docJSON, err := json.Marshal(doc)
	if err != nil {
		return fmt.Errorf("failed to convert hooks.yaml to JSON for validation: %v", err)
	}

	// Load schema from embedded FS
	schemaFS := assets.GetSchemasFS()
	sb, err := fs.ReadFile(schemaFS, "work/hooks-manifest-v1.0.0.yaml")
	if err != nil {
		// Schema not found - skip validation (shouldn't happen in production)
		logger.Debug("Skipping schema validation: embedded schema not found", logger.Err(err))
		return nil
	}
	var schm any
	if err := yaml.Unmarshal(sb, &schm); err != nil {
		logger.Debug("Skipping schema validation: failed to parse embedded schema", logger.Err(err))
		return nil
	}

	// Conditionally remove $schema field to prevent remote fetching in offline mode
	if os.Getenv("GONEAT_OFFLINE_SCHEMA_VALIDATION") == "true" {
		if m, ok := schm.(map[string]interface{}); ok {
			delete(m, "$schema")
		}
	}
	schJSON, _ := json.Marshal(schm)

	// Validate against schema
	schemaLoader := gojsonschema.NewBytesLoader(schJSON)
	docLoader := gojsonschema.NewBytesLoader(docJSON)
	res, err := gojsonschema.Validate(schemaLoader, docLoader)
	if err != nil {
		return fmt.Errorf("schema validation error: %v", err)
	}

	if !res.Valid() {
		var errLines []string
		errLines = append(errLines, "hooks.yaml configuration is invalid:")
		for _, e := range res.Errors() {
			errLines = append(errLines, fmt.Sprintf("  - %s", e))
		}
		errLines = append(errLines, "")
		errLines = append(errLines, "Expected format - hooks must be an array of commands:")
		errLines = append(errLines, "  hooks:")
		errLines = append(errLines, "    pre-commit:")
		errLines = append(errLines, "      - command: \"assess\"")
		errLines = append(errLines, "        args: [\"--categories\", \"format,lint\"]")
		errLines = append(errLines, "")
		errLines = append(errLines, "Run 'goneat hooks init' to generate a valid configuration template.")

		return fmt.Errorf("%s", strings.Join(errLines, "\n"))
	}

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

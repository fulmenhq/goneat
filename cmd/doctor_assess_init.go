package cmd

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/fulmenhq/goneat/internal/assets"
	"github.com/fulmenhq/goneat/internal/doctor"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var doctorAssessCmd = &cobra.Command{
	Use:   "assess",
	Short: "Assess configuration helpers",
	Long: `Helpers for working with assessment configuration.

This command group focuses on .goneat/assess.yaml, which provides repo-specific overrides
for assess lint integrations (shell, make, workflows, etc.).`,
}

var doctorAssessInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize .goneat/assess.yaml with recommended defaults",
	Long: `Initialize .goneat/assess.yaml by seeding it with recommended lint settings.

This command:
- Detects your repository type (Go, Python, TypeScript, Rust, C#, or unknown)
- Selects an appropriate starter template
- Writes .goneat/assess.yaml

Examples:
  goneat doctor assess init                  # Auto-detect repo type
  goneat doctor assess init --language go    # Force Go template
  goneat doctor assess init --force          # Overwrite existing file`,
	RunE: runDoctorAssessInit,
}

var (
	assessInitLanguage string
	assessInitForce    bool
)

func init() {
	doctorCmd.AddCommand(doctorAssessCmd)
	doctorAssessCmd.AddCommand(doctorAssessInitCmd)

	doctorAssessInitCmd.Flags().StringVar(&assessInitLanguage, "language", "", "Force language detection (go, python, typescript, rust, csharp)")
	doctorAssessInitCmd.Flags().BoolVar(&assessInitForce, "force", false, "Overwrite existing .goneat/assess.yaml")
}

func runDoctorAssessInit(cmd *cobra.Command, _ []string) error {
	configPath := filepath.Clean(".goneat/assess.yaml")

	// Check if file already exists
	if _, err := os.Stat(configPath); err == nil && !assessInitForce {
		return fmt.Errorf("%s already exists. Use --force to overwrite", configPath)
	}

	language := strings.TrimSpace(assessInitLanguage)
	if language == "" {
		repoType := doctor.DetectCurrentRepoType()
		language = repoType.String()
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "🔍 Detected repository type: %s\n", language)
	} else {
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "🎯 Using forced language: %s\n", language)
	}

	templatePath := assessTemplatePathForLanguage(language)

	templatesFS := assets.GetTemplatesFS()
	data, err := fs.ReadFile(templatesFS, templatePath)
	if err != nil {
		return fmt.Errorf("failed to read embedded assess template %s: %w", templatePath, err)
	}

	// Ensure .goneat directory exists
	if err := os.MkdirAll(".goneat", 0750); err != nil {
		return fmt.Errorf("failed to create .goneat directory: %w", err)
	}

	// Basic validation: ensure template is valid YAML
	var decoded map[string]any
	if err := yaml.Unmarshal(data, &decoded); err != nil {
		return fmt.Errorf("embedded template %s is invalid YAML: %w", templatePath, err)
	}

	// Write file with secure permissions
	if err := os.WriteFile(configPath, data, 0600); err != nil {
		return fmt.Errorf("failed to write %s: %w", configPath, err)
	}

	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "\n✨ Successfully created %s\n", configPath)
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "   Template: %s\n", templatePath)
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "\n📋 Next steps:\n")
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "   1. Review %s and customize as needed\n", configPath)
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "   2. Run: goneat assess --categories lint --fail-on high\n")
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "   3. If using hooks: goneat hooks validate\n")

	return nil
}

func assessTemplatePathForLanguage(language string) string {
	lang := strings.ToLower(strings.TrimSpace(language))
	switch lang {
	case "go", "python", "typescript", "rust", "unknown":
		return fmt.Sprintf("templates/assess/%s.yaml", lang)
	case "csharp":
		// Until we have C#-specific lint integrations, use the generic defaults.
		return "templates/assess/unknown.yaml"
	default:
		return "templates/assess/unknown.yaml"
	}
}

/*
Copyright © 2025 3 Leaps <info@3leaps.net>
*/

package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"github.com/fulmenhq/goneat/internal/ops"
	"github.com/fulmenhq/goneat/pkg/config"
	"github.com/fulmenhq/goneat/pkg/dependencies"
	"github.com/fulmenhq/goneat/pkg/sbom"
	"github.com/spf13/cobra"
)

// dependenciesCmd represents the dependencies command
var dependenciesCmd = &cobra.Command{
	Use:   "dependencies",
	Short: "Dependency policy enforcement and analysis",
	Long:  `Analyze dependencies for license compliance, cooling policy, and generate SBOMs.`,
	RunE:  runDependencies,
}

func init() {
	rootCmd.AddCommand(dependenciesCmd)

	// Register command in ops registry with taxonomy
	capabilities := ops.GetDefaultCapabilities(ops.GroupNeat, ops.CategoryDependencies)
	if err := ops.RegisterCommandWithTaxonomy("dependencies", ops.GroupNeat, ops.CategoryDependencies, capabilities, dependenciesCmd, "Dependency policy enforcement and analysis"); err != nil {
		panic(fmt.Sprintf("failed to register dependencies command: %v", err))
	}

	// Capability flags
	dependenciesCmd.Flags().Bool("licenses", false, "Run license compliance checks")
	dependenciesCmd.Flags().Bool("cooling", false, "Check package cooling policy")
	dependenciesCmd.Flags().Bool("sbom", false, "Generate SBOM artifact")

	// Policy and output
	dependenciesCmd.Flags().String("policy", ".goneat/dependencies.yaml", "Policy file path")
	dependenciesCmd.Flags().String("format", "json", "Output format (json, markdown, html)")
	dependenciesCmd.Flags().String("output", "", "Output file (default: stdout)")

	// SBOM-specific
	dependenciesCmd.Flags().String("sbom-format", "cyclonedx-json", "SBOM format (cyclonedx-json)")
	dependenciesCmd.Flags().String("sbom-output", "", "SBOM output file path (default: sbom/goneat-<timestamp>.cdx.json)")
	dependenciesCmd.Flags().Bool("sbom-stdout", false, "Output SBOM to stdout instead of file")
	dependenciesCmd.Flags().String("sbom-platform", "", "Target platform for SBOM (e.g., linux/amd64)")

	// Failure controls
	dependenciesCmd.Flags().String("fail-on", "critical", "Fail on severity (critical, high, medium, low)")
}

func shouldFailDependencies(result *dependencies.AnalysisResult, failOn string) bool {
	if !result.Passed && failOn == "any" {
		return true
	}

	// Map severity levels to numeric values for comparison
	severityLevels := map[string]int{
		"critical": 4,
		"high":     3,
		"medium":   2,
		"low":      1,
	}

	threshold, ok := severityLevels[failOn]
	if !ok {
		// Invalid threshold, treat as "any"
		return !result.Passed
	}

	// Check if any issues meet or exceed the threshold
	for _, issue := range result.Issues {
		issueLevel, exists := severityLevels[issue.Severity]
		if exists && issueLevel >= threshold {
			return true
		}
	}

	return false
}

func runDependencies(cmd *cobra.Command, args []string) error {
	licensesFlag, _ := cmd.Flags().GetBool("licenses")
	coolingFlag, _ := cmd.Flags().GetBool("cooling")
	sbomFlag, _ := cmd.Flags().GetBool("sbom")

	runAnalysis := licensesFlag || coolingFlag
	runSBOM := sbomFlag

	if !runAnalysis && !runSBOM {
		if err := cmd.Help(); err != nil {
			return fmt.Errorf("failed to show help: %w", err)
		}
		return nil
	}

	if runAnalysis {
		// Determine target directory (default to current directory)
		target := "."
		if len(args) > 0 {
			target = args[0]
		}

		cfg, err := config.LoadProjectConfig()
		if err != nil {
			return err
		}
		depsCfg := cfg.GetDependenciesConfig()

		policyPath, _ := cmd.Flags().GetString("policy")
		if policyPath == "" {
			policyPath = depsCfg.PolicyPath
		}

		analyzer := dependencies.NewGoAnalyzer()
		detector := dependencies.NewDetector(&depsCfg)

		lang, _, err := detector.Detect(target)
		if err != nil {
			return err
		}
		if lang == "" {
			return errors.New("no supported language detected")
		}

		analysisConfig := dependencies.AnalysisConfig{
			PolicyPath:    policyPath,
			EngineType:    depsCfg.Engine.Type,
			Languages:     []dependencies.Language{lang},
			Target:        target,
			CheckLicenses: licensesFlag,
			CheckCooling:  coolingFlag,
			Config:        &depsCfg,
		}

		result, err := analyzer.Analyze(context.Background(), target, analysisConfig)
		if err != nil {
			return err
		}
		if result.Dependencies == nil {
			result.Dependencies = []dependencies.Dependency{}
		}
		if result.Issues == nil {
			result.Issues = []dependencies.Issue{}
		}

		// Analyzer now handles all policy checks internally

		// Output
		output, _ := cmd.Flags().GetString("output")
		format, _ := cmd.Flags().GetString("format")
		if output != "" {
			// Write to file
			data, err := json.MarshalIndent(result, "", "  ")
			if err != nil {
				return fmt.Errorf("failed to marshal result: %w", err)
			}
			if err := os.WriteFile(output, data, 0600); err != nil {
				return fmt.Errorf("failed to write output file: %w", err)
			}
		} else {
			// Print to stdout
			switch format {
			case "json":
				if err := json.NewEncoder(os.Stdout).Encode(result); err != nil {
					return fmt.Errorf("failed to encode JSON: %w", err)
				}
			case "markdown":
				// TODO: Markdown output
				fmt.Printf("Dependencies: %d, Passed: %t\n", len(result.Dependencies), result.Passed)
			default:
				fmt.Printf("Dependencies: %d, Passed: %t\n", len(result.Dependencies), result.Passed)
			}
		}

		failOn, _ := cmd.Flags().GetString("fail-on")
		if shouldFailDependencies(result, failOn) {
			return errors.New("analysis failed")
		}
	}

	if runSBOM {
		target := "."
		if len(args) > 0 {
			target = args[0]
		}

		invoker, err := sbom.NewSyftInvoker()
		if err != nil {
			return fmt.Errorf("failed to initialize SBOM generator: %w\n\n"+
				"To install syft, run:\n"+
				"  goneat doctor tools --scope sbom --install --yes\n\n"+
				"Or install syft manually from: https://github.com/anchore/syft#installation", err)
		}

		sbomFormat, _ := cmd.Flags().GetString("sbom-format")
		sbomOutput, _ := cmd.Flags().GetString("sbom-output")
		sbomStdout, _ := cmd.Flags().GetBool("sbom-stdout")
		sbomPlatform, _ := cmd.Flags().GetString("sbom-platform")

		sbomConfig := sbom.Config{
			TargetPath: target,
			OutputPath: sbomOutput,
			Format:     sbomFormat,
			Stdout:     sbomStdout,
			Platform:   sbomPlatform,
		}

		result, err := invoker.Generate(context.Background(), sbomConfig)
		if err != nil {
			return fmt.Errorf("SBOM generation failed: %w", err)
		}

		if sbomStdout {
			fmt.Println(string(result.SBOMContent))
		} else {
			fmt.Printf("✅ SBOM generated: %s\n", result.OutputPath)
			fmt.Printf("   Format: %s\n", result.Format)
			fmt.Printf("   Packages: %d\n", result.PackageCount)
			fmt.Printf("   Tool Version: %s\n", result.ToolVersion)
			fmt.Printf("   Duration: %v\n", result.Duration)
			if result.DependencyGraph != nil {
				fmt.Printf("   Dependency Nodes: %d (roots: %d)\n", len(result.DependencyGraph.Nodes), len(result.DependencyGraph.Roots))
			}
		}
	}

	return nil
}

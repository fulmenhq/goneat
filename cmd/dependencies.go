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
	dependenciesCmd.Flags().String("sbom-format", "cyclonedx", "SBOM format (cyclonedx, spdx)")
	dependenciesCmd.Flags().Bool("sbom-enrich", false, "Enrich SBOM with vulnerability data")

	// Failure controls
	dependenciesCmd.Flags().String("fail-on", "critical", "Fail on severity (critical, high, medium, low)")
}

func runDependencies(cmd *cobra.Command, args []string) error {
	licensesFlag, _ := cmd.Flags().GetBool("licenses")
	coolingFlag, _ := cmd.Flags().GetBool("cooling")
	sbomFlag, _ := cmd.Flags().GetBool("sbom")

	if licensesFlag || coolingFlag {
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
			PolicyPath: policyPath,
			EngineType: depsCfg.Engine.Type,
			Languages:  []dependencies.Language{lang},
			Target:     target,
		}

		result, err := analyzer.Analyze(context.Background(), target, analysisConfig)
		if err != nil {
			return err
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
			if err := os.WriteFile(output, data, 0644); err != nil {
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
		if !result.Passed && failOn == "any" {
			return errors.New("analysis failed")
		}
	} else if sbomFlag {
		return fmt.Errorf("sbom not implemented in Wave 1")
	} else {
		if err := cmd.Help(); err != nil {
			return fmt.Errorf("failed to show help: %w", err)
		}
		return nil
	}

	return nil
}

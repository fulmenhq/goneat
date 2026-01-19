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
	"strings"
	"time"

	"github.com/fulmenhq/goneat/internal/ops"
	"github.com/fulmenhq/goneat/pkg/config"
	"github.com/fulmenhq/goneat/pkg/dependencies"
	"github.com/fulmenhq/goneat/pkg/logger"
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
	dependenciesCmd.Flags().Bool("vuln", false, "Generate vulnerability report (SBOM + grype)")

	// Policy and output
	dependenciesCmd.Flags().String("policy", ".goneat/dependencies.yaml", "Policy file path")
	dependenciesCmd.Flags().String("format", "text", "Output format (text, json, markdown, html)")
	dependenciesCmd.Flags().String("output", "", "Output file (default: stdout)")
	dependenciesCmd.Flags().Bool("quiet", false, "Suppress goneat logs (best-effort)")

	// SBOM-specific
	dependenciesCmd.Flags().String("sbom-format", "cyclonedx-json", "SBOM format (cyclonedx-json)")
	dependenciesCmd.Flags().String("sbom-output", "", "SBOM output file path (default: sbom/goneat-<timestamp>.cdx.json)")
	dependenciesCmd.Flags().String("sbom-input", "", "Use an existing SBOM file (skips syft) for vulnerability scanning")
	dependenciesCmd.Flags().Bool("sbom-stdout", false, "Output SBOM to stdout instead of file")
	dependenciesCmd.Flags().String("sbom-platform", "", "Target platform for SBOM (e.g., linux/amd64)")

	// Failure controls
	dependenciesCmd.Flags().String("fail-on", "critical", "Fail on severity (critical, high, medium, low)")
}

func renderDependenciesText(result *dependencies.AnalysisResult) string {
	var vulnInfo string
	issuesBySev := map[string]int{}
	for _, issue := range result.Issues {
		issuesBySev[issue.Severity]++
		if issue.Type == "vulnerability" && issue.Severity == "info" {
			vulnInfo = issue.Message
		}
	}

	lines := []string{}
	lines = append(lines, "Dependencies")
	lines = append(lines, "============")

	// Show analyzed dependencies count, or packages scanned (from SBOM) if only vuln scan ran
	if len(result.Dependencies) > 0 {
		lines = append(lines, fmt.Sprintf("Dependencies: %d", len(result.Dependencies)))
	} else if result.PackagesScanned > 0 {
		lines = append(lines, fmt.Sprintf("Packages scanned: %d", result.PackagesScanned))
	} else {
		lines = append(lines, "Dependencies: 0")
	}
	lines = append(lines, fmt.Sprintf("Passed: %t", result.Passed))
	if vulnInfo != "" {
		lines = append(lines, "")
		lines = append(lines, vulnInfo)
	}
	if len(result.Issues) > 0 {
		lines = append(lines, "")
		lines = append(lines, "Issues:")
		lines = append(lines, fmt.Sprintf("- critical: %d", issuesBySev["critical"]))
		lines = append(lines, fmt.Sprintf("- high: %d", issuesBySev["high"]))
		lines = append(lines, fmt.Sprintf("- medium: %d", issuesBySev["medium"]))
		lines = append(lines, fmt.Sprintf("- low: %d", issuesBySev["low"]))
		lines = append(lines, fmt.Sprintf("- info: %d", issuesBySev["info"]))
	}
	lines = append(lines, "")
	return strings.Join(lines, "\n")
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
	quiet, _ := cmd.Flags().GetBool("quiet")
	if quiet {
		// Best-effort: suppress goneat's own logs; external tool output may still appear.
		logJSON, _ := cmd.Flags().GetBool("json")
		noColor, _ := cmd.Flags().GetBool("no-color")
		noOp, _ := cmd.Flags().GetBool("no-op")
		_ = logger.Initialize(logger.Config{Level: logger.ErrorLevel, UseColor: !noColor, JSON: logJSON, Component: "goneat", NoOp: noOp})
	}

	licensesFlag, _ := cmd.Flags().GetBool("licenses")
	coolingFlag, _ := cmd.Flags().GetBool("cooling")
	sbomFlag, _ := cmd.Flags().GetBool("sbom")
	vulnFlag, _ := cmd.Flags().GetBool("vuln")

	runAnalysis := licensesFlag || coolingFlag
	runVuln := vulnFlag
	runSBOM := sbomFlag

	if !runAnalysis && !runVuln && !runSBOM {
		if err := cmd.Help(); err != nil {
			return fmt.Errorf("failed to show help: %w", err)
		}
		return nil
	}

	if runAnalysis || runVuln {
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

		result := &dependencies.AnalysisResult{
			Dependencies: []dependencies.Dependency{},
			Issues:       []dependencies.Issue{},
			Passed:       true,
		}

		if runAnalysis {
			detector := dependencies.NewDetector(&depsCfg)

			lang, _, err := detector.Detect(target)
			if err != nil {
				return err
			}
			if lang == "" {
				return errors.New("no supported language detected")
			}

			// Select the appropriate analyzer based on detected language
			var analyzer dependencies.Analyzer
			switch lang {
			case dependencies.LanguageRust:
				analyzer = dependencies.NewRustAnalyzer()
			case dependencies.LanguageGo:
				analyzer = dependencies.NewGoAnalyzer()
			case dependencies.LanguageTypeScript:
				analyzer = dependencies.NewTypeScriptAnalyzer()
			case dependencies.LanguagePython:
				analyzer = dependencies.NewPythonAnalyzer()
			case dependencies.LanguageCSharp:
				analyzer = dependencies.NewCSharpAnalyzer()
			default:
				return fmt.Errorf("no analyzer available for language: %s", lang)
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

			analysisResult, err := analyzer.Analyze(context.Background(), target, analysisConfig)
			if err != nil {
				return err
			}
			result = analysisResult
			if result.Dependencies == nil {
				result.Dependencies = []dependencies.Dependency{}
			}
			if result.Issues == nil {
				result.Issues = []dependencies.Issue{}
			}
		}

		// Vulnerability scan is orchestrated here (SBOM + grype) because it is language-agnostic.
		if runVuln {
			sbomInput, _ := cmd.Flags().GetString("sbom-input")
			vulnResult, vulnIssues, vErr := dependencies.RunVulnerabilityScan(context.Background(), target, policyPath, sbomInput, 10*time.Minute)
			if vErr != nil {
				return vErr
			}
			if vulnResult != nil {
				result.PackagesScanned = vulnResult.PackagesScanned
			}
			if len(vulnIssues) > 0 {
				result.Issues = append(result.Issues, vulnIssues...)
				for _, vi := range vulnIssues {
					if vi.Severity != "info" {
						result.Passed = false
					}
				}
			}
		}

		// Output
		output, _ := cmd.Flags().GetString("output")
		format, _ := cmd.Flags().GetString("format")
		if output != "" {
			// Write to file
			switch format {
			case "json":
				data, err := json.MarshalIndent(result, "", "  ")
				if err != nil {
					return fmt.Errorf("failed to marshal result: %w", err)
				}
				if err := os.WriteFile(output, data, 0600); err != nil {
					return fmt.Errorf("failed to write output file: %w", err)
				}
			default:
				if err := os.WriteFile(output, []byte(renderDependenciesText(result)), 0600); err != nil {
					return fmt.Errorf("failed to write output file: %w", err)
				}
			}
		} else {
			// Print to stdout
			switch format {
			case "json":
				if err := json.NewEncoder(os.Stdout).Encode(result); err != nil {
					return fmt.Errorf("failed to encode JSON: %w", err)
				}
			default:
				fmt.Print(renderDependenciesText(result))
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

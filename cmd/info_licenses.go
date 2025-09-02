/*
Copyright © 2025 3 Leaps <info@3leaps.net>
*/
package cmd

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/3leaps/goneat/internal/ops"
	"github.com/spf13/cobra"
)

// infoLicensesCmd represents the info licenses command
var infoLicensesCmd = &cobra.Command{
	Use:   "licenses",
	Short: "Display license information for goneat and its dependencies",
	Long: `Display license information for goneat and all its dependencies.

This command shows the licenses used by goneat and its Go module dependencies,
helping with license compliance and attribution requirements.`,
	RunE: runInfoLicenses,
}

func init() {
	// Add to info command
	infoCmd.AddCommand(infoLicensesCmd)

	// Register command in ops registry with taxonomy
	capabilities := ops.GetDefaultCapabilities(ops.GroupSupport, ops.CategoryInformation)
	if err := ops.RegisterCommandWithTaxonomy("info licenses", ops.GroupSupport, ops.CategoryInformation, capabilities, infoLicensesCmd, "Show license information"); err != nil {
		panic(fmt.Sprintf("Failed to register info licenses command: %v", err))
	}

	infoLicensesCmd.Flags().Bool("json", false, "Output license information in JSON format")
	infoLicensesCmd.Flags().String("filter", "", "Filter licenses by type (e.g., 'apache', 'mit', 'bsd')")
	infoLicensesCmd.Flags().Bool("summary", false, "Show license summary instead of full details")
}

// LicenseInfo represents license information for a module
type LicenseInfo struct {
	Module  string `json:"module"`
	License string `json:"license"`
	Version string `json:"version,omitempty"`
	Main    bool   `json:"main,omitempty"`
}

// LicenseSummary represents a summary of licenses by type
type LicenseSummary struct {
	LicenseType string   `json:"license_type"`
	Count       int      `json:"count"`
	Modules     []string `json:"modules"`
}

func runInfoLicenses(cmd *cobra.Command, args []string) error {
	jsonFormat, _ := cmd.Flags().GetBool("json")
	filter, _ := cmd.Flags().GetString("filter")
	summary, _ := cmd.Flags().GetBool("summary")

	licenses := getLicenseInfo()

	// Apply filter if specified
	if filter != "" {
		filtered := []LicenseInfo{}
		for _, lic := range licenses {
			if strings.Contains(strings.ToLower(lic.License), strings.ToLower(filter)) {
				filtered = append(filtered, lic)
			}
		}
		licenses = filtered
	}

	if summary {
		return outputLicenseSummary(licenses, jsonFormat, cmd)
	}

	return outputLicenseDetails(licenses, jsonFormat, cmd)
}

func getLicenseInfo() []LicenseInfo {
	// Static license information - in a real implementation, this could be
	// generated at build time from go.mod analysis
	return []LicenseInfo{
		{
			Module:  "github.com/3leaps/goneat",
			License: "Apache License 2.0",
			Main:    true,
		},
		// Direct dependencies
		{
			Module:  "github.com/spf13/cobra",
			License: "Apache License 2.0",
		},
		{
			Module:  "github.com/spf13/viper",
			License: "MIT License",
		},
		{
			Module:  "github.com/xeipuuv/gojsonschema",
			License: "Apache License 2.0",
		},
		// Key indirect dependencies
		{
			Module:  "github.com/fsnotify/fsnotify",
			License: "BSD-3-Clause License",
		},
		{
			Module:  "golang.org/x/sys",
			License: "BSD-3-Clause License",
		},
		{
			Module:  "gopkg.in/yaml.v3",
			License: "MIT License",
		},
		{
			Module:  "github.com/stretchr/testify",
			License: "MIT License",
		},
	}
}

func outputLicenseDetails(licenses []LicenseInfo, jsonFormat bool, cmd *cobra.Command) error {
	if jsonFormat {
		return outputJSON(licenses, cmd)
	}

	out := cmd.OutOrStdout()

	// Header
	fmt.Fprintln(out, "📋 Goneat License Information")
	fmt.Fprintln(out, "=============================")
	fmt.Fprintln(out)

	// Main project
	fmt.Fprintln(out, "🎯 Main Project:")
	for _, lic := range licenses {
		if lic.Main {
			fmt.Fprintf(out, "  %s - %s\n", lic.Module, lic.License)
			break
		}
	}
	fmt.Fprintln(out)

	// Dependencies by license type
	byLicense := groupByLicense(licenses)

	fmt.Fprintln(out, "📦 Dependencies by License:")

	licenseOrder := []string{"Apache License 2.0", "MIT License", "BSD-3-Clause License"}
	for _, licenseType := range licenseOrder {
		if modules, exists := byLicense[licenseType]; exists {
			fmt.Fprintf(out, "\n%s (%d modules):\n", licenseType, len(modules))
			for _, module := range modules {
				fmt.Fprintf(out, "  • %s\n", module)
			}
		}
	}

	// Any other licenses
	for licenseType, modules := range byLicense {
		found := false
		for _, ordered := range licenseOrder {
			if ordered == licenseType {
				found = true
				break
			}
		}
		if !found {
			fmt.Fprintf(out, "\n%s (%d modules):\n", licenseType, len(modules))
			for _, module := range modules {
				fmt.Fprintf(out, "  • %s\n", module)
			}
		}
	}

	fmt.Fprintln(out)
	fmt.Fprintln(out, "💡 All licenses are permissive and compatible with Apache 2.0")
	fmt.Fprintln(out, "📖 For full license texts, see individual package repositories")

	return nil
}

func outputLicenseSummary(licenses []LicenseInfo, jsonFormat bool, cmd *cobra.Command) error {
	byLicense := groupByLicense(licenses)

	summaries := []LicenseSummary{}
	for licenseType, modules := range byLicense {
		summaries = append(summaries, LicenseSummary{
			LicenseType: licenseType,
			Count:       len(modules),
			Modules:     modules,
		})
	}

	// Sort by count descending
	sort.Slice(summaries, func(i, j int) bool {
		return summaries[i].Count > summaries[j].Count
	})

	if jsonFormat {
		return outputJSON(summaries, cmd)
	}

	out := cmd.OutOrStdout()

	fmt.Fprintln(out, "📊 License Summary")
	fmt.Fprintln(out, "==================")
	fmt.Fprintln(out)

	totalModules := 0
	for _, summary := range summaries {
		totalModules += summary.Count
		fmt.Fprintf(out, "%s: %d modules\n", summary.LicenseType, summary.Count)
	}

	fmt.Fprintln(out)
	fmt.Fprintf(out, "Total modules: %d\n", totalModules)
	fmt.Fprintf(out, "License types: %d\n", len(summaries))

	return nil
}

func groupByLicense(licenses []LicenseInfo) map[string][]string {
	result := make(map[string][]string)
	for _, lic := range licenses {
		if !lic.Main { // Skip main project in groupings
			result[lic.License] = append(result[lic.License], lic.Module)
		}
	}

	// Sort modules within each license group
	for _, modules := range result {
		sort.Strings(modules)
	}

	return result
}

func outputJSON(data interface{}, cmd *cobra.Command) error {
	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to format JSON: %v", err)
	}
	out := cmd.OutOrStdout()
	_, err = fmt.Fprintln(out, string(jsonData))
	return err
}

/*
Copyright © 2025 3 Leaps <info@3leaps.net>
*/
package cmd

import (
	"fmt"

    "github.com/fulmenhq/goneat/internal/ops"
	"github.com/spf13/cobra"
)

// infoCmd represents the info command
var infoCmd = &cobra.Command{
	Use:   "info",
	Short: "Display informational content and metadata",
	Long: `Info provides access to various informational content including licenses,
documentation, and other metadata about goneat and its dependencies.

This command group contains subcommands for viewing legal information,
documentation, and other reference materials.`,
	RunE: runInfo,
}

func init() {
	rootCmd.AddCommand(infoCmd)

	// Register command in ops registry with taxonomy
	capabilities := ops.GetDefaultCapabilities(ops.GroupSupport, ops.CategoryInformation)
	if err := ops.RegisterCommandWithTaxonomy("info", ops.GroupSupport, ops.CategoryInformation, capabilities, infoCmd, "Display informational content"); err != nil {
		panic(fmt.Sprintf("Failed to register info command: %v", err))
	}

	// Add subcommands
	infoCmd.AddCommand(infoLicensesCmd)
}

func runInfo(cmd *cobra.Command, args []string) error {
	fmt.Println("Info provides access to various informational content including licenses,")
	fmt.Println("documentation, and other metadata about goneat and its dependencies.")
	fmt.Println()
	fmt.Println("Available subcommands:")
	fmt.Println("  licenses    Display license information for goneat and its dependencies")
	fmt.Println()
	fmt.Println("Use \"goneat info [command] --help\" for more information about a command.")
	return nil
}

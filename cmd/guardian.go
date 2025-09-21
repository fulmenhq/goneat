package cmd

import (
	"errors"
	"fmt"
	"strings"

	"github.com/fulmenhq/goneat/internal/guardian"
	"github.com/fulmenhq/goneat/internal/ops"
	"github.com/fulmenhq/goneat/pkg/logger"
	"github.com/spf13/cobra"
)

var (
	guardianBranch string
	guardianRemote string
	guardianUser   string
)

var guardianCmd = &cobra.Command{
	Use:   "guardian",
	Short: "Manage goneat guardian security approvals",
	Long: `Guardian provides security enforcement for high-risk operations such as
commits and pushes. It evaluates repository policies and orchestrates
out-of-band approval flows when required.`,
}

var guardianCheckCmd = &cobra.Command{
	Use:   "check <scope> <operation>",
	Short: "Evaluate guardian policy for an operation",
	Args:  cobra.ExactArgs(2),
	RunE:  runGuardianCheck,
}

var guardianApproveCmd = &cobra.Command{
	Use:   "approve <scope> <operation>",
	Short: "Initiate interactive approval flow (browser)",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, _ []string) error {
		cmd.SilenceUsage = true
		return errors.New("guardian approve not yet implemented - pending interactive flow")
	},
}

var guardianGrantCmd = &cobra.Command{
	Use:   "grant <scope> <operation>",
	Short: "Pre-authorize an operation via grant token",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, _ []string) error {
		cmd.SilenceUsage = true
		return errors.New("guardian grant not yet implemented - grant store work pending")
	},
}

var guardianStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show guardian status, policies, and active grants",
	RunE: func(cmd *cobra.Command, _ []string) error {
		cmd.SilenceUsage = true
		return errors.New("guardian status not yet implemented")
	},
}

var guardianSetupCmd = &cobra.Command{
	Use:   "setup",
	Short: "Bootstrap guardian configuration",
	RunE:  runGuardianSetup,
}

func init() {
	rootCmd.AddCommand(guardianCmd)

	capabilities := ops.GetDefaultCapabilities(ops.GroupWorkflow, ops.CategoryManagement)
	if err := ops.RegisterCommandWithTaxonomy("guardian", ops.GroupWorkflow, ops.CategoryManagement, capabilities, guardianCmd, "Guardian approval workflows"); err != nil {
		panic(fmt.Sprintf("Failed to register guardian command: %v", err))
	}

	guardianCmd.AddCommand(guardianCheckCmd)
	guardianCmd.AddCommand(guardianApproveCmd)
	guardianCmd.AddCommand(guardianGrantCmd)
	guardianCmd.AddCommand(guardianStatusCmd)
	guardianCmd.AddCommand(guardianSetupCmd)

	guardianCheckCmd.Flags().StringVar(&guardianBranch, "branch", "", "Active branch (used for branch-based policies)")
	guardianCheckCmd.Flags().StringVar(&guardianRemote, "remote", "", "Git remote name or URL (used for remote-based policies)")
	guardianCheckCmd.Flags().StringVar(&guardianUser, "user", "", "User performing the operation (optional)")
}

func runGuardianCheck(cmd *cobra.Command, args []string) error {
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true

	scope := strings.TrimSpace(args[0])
	operation := strings.TrimSpace(args[1])

	ctx := guardian.OperationContext{
		Branch: guardianBranch,
		Remote: guardianRemote,
		User:   guardianUser,
	}

	policy, err := guardian.CheckAndExplain(scope, operation, ctx)
	if err == nil {
		return nil
	}

	if guardian.IsApprovalRequired(err) {
		var approvalMsg string
		if ar, ok := err.(*guardian.ApprovalRequiredError); ok && ar.Policy != nil {
			approvalMsg = fmt.Sprintf("guardian approval required for %s.%s (method=%s, expires=%s)", ar.Scope, ar.Operation, ar.Policy.Method, ar.Policy.Expires)
		} else if policy != nil {
			approvalMsg = fmt.Sprintf("guardian approval required for %s.%s (method=%s, expires=%s)", scope, operation, policy.Method, policy.Expires)
		} else {
			approvalMsg = fmt.Sprintf("guardian approval required for %s.%s", scope, operation)
		}
		fmt.Fprintln(cmd.ErrOrStderr(), approvalMsg)
		return err
	}

	fmt.Fprintf(cmd.ErrOrStderr(), "guardian check failed: %v\n", err)
	return err
}

func runGuardianSetup(cmd *cobra.Command, _ []string) error {
	cmd.SilenceUsage = true

	path, err := guardian.EnsureConfig()
	if err != nil {
		return err
	}

	logger.Info("Guardian config ensured", logger.String("path", path))
	fmt.Fprintf(cmd.OutOrStdout(), "Guardian configuration available at %s\n", path)
	return nil
}

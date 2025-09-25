package cmd

import (
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/fulmenhq/goneat/internal/guardian"
	"github.com/fulmenhq/goneat/internal/ops"
	"github.com/fulmenhq/goneat/pkg/logger"
	"github.com/spf13/cobra"
)

var (
	guardianBranch string
	guardianRemote string
	guardianUser   string
	guardianReason string
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
	Use:   "approve <scope> <operation> <command...>",
	Short: "Initiate interactive approval flow and execute command if approved",
	Args:  cobra.MinimumNArgs(3),
	RunE:  runGuardianApprove,
}

var guardianGrantCmd = &cobra.Command{
	Use:   "grant <scope> <operation>",
	Short: "Pre-authorize an operation via grant token",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, _ []string) error {
		cmd.SilenceUsage = true
		return fmt.Errorf("guardian grant not yet implemented - grant store work pending")
	},
}

var guardianStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show guardian status, policies, and active grants",
	RunE: func(cmd *cobra.Command, _ []string) error {
		cmd.SilenceUsage = true
		return fmt.Errorf("guardian status not yet implemented")
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

	guardianApproveCmd.Flags().StringVar(&guardianBranch, "branch", "", "Active branch (used for branch-based policies)")
	guardianApproveCmd.Flags().StringVar(&guardianRemote, "remote", "", "Git remote name or URL (used for remote-based policies)")
	guardianApproveCmd.Flags().StringVar(&guardianUser, "user", "", "User requesting approval (optional)")
	guardianApproveCmd.Flags().StringVar(&guardianReason, "reason", "", "Reason for requesting approval (displayed to reviewers)")
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

func runGuardianApprove(cmd *cobra.Command, args []string) error {
	cmd.SilenceUsage = true

	if len(args) < 3 {
		return fmt.Errorf("usage: goneat guardian approve <scope> <operation> <command...>")
	}

	scope := strings.TrimSpace(args[0])
	operation := strings.TrimSpace(args[1])
	cmdArgs := args[2:]

	if len(cmdArgs) == 0 {
		return fmt.Errorf("command to execute is required. Wrap your operation, for example: goneat guardian approve %s %s -- git push origin main", scope, operation)
	}

	opCtx := guardian.OperationContext{
		Branch: guardianBranch,
		Remote: guardianRemote,
		User:   guardianUser,
	}

	engine, err := guardian.NewEngine()
	if err != nil {
		return err
	}

	policy, err := engine.Check(scope, operation, opCtx)
	if err == nil && policy == nil {
		fmt.Fprintf(cmd.OutOrStdout(), "No guardian policy requires approval for %s.%s\n", scope, operation)
		return nil
	}
	if err != nil && !guardian.IsApprovalRequired(err) {
		return err
	}

	if policy == nil {
		if approvalErr, ok := err.(*guardian.ApprovalRequiredError); ok && approvalErr.Policy != nil {
			policy = approvalErr.Policy
		}
	}
	if policy == nil {
		return fmt.Errorf("guardian policy not found for %s.%s", scope, operation)
	}
	if policy.Method != guardian.MethodBrowser {
		return fmt.Errorf("guardian method %s not yet supported", policy.Method)
	}

	session := guardian.ApprovalSession{
		Scope:       scope,
		Operation:   operation,
		Policy:      policy,
		Reason:      guardianReason,
		RequestedAt: time.Now().UTC(),
	}

	server, err := guardian.StartBrowserApproval(cmd.Context(), session)
	if err != nil {
		return err
	}

	expiresAt := server.ExpiresAt()
	remaining := time.Until(expiresAt).Round(time.Second)
	if remaining < 0 {
		remaining = 0
	}
	fmt.Fprintf(cmd.OutOrStdout(), "Guardian approval server listening on %s\n", server.URL())
	fmt.Fprintf(cmd.OutOrStdout(), "Approval URL: %s\n", server.ApprovalURL())
	fmt.Fprintf(cmd.OutOrStdout(), "Approval expires at %s (%s remaining)\n", expiresAt.Format(time.RFC3339), remaining)
	fmt.Fprintln(cmd.OutOrStdout(), "Press Ctrl+C to cancel the approval session.")

	if err := server.Wait(); err != nil {
		if errors.Is(err, guardian.ErrApprovalExpired) {
			return fmt.Errorf("guardian approval expired before the command could be executed")
		}
		return err
	}

	// Approval granted, execute the command
	logger.Info("Guardian approval granted, executing command", logger.String("command", strings.Join(cmdArgs, " ")))
	ecmd := exec.Command(cmdArgs[0], cmdArgs[1:]...)
	ecmd.Stdout = cmd.OutOrStdout()
	ecmd.Stderr = cmd.ErrOrStderr()
	ecmd.Stdin = cmd.InOrStdin()

	if err := ecmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			// Return the command's exit code
			return exitErr
		}
		return fmt.Errorf("failed to execute command: %w", err)
	}

	return nil
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

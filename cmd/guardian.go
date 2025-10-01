package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/fulmenhq/goneat/internal/guardian"
	"github.com/fulmenhq/goneat/internal/ops"
	"github.com/fulmenhq/goneat/pkg/logger"
	"github.com/spf13/cobra"
)

// Helper function to handle fmt print errors without cluttering code
func printErr(w interface{ Write([]byte) (int, error) }, message string) {
	if _, err := fmt.Fprintln(w, message); err != nil {
		logger.Error("Failed to write message", logger.Err(err))
	}
}

// Helper function for formatted messages
func printErrf(w interface{ Write([]byte) (int, error) }, format string, args ...interface{}) {
	if _, err := fmt.Fprintf(w, format, args...); err != nil {
		logger.Error("Failed to write message", logger.Err(err))
	}
}

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
	Use:   "check <scope> <operation> [command...]",
	Short: "Evaluate guardian policy for an operation",
	Args:  cobra.MinimumNArgs(2),
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
	cmdArgs := args[2:]

	ctx := guardian.OperationContext{
		Branch: guardianBranch,
		Remote: guardianRemote,
		User:   guardianUser,
	}

	policy, err := guardian.CheckAndExplain(scope, operation, ctx)
	if err == nil {
		printErrf(cmd.ErrOrStderr(), "No guardian policy requires approval for %s.%s\n", scope, operation)
		return nil
	}

	if !guardian.IsApprovalRequired(err) {
		printErrf(cmd.ErrOrStderr(), "guardian check failed: %v\n", err)
		return err
	}

	// Approval is required - start browser approval flow
	approvalMsg := fmt.Sprintf("Guardian approval required for %s.%s", scope, operation)
	printErr(cmd.ErrOrStderr(), approvalMsg)

	if policy == nil {
		printErrf(cmd.ErrOrStderr(), "guardian check failed: policy not found\n")
		return err
	}

	// Construct full command - use provided args if available, otherwise generic
	var fullCommand string
	if len(cmdArgs) > 0 {
		// Include the actual command (scope + operation) with the arguments
		switch scope {
		case "git":
			fullCommand = fmt.Sprintf("git %s %s", operation, strings.Join(cmdArgs, " "))
		case "system":
			fullCommand = fmt.Sprintf("%s %s", operation, strings.Join(cmdArgs, " "))
		default:
			fullCommand = fmt.Sprintf("%s.%s %s", scope, operation, strings.Join(cmdArgs, " "))
		}
	} else {
		switch scope {
		case "git":
			fullCommand = fmt.Sprintf("git %s", operation)
		case "system":
			fullCommand = fmt.Sprintf("%s command", operation)
		default:
			fullCommand = fmt.Sprintf("%s.%s operation", scope, operation)
		}
	}

	session := guardian.ApprovalSession{
		Scope:     scope,
		Operation: operation,
		Policy:    policy,
		Reason: func() string {
			if os.Getenv("GONEAT_GUARDIAN_TEST_MODE") != "" {
				return fmt.Sprintf("Guardian policy check for %s.%s - testing only", scope, operation)
			}
			return fmt.Sprintf("Guardian policy check for %s.%s", scope, operation)
		}(),
		RequestedAt: time.Now().UTC(),
		ProjectName: "", // Will be populated by StartBrowserApproval from git repo or config
		FullCommand: fullCommand,
	}

	// In test mode, auto-deny for testing
	if os.Getenv("GONEAT_GUARDIAN_AUTO_DENY") != "" {
		printErr(cmd.ErrOrStderr(), approvalMsg)
		printErr(cmd.ErrOrStderr(), "❌ Approval failed: operation denied by user")
		return err
	}

	server, serverErr := guardian.StartBrowserApproval(context.Background(), session)
	if serverErr != nil {
		printErrf(cmd.ErrOrStderr(), "Failed to start approval server: %v\n", serverErr)
		printErr(cmd.ErrOrStderr(), approvalMsg)
		return err
	}

	printErrf(cmd.ErrOrStderr(), "Approval URL: %s\n", server.ApprovalURL())
	printErr(cmd.ErrOrStderr(), "Open this URL in your browser to approve/deny the operation.")
	printErr(cmd.ErrOrStderr(), "The approval server will run for the duration of this check.")

	// Wait for approval or timeout
	waitErr := server.Wait()
	if waitErr == nil {
		printErr(cmd.ErrOrStderr(), "✅ Approval granted!")
		return nil
	} else if errors.Is(waitErr, guardian.ErrApprovalExpired) {
		printErr(cmd.ErrOrStderr(), "❌ Approval expired")
		return waitErr
	} else {
		printErrf(cmd.ErrOrStderr(), "❌ Approval failed: %v\n", waitErr)
		return waitErr
	}
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
	requiresApproval := false
	var grant *guardian.Grant
	var revokeGrant bool
	if err == nil && policy == nil {
		printErrf(cmd.OutOrStdout(), "No guardian policy requires approval for %s.%s\n", scope, operation)
	} else if err != nil && !guardian.IsApprovalRequired(err) {
		return err
	} else {
		requiresApproval = true
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

		// Construct full command including the actual command itself
		var fullCommand string
		switch scope {
		case "git":
			fullCommand = fmt.Sprintf("git %s %s", operation, strings.Join(cmdArgs, " "))
		case "system":
			fullCommand = fmt.Sprintf("%s %s", operation, strings.Join(cmdArgs, " "))
		default:
			fullCommand = fmt.Sprintf("%s.%s %s", scope, operation, strings.Join(cmdArgs, " "))
		}

		session := guardian.ApprovalSession{
			Scope:       scope,
			Operation:   operation,
			Policy:      policy,
			Reason:      guardianReason,
			RequestedAt: time.Now().UTC(),
			ProjectName: "", // Will be populated by StartBrowserApproval from git repo or config
			FullCommand: fullCommand,
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
		printErrf(cmd.OutOrStdout(), "Guardian approval server listening on %s\n", server.URL())
		printErrf(cmd.OutOrStdout(), "Approval URL: %s\n", server.ApprovalURL())
		printErrf(cmd.OutOrStdout(), "Approval expires at %s (%s remaining)\n", expiresAt.Format(time.RFC3339), remaining)
		printErr(cmd.OutOrStdout(), "Press Ctrl+C to cancel the approval session.")

		if err := server.Wait(); err != nil {
			if errors.Is(err, guardian.ErrApprovalExpired) {
				return fmt.Errorf("guardian approval expired before the command could be executed")
			}
			// Check for denial error and provide clear feedback
			if strings.Contains(err.Error(), "denied") {
				printErrf(cmd.ErrOrStderr(), "❌ Guardian approval denied by user - operation cancelled\n")
			}
			return err
		}

		grant, err = guardian.IssueGrant(scope, operation, policy, opCtx)
		if err != nil {
			return fmt.Errorf("failed to issue guardian grant: %w", err)
		}
		revokeGrant = true
		defer func() {
			if revokeGrant && grant != nil {
				guardian.RevokeGrant(grant.ID)
			}
		}()
	}

	// Execute the command (whether approval was required or not)
	logger.Info("Executing command", logger.String("command", strings.Join(cmdArgs, " ")))
	ecmd := exec.Command(cmdArgs[0], cmdArgs[1:]...) // #nosec G204 -- intentional execution of user command after guardian check
	ecmd.Stdout = cmd.OutOrStdout()
	ecmd.Stderr = cmd.ErrOrStderr()
	ecmd.Stdin = cmd.InOrStdin()

	if err := ecmd.Run(); err != nil {
		if requiresApproval && grant != nil {
			guardian.RevokeGrant(grant.ID)
		}
		if exitErr, ok := err.(*exec.ExitError); ok {
			// Return the command's exit code
			return exitErr
		}
		return fmt.Errorf("failed to execute command: %w", err)
	}

	if requiresApproval {
		revokeGrant = false
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
	printErrf(cmd.OutOrStdout(), "Guardian configuration available at %s\n", path)
	return nil
}

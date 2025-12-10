// Package assess provides codebase assessment functionality.
package assess

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"
	"time"

	"github.com/fulmenhq/goneat/pkg/logger"
)

// HookCommand represents a single command from the hooks manifest.
// This struct aligns with the hooks-manifest schema (schemas/work/v1.0.0/hooks-manifest.yaml).
type HookCommand struct {
	Command  string   `yaml:"command"`
	Args     []string `yaml:"args"`
	Priority int      `yaml:"priority"`
	Timeout  string   `yaml:"timeout"`
	Fallback string   `yaml:"fallback"` // Phase 2: not implemented yet
}

// InternalCommandHandler is a function that handles internal goneat commands.
// It receives the command name, args, and context, and returns an error if execution fails.
type InternalCommandHandler func(ctx context.Context, command string, args []string) error

// HookExecutor executes commands defined in a hooks manifest.
type HookExecutor struct {
	// WorkDir is the working directory for command execution (typically repo root)
	WorkDir string
	// Verbose enables detailed command output (progress logs)
	Verbose bool
	// InternalHandler handles internal goneat commands (assess, format, etc.)
	// If nil, internal commands will be executed as external processes.
	InternalHandler InternalCommandHandler
}

// NewHookExecutor creates a new HookExecutor with the given working directory.
func NewHookExecutor(workDir string) *HookExecutor {
	return &HookExecutor{
		WorkDir: workDir,
		Verbose: false,
	}
}

// ExecuteHookCommands executes all commands for a given hook type in priority order.
// Commands are sorted by priority (lower numbers run first). For equal priorities,
// the original manifest order is preserved (stable sort).
// Execution stops on first error (fail-fast behavior).
//
// Internal goneat commands (assess, format, dependencies, etc.) are routed to
// InternalHandler if set; otherwise they execute as external processes.
//
// Security note: External commands are executed with the same privileges as the
// calling process. The hooks.yaml file is part of the repository and has the same
// trust level as Makefile or any other checked-in configuration.
// See feature brief for security analysis.
func (e *HookExecutor) ExecuteHookCommands(ctx context.Context, commands []HookCommand) error {
	if len(commands) == 0 {
		logger.Debug("hook-executor: no commands to execute")
		return nil
	}

	// Sort by priority (lower numbers first), stable to preserve manifest order for ties
	sorted := make([]HookCommand, len(commands))
	copy(sorted, commands)
	sort.SliceStable(sorted, func(i, j int) bool {
		return sorted[i].Priority < sorted[j].Priority
	})

	e.logInfo(fmt.Sprintf("hook-executor: executing %d command(s)", len(sorted)))

	for i, cmd := range sorted {
		cmdStr := formatCommandString(cmd)
		e.logInfo(fmt.Sprintf("hook-executor: [%d/%d] running: %s", i+1, len(sorted), cmdStr))

		if err := e.executeCommand(ctx, cmd); err != nil {
			logger.Error(fmt.Sprintf("hook-executor: command failed: %s: %v", cmdStr, err))
			return fmt.Errorf("hook command failed: %s: %w", cmd.Command, err)
		}

		e.logInfo(fmt.Sprintf("hook-executor: [%d/%d] completed: %s", i+1, len(sorted), cmd.Command))
	}

	e.logInfo("hook-executor: all commands completed successfully")
	return nil
}

// logInfo logs at info level only if Verbose is enabled, otherwise logs at debug.
func (e *HookExecutor) logInfo(msg string) {
	if e.Verbose {
		logger.Info(msg)
	} else {
		logger.Debug(msg)
	}
}

// executeCommand runs a single hook command with timeout enforcement.
// Routes internal goneat commands to InternalHandler if set.
func (e *HookExecutor) executeCommand(ctx context.Context, hookCmd HookCommand) error {
	// Parse timeout (default 2m if not specified or invalid)
	timeout := 2 * time.Minute
	if hookCmd.Timeout != "" {
		if parsed, err := time.ParseDuration(hookCmd.Timeout); err == nil {
			timeout = parsed
		} else {
			logger.Warn(fmt.Sprintf("hook-executor: invalid timeout %q, using default 2m", hookCmd.Timeout))
		}
	}

	// Create context with timeout
	cmdCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	var err error

	// Route internal commands to handler if available
	if IsInternalCommand(hookCmd.Command) {
		if e.InternalHandler != nil {
			logger.Debug(fmt.Sprintf("hook-executor: routing internal command %q to handler", hookCmd.Command))
			err = e.InternalHandler(cmdCtx, hookCmd.Command, hookCmd.Args)
		} else {
			// No internal handler - warn and execute as external (goneat binary)
			logger.Warn(fmt.Sprintf("hook-executor: no internal handler for %q, executing as external command", hookCmd.Command))
			err = e.executeExternalCommand(cmdCtx, hookCmd)
		}
	} else {
		// External command (make, npm, etc.)
		err = e.executeExternalCommand(cmdCtx, hookCmd)
	}

	// Check for timeout
	if cmdCtx.Err() == context.DeadlineExceeded {
		return fmt.Errorf("command timed out after %s", timeout)
	}

	return err
}

// executeExternalCommand runs an external command (non-goneat).
func (e *HookExecutor) executeExternalCommand(ctx context.Context, hookCmd HookCommand) error {
	// Build the command
	// #nosec G204 -- Commands come from hooks.yaml which is a checked-in config file
	// with the same trust level as Makefile. See security analysis in feature brief.
	cmd := exec.CommandContext(ctx, hookCmd.Command, hookCmd.Args...)
	cmd.Dir = e.WorkDir
	cmd.Env = os.Environ()

	// Connect stdout/stderr for visibility
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

// IsInternalCommand returns true if the command is a goneat internal command
// that should be routed to InternalHandler rather than executed as an external process.
func IsInternalCommand(command string) bool {
	switch command {
	case "assess", "format", "lint", "security", "dependencies", "validate":
		return true
	default:
		return false
	}
}

// formatCommandString returns a human-readable representation of a hook command.
func formatCommandString(cmd HookCommand) string {
	if len(cmd.Args) == 0 {
		return cmd.Command
	}
	return fmt.Sprintf("%s %s", cmd.Command, strings.Join(cmd.Args, " "))
}

/*
Copyright © 2025 3 Leaps <info@3leaps.com>
*/
package tools

import (
	"context"
	"io"
	"os"
	"strings"
)

// ExecutionMode determines how tools are executed
type ExecutionMode string

const (
	// ModeAuto automatically selects the best executor
	ModeAuto ExecutionMode = "auto"
	// ModeLocal forces local execution via PATH/shim directories
	ModeLocal ExecutionMode = "local"
	// ModeDocker forces execution via goneat-tools Docker image
	ModeDocker ExecutionMode = "docker"
)

// ExecuteOptions configures tool execution
type ExecuteOptions struct {
	// Tool name (e.g., "prettier", "yamlfmt")
	Tool string

	// Args to pass to the tool
	Args []string

	// WorkDir is the working directory (defaults to current directory)
	WorkDir string

	// Stdin to pipe to the tool (optional)
	Stdin io.Reader

	// Env contains additional environment variables
	Env map[string]string
}

// ExecuteResult contains the output of tool execution
type ExecuteResult struct {
	// ExitCode from the tool
	ExitCode int

	// Stdout contains standard output
	Stdout []byte

	// Stderr contains standard error
	Stderr []byte

	// Executor indicates which executor was used ("local" or "docker")
	Executor string
}

// ToolExecutor executes external tools
type ToolExecutor interface {
	// Execute runs a tool with the given options
	Execute(ctx context.Context, opts ExecuteOptions) (*ExecuteResult, error)

	// IsAvailable checks if this executor can run the specified tool
	IsAvailable(tool string) bool

	// Name returns the executor name for logging
	Name() string
}

// NewExecutor creates a ToolExecutor based on the specified mode
// If mode is empty, it reads from GONEAT_TOOL_MODE environment variable
func NewExecutor(mode ExecutionMode) ToolExecutor {
	if mode == "" {
		mode = getModeFromEnv()
	}

	switch mode {
	case ModeLocal:
		return NewLocalExecutor()
	case ModeDocker:
		return NewDockerExecutor()
	case ModeAuto:
		fallthrough
	default:
		return NewAutoExecutor()
	}
}

// getModeFromEnv reads execution mode from environment
func getModeFromEnv() ExecutionMode {
	mode := os.Getenv("GONEAT_TOOL_MODE")
	switch strings.ToLower(mode) {
	case "local":
		return ModeLocal
	case "docker":
		return ModeDocker
	default:
		return ModeAuto
	}
}

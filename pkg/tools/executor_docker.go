/*
Copyright © 2025 3 Leaps <info@3leaps.com>
*/
package tools

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"

	"github.com/fulmenhq/goneat/pkg/logger"
)

const (
	// DefaultToolsImage is the default goneat-tools Docker image
	DefaultToolsImage = "ghcr.io/fulmenhq/goneat-tools:latest"

	// ToolsImageEnvVar allows overriding the goneat-tools image
	ToolsImageEnvVar = "GONEAT_TOOLS_IMAGE"
)

// DockerExecutor runs tools via the goneat-tools Docker image
type DockerExecutor struct {
	image      string
	dockerPath string
}

// NewDockerExecutor creates a new DockerExecutor
func NewDockerExecutor() *DockerExecutor {
	image := os.Getenv(ToolsImageEnvVar)
	if image == "" {
		image = DefaultToolsImage
	}

	dockerPath, _ := exec.LookPath("docker")

	return &DockerExecutor{
		image:      image,
		dockerPath: dockerPath,
	}
}

// Name returns the executor name
func (e *DockerExecutor) Name() string {
	return "docker"
}

// IsAvailable checks if docker is available and the tool is in the image
func (e *DockerExecutor) IsAvailable(tool string) bool {
	if e.dockerPath == "" {
		return false
	}
	return e.isToolInImage(tool)
}

// isToolInImage checks if a tool is available in the goneat-tools image
func (e *DockerExecutor) isToolInImage(tool string) bool {
	// Tools available in ghcr.io/fulmenhq/goneat-tools:latest
	// Keep in sync with fulmen-toolbox/images/goneat-tools/Dockerfile
	supportedTools := map[string]bool{
		"prettier": true,
		"yamlfmt":  true,
		"jq":       true,
		"yq":       true,
		"rg":       true,
		"ripgrep":  true,
		"git":      true,
		"bash":     true,
	}
	return supportedTools[tool]
}

// DockerAvailable returns true if docker is installed and accessible
func (e *DockerExecutor) DockerAvailable() bool {
	return e.dockerPath != ""
}

// Execute runs the tool via docker
func (e *DockerExecutor) Execute(ctx context.Context, opts ExecuteOptions) (*ExecuteResult, error) {
	if e.dockerPath == "" {
		return nil, fmt.Errorf("docker not found in PATH")
	}

	workDir := opts.WorkDir
	if workDir == "" {
		var err error
		workDir, err = os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("failed to get working directory: %w", err)
		}
	}

	// Build docker run command
	// docker run --rm -v "$PWD:/work" -w /work ghcr.io/fulmenhq/goneat-tools:latest <tool> <args>
	dockerArgs := []string{
		"run", "--rm",
		"-v", fmt.Sprintf("%s:/work", workDir),
		"-w", "/work",
	}

	// Add environment variables
	for k, v := range opts.Env {
		dockerArgs = append(dockerArgs, "-e", fmt.Sprintf("%s=%s", k, v))
	}

	// Handle stdin
	if opts.Stdin != nil {
		dockerArgs = append(dockerArgs, "-i")
	}

	// Add image and tool command
	dockerArgs = append(dockerArgs, e.image, opts.Tool)
	dockerArgs = append(dockerArgs, opts.Args...)

	logger.Debug(fmt.Sprintf("docker executor: docker %v", dockerArgs))

	// #nosec G204 - dockerPath comes from exec.LookPath, args are controlled
	cmd := exec.CommandContext(ctx, e.dockerPath, dockerArgs...)

	if opts.Stdin != nil {
		cmd.Stdin = opts.Stdin
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	result := &ExecuteResult{
		Stdout:   stdout.Bytes(),
		Stderr:   stderr.Bytes(),
		Executor: "docker",
	}

	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			result.ExitCode = exitErr.ExitCode()
			// Return result with exit code, not an error
			return result, nil
		}
		return nil, fmt.Errorf("docker execution failed: %w", err)
	}

	return result, nil
}

// GetImage returns the configured image
func (e *DockerExecutor) GetImage() string {
	return e.image
}

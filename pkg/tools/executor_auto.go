/*
Copyright © 2025 3 Leaps <info@3leaps.com>
*/
package tools

import (
	"context"
	"fmt"
	"os"

	"github.com/fulmenhq/goneat/pkg/logger"
)

// AutoExecutor automatically selects the best executor based on environment
type AutoExecutor struct {
	local  *LocalExecutor
	docker *DockerExecutor
}

// NewAutoExecutor creates a new AutoExecutor
func NewAutoExecutor() *AutoExecutor {
	return &AutoExecutor{
		local:  NewLocalExecutor(),
		docker: NewDockerExecutor(),
	}
}

// Name returns the executor name
func (e *AutoExecutor) Name() string {
	return "auto"
}

// IsAvailable checks if either executor can run the tool
func (e *AutoExecutor) IsAvailable(tool string) bool {
	return e.local.IsAvailable(tool) || e.docker.IsAvailable(tool)
}

// Execute runs the tool using the best available executor
//
// Selection logic:
// 1. If in CI environment and docker available, prefer docker for consistency
// 2. If tool is locally available, use local (faster)
// 3. If tool is in docker image, use docker as fallback
// 4. Error with helpful message
func (e *AutoExecutor) Execute(ctx context.Context, opts ExecuteOptions) (*ExecuteResult, error) {
	inCI := isInCI()
	dockerAvailable := e.docker.DockerAvailable() && e.docker.isToolInImage(opts.Tool)
	localAvailable := e.local.IsAvailable(opts.Tool)

	// CI environment: prefer docker for consistency
	if inCI && dockerAvailable {
		logger.Debug(fmt.Sprintf("auto executor: CI detected, using docker for %s", opts.Tool))
		return e.docker.Execute(ctx, opts)
	}

	// Local tool available: use it (faster)
	if localAvailable {
		logger.Debug(fmt.Sprintf("auto executor: using local for %s", opts.Tool))
		return e.local.Execute(ctx, opts)
	}

	// Docker available: use it as fallback
	if dockerAvailable {
		logger.Debug(fmt.Sprintf("auto executor: tool not local, using docker for %s", opts.Tool))
		return e.docker.Execute(ctx, opts)
	}

	// Neither available - provide helpful error
	return nil, fmt.Errorf(`tool %s not found

Try one of:
  - Install locally:  goneat doctor tools --install %s
  - Use docker mode:  export GONEAT_TOOL_MODE=docker
  - Pull image:       docker pull %s

In CI workflows, consider using:
  container: %s`,
		opts.Tool, opts.Tool, e.docker.GetImage(), e.docker.GetImage())
}

// isInCI detects if we're running in a CI environment
func isInCI() bool {
	// Common CI environment variables
	ciEnvVars := []string{
		"CI",
		"GITHUB_ACTIONS",
		"GITLAB_CI",
		"CIRCLECI",
		"TRAVIS",
		"JENKINS_URL",
		"BUILDKITE",
		"DRONE",
		"TEAMCITY_VERSION",
	}

	for _, envVar := range ciEnvVars {
		if os.Getenv(envVar) != "" {
			return true
		}
	}

	return false
}

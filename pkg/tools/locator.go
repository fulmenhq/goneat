package tools

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/fulmenhq/goneat/pkg/logger"
)

// ResolveOptions configures how binary resolution works
type ResolveOptions struct {
	// EnvOverride specifies an environment variable name to check for explicit override
	// e.g., "GONEAT_TOOL_SYFT" would check the GONEAT_TOOL_SYFT environment variable
	EnvOverride string
	// AllowPath determines if PATH fallback is allowed when managed binary not found
	AllowPath bool
}

// ResolveBinary finds the path to a tool binary following the resolution order:
// 1. Environment variable override (if specified)
// 2. Managed binary directory (~/.goneat/tools/bin/<tool>@<version>/)
// 3. PATH fallback (if AllowPath is true)
//
// Returns the full path to the binary and any error encountered.
func ResolveBinary(toolName string, opts ResolveOptions) (string, error) {
	logger.Debug("starting binary resolution", logger.String("tool", toolName), logger.String("env_override", opts.EnvOverride), logger.Bool("allow_path", opts.AllowPath))

	// 1. Check environment variable override
	if opts.EnvOverride != "" {
		if overridePath := os.Getenv(opts.EnvOverride); overridePath != "" {
			logger.Debug("checking env override", logger.String("env_var", opts.EnvOverride), logger.String("path", overridePath))
			if _, err := os.Stat(overridePath); err == nil {
				logger.Debug("resolution successful: env override", logger.String("path", overridePath))
				return overridePath, nil
			}
			logger.Debug("env override path invalid", logger.String("path", overridePath))
		} else {
			logger.Debug("no env override set", logger.String("env_var", opts.EnvOverride))
		}
	} else {
		logger.Debug("no env override configured for tool", logger.String("tool", toolName))
	}

	// 2. Check managed binary directory
	binDir, err := GetBinDir()
	if err != nil {
		logger.Debug("failed to get bin directory, skipping managed check", logger.String("error", err.Error()))
	} else {
		entries, err := os.ReadDir(binDir)
		if err == nil {
			binaryName := toolName
			if runtime.GOOS == "windows" {
				binaryName += ".exe"
			}

			logger.Debug("scanning managed bin directory", logger.String("bin_dir", binDir), logger.Int("entries", len(entries)))
			for _, entry := range entries {
				if strings.HasPrefix(entry.Name(), toolName+"@") {
					binaryPath := filepath.Join(binDir, entry.Name(), binaryName)
					logger.Debug("checking managed binary candidate", logger.String("candidate", binaryPath))
					if _, err := os.Stat(binaryPath); err == nil {
						logger.Debug("resolution successful: managed binary", logger.String("path", binaryPath))
						return binaryPath, nil
					}
					logger.Debug("managed binary candidate not accessible", logger.String("path", binaryPath))
				}
			}
			logger.Debug("no valid managed binaries found", logger.String("tool", toolName))
		} else {
			logger.Debug("failed to read bin directory", logger.String("bin_dir", binDir), logger.String("error", err.Error()))
		}
	}

	// 3. PATH fallback (if allowed)
	if opts.AllowPath {
		logger.Debug("checking PATH fallback", logger.String("tool", toolName))
		pathBinary, err := exec.LookPath(toolName)
		if err == nil {
			logger.Debug("resolution successful: PATH fallback", logger.String("path", pathBinary))
			return pathBinary, nil
		}
		logger.Debug("PATH fallback failed", logger.String("tool", toolName))
	} else {
		logger.Debug("PATH fallback disabled", logger.String("tool", toolName))
	}

	// No binary found - construct helpful error message
	logger.Debug("resolution failed: no binary found", logger.String("tool", toolName))
	var suggestions []string
	if opts.EnvOverride != "" {
		suggestions = append(suggestions, fmt.Sprintf("set %s=/path/to/%s", opts.EnvOverride, toolName))
	}
	suggestions = append(suggestions, fmt.Sprintf("run 'goneat doctor tools --scope sbom --install' to install managed %s", toolName))
	if opts.AllowPath {
		suggestions = append(suggestions, fmt.Sprintf("install %s and ensure it's in your PATH", toolName))
	}

	return "", fmt.Errorf("tool %s not found: %s", toolName, strings.Join(suggestions, " or "))
}

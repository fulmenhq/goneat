package vulnerabilities

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"time"

	"github.com/fulmenhq/goneat/pkg/tools"
)

type GrypeInvoker struct {
	grypePath string
}

func NewGrypeInvoker() (*GrypeInvoker, error) {
	grypePath, err := tools.ResolveBinary("grype", tools.ResolveOptions{
		EnvOverride: "GONEAT_TOOL_GRYPE",
		AllowPath:   true,
	})
	if err != nil {
		return nil, fmt.Errorf("grype binary not found: %w", err)
	}
	return &GrypeInvoker{grypePath: grypePath}, nil
}

func (g *GrypeInvoker) GetVersion(ctx context.Context) (string, error) {
	cmd := exec.CommandContext(ctx, g.grypePath, "version", "-o", "json") // #nosec G204
	out, err := cmd.Output()
	if err != nil {
		cmd := exec.CommandContext(ctx, g.grypePath, "version") // #nosec G204
		text, terr := cmd.Output()
		if terr != nil {
			return "", fmt.Errorf("failed to get grype version: %w", err)
		}
		return string(text), nil
	}
	var version struct {
		Version string `json:"version"`
	}
	if err := json.Unmarshal(out, &version); err != nil {
		return "", fmt.Errorf("failed to parse grype version json: %w", err)
	}
	return version.Version, nil
}

func (g *GrypeInvoker) ScanSBOM(ctx context.Context, sbomPath string, outputPath string, timeout time.Duration) ([]byte, string, error) {
	if sbomPath == "" {
		return nil, "", fmt.Errorf("sbom path is required")
	}
	if outputPath == "" {
		return nil, "", fmt.Errorf("output path is required")
	}
	if _, err := os.Stat(sbomPath); err != nil {
		return nil, "", fmt.Errorf("sbom does not exist: %w", err)
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	args := []string{"sbom:" + sbomPath, "-o", "json"}
	cmd := exec.CommandContext(ctx, g.grypePath, args...) // #nosec G204
	out, err := cmd.Output()
	if err != nil {
		return nil, "", fmt.Errorf("grype scan failed: %w", err)
	}
	if err := os.WriteFile(outputPath, out, 0o600); err != nil {
		return nil, "", fmt.Errorf("failed to write grype output: %w", err)
	}

	version, verr := g.GetVersion(context.Background())
	if verr != nil {
		version = "unknown"
	}
	return out, version, nil
}

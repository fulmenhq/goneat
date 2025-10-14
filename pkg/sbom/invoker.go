package sbom

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/fulmenhq/goneat/pkg/logger"
	"github.com/fulmenhq/goneat/pkg/tools"
)

type Config struct {
	TargetPath string
	OutputPath string
	Format     string
	Stdout     bool
	Platform   string
}

type Result struct {
	OutputPath   string
	Format       string
	GeneratedAt  time.Time
	ToolVersion  string
	Duration     time.Duration
	PackageCount int
	SBOMContent  json.RawMessage
}

type Metadata struct {
	GeneratedAt string `json:"generated_at"`
	ToolVersion string `json:"tool_version"`
	Format      string `json:"format"`
	Target      string `json:"target"`
}

type SyftInvoker struct {
	syftPath string
}

func NewSyftInvoker() (*SyftInvoker, error) {
	syftPath, err := tools.FindToolBinary("syft")
	if err != nil {
		return nil, fmt.Errorf("syft binary not found: %w (install with: goneat doctor tools --scope sbom --install)", err)
	}

	return &SyftInvoker{
		syftPath: syftPath,
	}, nil
}

func (s *SyftInvoker) GetVersion(ctx context.Context) (string, error) {
	cmd := exec.CommandContext(ctx, s.syftPath, "version", "--output", "json")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get syft version: %w", err)
	}

	var versionData struct {
		Version string `json:"version"`
	}
	if err := json.Unmarshal(output, &versionData); err != nil {
		logger.Debug("sbom: failed to parse version JSON, using text output")
		cmd := exec.CommandContext(ctx, s.syftPath, "version")
		textOutput, err := cmd.Output()
		if err != nil {
			return "", fmt.Errorf("failed to get syft version: %w", err)
		}
		return string(textOutput), nil
	}

	return versionData.Version, nil
}

func (s *SyftInvoker) Generate(ctx context.Context, config Config) (*Result, error) {
	startTime := time.Now()

	if config.TargetPath == "" {
		config.TargetPath = "."
	}

	targetPath, err := filepath.Abs(config.TargetPath)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve target path: %w", err)
	}

	if _, err := os.Stat(targetPath); err != nil {
		return nil, fmt.Errorf("target path does not exist: %w", err)
	}

	if config.Format == "" {
		config.Format = "cyclonedx-json"
	}

	toolVersion, err := s.GetVersion(ctx)
	if err != nil {
		logger.Warn("sbom: failed to get syft version", logger.String("error", err.Error()))
		toolVersion = "unknown"
	}

	args := []string{
		"packages",
		targetPath,
		"--scope", "all-layers",
		"--output", config.Format,
	}

	if config.Platform != "" {
		args = append(args, "--platform", config.Platform)
	}

	var outputPath string
	if config.Stdout {
		args = append(args, "--file", "-")
	} else {
		if config.OutputPath == "" {
			timestamp := time.Now().Format("20060102-150405")
			outputPath = filepath.Join("sbom", fmt.Sprintf("goneat-%s.cdx.json", timestamp))
		} else {
			outputPath = config.OutputPath
		}

		outputDir := filepath.Dir(outputPath)
		if err := os.MkdirAll(outputDir, 0o750); err != nil {
			return nil, fmt.Errorf("failed to create output directory: %w", err)
		}

		args = append(args, "--file", outputPath)
	}

	logger.Debug("sbom: invoking syft", logger.String("path", s.syftPath), logger.String("target", targetPath), logger.String("output", outputPath))

	cmd := exec.CommandContext(ctx, s.syftPath, args...)
	cmd.Stderr = os.Stderr

	var sbomContent json.RawMessage
	if config.Stdout {
		output, err := cmd.Output()
		if err != nil {
			return nil, fmt.Errorf("syft execution failed: %w", err)
		}
		sbomContent = output
	} else {
		if err := cmd.Run(); err != nil {
			return nil, fmt.Errorf("syft execution failed: %w", err)
		}

		content, err := os.ReadFile(outputPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read generated SBOM: %w", err)
		}
		sbomContent = content
	}

	duration := time.Since(startTime)

	packageCount, err := extractPackageCount(sbomContent, config.Format)
	if err != nil {
		logger.Warn("sbom: failed to extract package count", logger.String("error", err.Error()))
		packageCount = 0
	}

	result := &Result{
		OutputPath:   outputPath,
		Format:       config.Format,
		GeneratedAt:  startTime,
		ToolVersion:  toolVersion,
		Duration:     duration,
		PackageCount: packageCount,
		SBOMContent:  sbomContent,
	}

	logger.Info("sbom: generation complete", logger.String("output", outputPath), logger.Int("packages", packageCount), logger.String("duration", duration.String()))

	return result, nil
}

func extractPackageCount(content json.RawMessage, format string) (int, error) {
	if format != "cyclonedx-json" {
		return 0, fmt.Errorf("unsupported format for package counting: %s", format)
	}

	var cdx struct {
		Components []interface{} `json:"components"`
	}

	if err := json.Unmarshal(content, &cdx); err != nil {
		return 0, fmt.Errorf("failed to parse CycloneDX SBOM: %w", err)
	}

	return len(cdx.Components), nil
}

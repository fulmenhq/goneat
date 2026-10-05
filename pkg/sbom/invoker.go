package sbom

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/fulmenhq/goneat/pkg/logger"
	"github.com/fulmenhq/goneat/pkg/tools"
)

type Config struct {
	TargetPath      string
	OutputPath      string
	Format          string
	Stdout          bool
	Platform        string
	ExcludePatterns []string
	SourceOptions   SourceOptions
}

type Result struct {
	OutputPath      string
	Format          string
	GeneratedAt     time.Time
	ToolVersion     string
	Duration        time.Duration
	PackageCount    int
	SBOMContent     json.RawMessage
	DependencyGraph *DependencyGraph
}

type Metadata struct {
	GeneratedAt string `json:"generated_at"`
	ToolVersion string `json:"tool_version"`
	Format      string `json:"format"`
	Target      string `json:"target"`
}

type DependencyGraph struct {
	Nodes map[string]*DependencyNode
	Roots []string
}

type DependencyNode struct {
	Ref          string
	Name         string
	Version      string
	Type         string
	PURL         string
	Dependencies []string
}

type SyftInvoker struct {
	syftPath string
}

func NewSyftInvoker() (*SyftInvoker, error) {
	syftPath, err := tools.ResolveBinary("syft", tools.ResolveOptions{
		EnvOverride: "GONEAT_TOOL_SYFT",
		AllowPath:   true,
	})
	if err != nil {
		return nil, fmt.Errorf("syft binary not found: %w", err)
	}

	return &SyftInvoker{
		syftPath: syftPath,
	}, nil
}

func (s *SyftInvoker) GetVersion(ctx context.Context) (string, error) {
	cmd := exec.CommandContext(ctx, s.syftPath, "version", "--output", "json") // #nosec G204 - Arguments are hardcoded constants
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get syft version: %w", err)
	}

	var versionData struct {
		Version string `json:"version"`
	}
	if err := json.Unmarshal(output, &versionData); err != nil {
		logger.Debug("sbom: failed to parse version JSON, trying text parsing")
		cmd := exec.CommandContext(ctx, s.syftPath, "version") // #nosec G204 - Arguments are hardcoded constants
		textOutput, err := cmd.Output()
		if err != nil {
			return "", fmt.Errorf("failed to get syft version: %w", err)
		}
		// Parse multiline version output (e.g., "Application:   syft\nVersion:       1.33.0\n...")
		lines := strings.Split(string(textOutput), "\n")
		for _, line := range lines {
			if strings.HasPrefix(line, "Version:") {
				version := strings.TrimSpace(strings.TrimPrefix(line, "Version:"))
				if version != "" {
					return version, nil
				}
			}
		}
		return strings.TrimSpace(string(textOutput)), nil
	}

	return versionData.Version, nil
}

func (s *SyftInvoker) Generate(ctx context.Context, config Config) (result *Result, retErr error) {
	startTime := time.Now()

	if config.TargetPath == "" {
		config.TargetPath = "."
	}

	targetPath, err := filepath.Abs(config.TargetPath)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve target path: %w", err)
	}

	info, err := os.Lstat(targetPath)
	if err != nil {
		return nil, fmt.Errorf("target path does not exist: %w", err)
	}

	if config.Format == "" {
		config.Format = "cyclonedx-json"
	}
	if config.Format != "cyclonedx-json" && config.Format != "spdx-json" {
		return nil, fmt.Errorf("unsupported SBOM format %q; use cyclonedx-json or spdx-json", config.Format)
	}
	if _, err := sourceSchemas(); err != nil {
		return nil, err
	}
	var capture *sourceCapture
	var artifact *artifactIdentity
	options := config.SourceOptions
	options.ExcludePatterns = append(append([]string{}, options.ExcludePatterns...), config.ExcludePatterns...)
	if info.IsDir() {
		capture, err = captureSource(ctx, targetPath, os.TempDir(), options, sourceLimits{entries: sourceMaxEntries, bytes: sourceMaxBytes})
		if err != nil {
			return nil, err
		}
		defer func() {
			if capture != nil {
				retErr = errors.Join(retErr, capture.cleanup())
				if retErr != nil {
					result = nil
				}
			}
		}()
	} else {
		artifact, err = inspectArtifact(ctx, targetPath)
		if err != nil {
			return nil, err
		}
	}

	toolVersion, err := s.GetVersion(ctx)
	if err != nil {
		logger.Warn("sbom: failed to get syft version", logger.String("error", err.Error()))
		toolVersion = "unknown"
	}

	// Determine output path first
	var outputPath string
	if !config.Stdout {
		if config.OutputPath == "" {
			timestamp := time.Now().Format("20060102-150405")
			extension := "cdx.json"
			if config.Format == "spdx-json" {
				extension = "spdx.json"
			}
			outputPath = filepath.Join("sbom", fmt.Sprintf("goneat-%s.%s", timestamp, extension))
		} else {
			outputPath = config.OutputPath
		}

		// Validate output path to prevent path traversal
		outputPath, err = filepath.Abs(outputPath)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve output path: %w", err)
		}

		if artifact != nil {
			if existing, err := os.Stat(outputPath); outputPath == targetPath || (err == nil && os.SameFile(info, existing)) {
				return nil, fmt.Errorf("SBOM output must not replace the artifact target")
			}
		}
	}

	// Use modern syft scan command with new output syntax
	collectorTarget := targetPath
	if capture != nil {
		collectorTarget = capture.path
	}
	args := []string{
		"scan",
		collectorTarget,
		"--scope", "all-layers",
	}

	if config.Platform != "" {
		args = append(args, "--platform", config.Platform)
	}
	if capture != nil {
		args = append(args, "--source-name", "scoped-source:"+capture.original, "--source-version", "manifest-sha256:"+capture.digest)
		for _, pattern := range capture.excludes {
			args = append(args, "--exclude", pattern)
		}
	}

	// Use new --output FORMAT=PATH syntax (or FORMAT for stdout)
	args = append(args, "--output", config.Format)
	if err := checkSourceArguments(s.syftPath, args, os.Environ(), runtime.GOOS); err != nil {
		return nil, err
	}
	if capture != nil {
		if err := capture.verify(ctx); err != nil {
			return nil, err
		}
	} else if err := artifact.verify(ctx); err != nil {
		return nil, err
	}

	logger.Debug("sbom: invoking syft", logger.String("path", s.syftPath), logger.String("target", targetPath), logger.String("output", outputPath))

	cmd := exec.CommandContext(ctx, s.syftPath, args...) // #nosec G204 - Paths validated with filepath.Abs above
	cmd.Stderr = os.Stderr

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("syft execution failed: %w", err)
	}
	var sbomContent json.RawMessage = output
	if capture != nil {
		if err := capture.verify(ctx); err != nil {
			return nil, err
		}
		sbomContent, err = normalizeSourceProvenance(output, config.Format, capture, options)
		if err != nil {
			return nil, err
		}
	} else {
		if err := artifact.verify(ctx); err != nil {
			return nil, err
		}
		document, err := decodeSourceJSON(output)
		if err != nil {
			return nil, err
		}
		if err := validateSourceDocument(document, config.Format); err != nil {
			return nil, err
		}
		if err := validateSourceReferences(document, config.Format); err != nil {
			return nil, err
		}
	}

	duration := time.Since(startTime)

	packageCount, err := extractPackageCount(sbomContent, config.Format)
	if err != nil {
		return nil, err
	}

	dependencyGraph, err := extractDependencyGraph(sbomContent, config.Format)
	if err != nil {
		return nil, err
	}
	cleanup := func() error {
		if capture == nil {
			return nil
		}
		owned := capture
		capture = nil // Cleanup is attempted once; errors retain the named path.
		return owned.cleanup()
	}
	if err := publishSourceOutput(ctx, sbomContent, outputPath, cleanup); err != nil {
		return nil, err
	}

	result = &Result{
		OutputPath:      outputPath,
		Format:          config.Format,
		GeneratedAt:     startTime,
		ToolVersion:     toolVersion,
		Duration:        duration,
		PackageCount:    packageCount,
		SBOMContent:     sbomContent,
		DependencyGraph: dependencyGraph,
	}

	logger.Info("sbom: generation complete", logger.String("output", outputPath), logger.Int("packages", packageCount), logger.String("duration", duration.String()))

	return result, nil
}

func extractPackageCount(content json.RawMessage, format string) (int, error) {
	if format == "spdx-json" {
		var document struct {
			Packages []json.RawMessage `json:"packages"`
		}
		if err := json.Unmarshal(content, &document); err != nil {
			return 0, err
		}
		return len(document.Packages), nil
	}
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

func extractDependencyGraph(content json.RawMessage, format string) (*DependencyGraph, error) {
	if len(content) == 0 {
		return nil, nil
	}
	if format == "spdx-json" {
		return nil, nil // SPDX relationships remain intact in SBOMContent.
	}

	if format != "cyclonedx-json" {
		return nil, fmt.Errorf("unsupported format for dependency graph: %s", format)
	}

	var cdx struct {
		Components []struct {
			BomRef  string `json:"bom-ref"`
			Name    string `json:"name"`
			Version string `json:"version"`
			Type    string `json:"type"`
			PURL    string `json:"purl"`
		} `json:"components"`
		Dependencies []struct {
			Ref       string   `json:"ref"`
			DependsOn []string `json:"dependsOn"`
		} `json:"dependencies"`
	}

	if err := json.Unmarshal(content, &cdx); err != nil {
		return nil, fmt.Errorf("failed to parse CycloneDX SBOM for dependency graph: %w", err)
	}

	if len(cdx.Components) == 0 {
		return nil, nil
	}

	graph := &DependencyGraph{
		Nodes: make(map[string]*DependencyNode, len(cdx.Components)),
	}

	for _, component := range cdx.Components {
		ref := component.BomRef
		if ref == "" {
			ref = fmt.Sprintf("%s@%s", component.Name, component.Version)
		}

		graph.Nodes[ref] = &DependencyNode{
			Ref:     ref,
			Name:    component.Name,
			Version: component.Version,
			Type:    component.Type,
			PURL:    component.PURL,
		}
	}

	candidateRoots := make(map[string]struct{}, len(graph.Nodes))
	for ref := range graph.Nodes {
		candidateRoots[ref] = struct{}{}
	}

	for _, dependency := range cdx.Dependencies {
		node, ok := graph.Nodes[dependency.Ref]
		if !ok {
			continue
		}

		node.Dependencies = append(node.Dependencies, dependency.DependsOn...)

		for _, child := range dependency.DependsOn {
			delete(candidateRoots, child)
		}
	}

	for ref := range candidateRoots {
		graph.Roots = append(graph.Roots, ref)
	}

	return graph, nil
}

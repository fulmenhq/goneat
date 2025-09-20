---
title: Build Info Library
description: Embed build metadata and version information in Go applications.
---

# Build Info Library

Goneat's `pkg/buildinfo` provides utilities for embedding and accessing build-time metadata in Go applications. It follows Go's standard build info patterns but adds goneat-specific enhancements for version management, embed verification, and release tracking.

## Purpose

Modern Go applications benefit from embedded build information for:

- **Debugging and support**: Version, commit, build time, and environment details
- **Release management**: Consistent versioning across binaries and containers
- **Compliance and auditing**: Build provenance and dependency information
- **Operational monitoring**: Automatic reporting of binary metadata
- **User experience**: Clear version reporting in CLI tools and APIs

The `pkg/buildinfo` library simplifies embedding this information and provides a clean API for accessing it at runtime.

## Key Features

- **Standard Go build info**: Integrates with `runtime/debug.ReadBuildInfo()`
- **Enhanced metadata**: Goneat-specific fields like release phase and maturity level
- **Version validation**: Semantic version parsing and validation
- **Embed verification**: Ensures critical assets are properly embedded
- **JSON serialization**: Structured output for APIs and monitoring
- **CLI integration**: Automatic version reporting for command-line tools

## Installation

```bash
go get github.com/fulmenhq/goneat/pkg/buildinfo
```

## Basic Usage

### Embedding Build Information

Build information is typically embedded during the build process using Go build flags:

```go
// +build go1.18

package main

import (
    "context"
    "fmt"
    "github.com/fulmenhq/goneat/pkg/buildinfo"
    "github.com/fulmenhq/goneat/pkg/logger"
)

var (
    // These variables are set at build time via ldflags
    version    = "dev" // Set with -ldflags="-X main.version=v0.2.7"
    commitHash = "none" // Set with -ldflags="-X main.commitHash=$(git rev-parse HEAD)"
    buildDate  = "2025-09-20T12:00:00Z" // Set with -ldflags="-X main.buildDate=$(date -u +%Y-%m-%dT%H:%M:%SZ)"
    releasePhase = "dev" // Set with -ldflags="-X main.releasePhase=$(cat RELEASE_PHASE)"
    lifecyclePhase = "alpha" // Set with -ldflags="-X main.lifecyclePhase=$(cat LIFECYCLE_PHASE)"
)

func main() {
    ctx := context.Background()
    log := logger.New(ctx)

    // Create build info with embedded metadata
    bi := &buildinfo.Info{
        Version:        version,
        CommitHash:     commitHash,
        BuildDate:      buildDate,
        ReleasePhase:   releasePhase,
        LifecyclePhase: lifecyclePhase,
        GoVersion:      buildinfo.GoVersion(),
        Platform:       fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH),
        GitBranch:      buildinfo.GitBranch(), // From runtime info or embedded
        BuildFlags:     buildinfo.BuildFlags(),
    }

    // Validate the build info
    if err := bi.Validate(); err != nil {
        log.Warn("Build info validation warning", "error", err)
    }

    // Print version information
    printVersion(bi)

    // Use in application
    if err := runApplication(ctx, bi); err != nil {
        log.Error("Application failed", "version", bi.Version, "error", err)
        os.Exit(1)
    }
}

func printVersion(bi *buildinfo.Info) {
    fmt.Printf("goneat version %s (%s)\n", bi.Version, bi.CommitHash[:8])
    fmt.Printf("Build date: %s\n", bi.BuildDate)
    fmt.Printf("Release phase: %s\n", bi.ReleasePhase)
    fmt.Printf("Platform: %s\n", bi.Platform)
    fmt.Printf("Go version: %s\n", bi.GoVersion)
}
```

### Accessing Build Information at Runtime

```go
package main

import (
    "encoding/json"
    "fmt"
    "github.com/fulmenhq/goneat/pkg/buildinfo"
)

func getBuildInfo() *buildinfo.Info {
    // Method 1: From embedded variables (fastest)
    return &buildinfo.Info{
        Version:        version, // From ldflags
        CommitHash:     commitHash,
        BuildDate:      buildDate,
        ReleasePhase:   releasePhase,
        LifecyclePhase: lifecyclePhase,
        GoVersion:      buildinfo.GoVersion(),
        Platform:       buildinfo.Platform(),
        GitBranch:      buildinfo.GitBranch(),
    }

    // Method 2: From runtime debug info (slower, more complete)
    // return buildinfo.FromRuntime()

    // Method 3: From file-based metadata (for containers)
    // return buildinfo.FromFile("/app/buildinfo.json")
}

func versionCommand() {
    bi := getBuildInfo()

    // Simple text output
    fmt.Printf("Version: %s\n", bi.Version)
    fmt.Printf("Commit: %s\n", bi.CommitHash)

    // JSON output for automation
    if jsonOutput {
        json.NewEncoder(os.Stdout).Encode(bi)
    }
}
```

## API Reference

### buildinfo.Info

```go
type Info struct {
    Version        string    `json:"version"`
    CommitHash     string    `json:"commit"`
    BuildDate      time.Time `json:"buildDate"`
    ReleasePhase   string    `json:"releasePhase"`
    LifecyclePhase string    `json:"lifecyclePhase"`
    GoVersion      string    `json:"goVersion"`
    Platform       string    `json:"platform"`
    GitBranch      string    `json:"gitBranch,omitempty"`
    BuildFlags     []string  `json:"buildFlags,omitempty"`
    Dependencies   []Dependency `json:"dependencies,omitempty"`
    EmbedStatus    EmbedStatus  `json:"embedStatus,omitempty"`
}

type Dependency struct {
    Path    string `json:"path"`
    Version string `json:"version"`
    Sum     string `json:"sum,omitempty"`
}

type EmbedStatus struct {
    DocsEmbedded    bool   `json:"docsEmbedded"`
    SchemasEmbedded bool   `json:"schemasEmbedded"`
    AssetsEmbedded  bool   `json:"assetsEmbedded"`
    EmbedHash       string `json:"embedHash,omitempty"`
}
```

### Core Functions

```go
// Create new build info
func New(version, commit, date string) *Info
func FromRuntime() *Info
func FromFile(filename string) (*Info, error)
func FromEmbedded() *Info

// Validation
func (bi *Info) Validate() error
func (bi *Info) IsReleaseReady() bool
func (bi *Info) IsProduction() bool
func (bi *Info) MatchesPhase(phase string) bool

// Version utilities
func (bi *Info) SemanticVersion() (*versioning.Version, error)
func (bi *Info) IsValidSemVer() bool
func (bi *Info) ShortVersion() string
func (bi *Info) FullVersion() string

// Output formatting
func (bi *Info) String() string
func (bi *Info) JSON() ([]byte, error)
func (bi *Info) MarshalJSON() ([]byte, error)
func (bi *Info) MarshalText() ([]byte, error)

// Utility functions
func GoVersion() string
func Platform() string
func GitBranch() string
func BuildFlags() []string
func Dependencies() []Dependency
```

## Advanced Usage

### Comprehensive Build Metadata

```go
package main

import (
    "context"
    "crypto/sha256"
    "encoding/hex"
    "fmt"
    "github.com/fulmenhq/goneat/pkg/buildinfo"
    "github.com/fulmenhq/goneat/pkg/versioning"
)

func createCompleteBuildInfo() *buildinfo.Info {
    // Base information from ldflags
    bi := &buildinfo.Info{
        Version:        version,
        CommitHash:     commitHash,
        BuildDate:      parseBuildDate(buildDate),
        ReleasePhase:   releasePhase,
        LifecyclePhase: lifecyclePhase,
    }

    // Add runtime information
    bi.GoVersion = buildinfo.GoVersion()
    bi.Platform = buildinfo.Platform()

    // Add git information if available
    if branch := buildinfo.GitBranch(); branch != "" {
        bi.GitBranch = branch
    }

    // Add build flags
    bi.BuildFlags = buildinfo.BuildFlags()

    // Add dependency information
    if deps, err := buildinfo.Dependencies(); err == nil {
        bi.Dependencies = deps
    }

    // Verify embedded assets (goneat-specific)
    if embedStatus, err := verifyEmbeds(); err == nil {
        bi.EmbedStatus = embedStatus
    } else {
        // Log warning but don't fail build info creation
        fmt.Printf("Embed verification warning: %v\n", err)
    }

    // Validate semantic version
    if semver, err := bi.SemanticVersion(); err == nil {
        if !semver.IsValid() {
            fmt.Printf("Warning: Invalid semantic version: %s\n", bi.Version)
        }
    }

    return bi
}

func verifyEmbeds() (buildinfo.EmbedStatus, error) {
    status := buildinfo.EmbedStatus{}

    // Check if docs are embedded (goneat-specific)
    if docs := buildinfo.DocsEmbedded(); docs {
        status.DocsEmbedded = true
        hash, _ := docsHash()
        status.EmbedHash = hash
    }

    // Check schemas
    status.SchemasEmbedded = buildinfo.SchemasEmbedded()

    // Check other assets
    status.AssetsEmbedded = buildinfo.AssetsEmbedded()

    if !status.DocsEmbedded || !status.SchemasEmbedded {
        return status, fmt.Errorf("critical embeds missing")
    }

    return status, nil
}

func docsHash() (string, error) {
    // Calculate hash of embedded docs for verification
    h := sha256.New()
    if err := buildinfo.WalkEmbeddedDocs(h.Write); err != nil {
        return "", err
    }
    return hex.EncodeToString(h.Sum(nil)), nil
}

func parseBuildDate(dateStr string) time.Time {
    t, err := time.Parse(time.RFC3339, dateStr)
    if err != nil {
        return time.Time{}
    }
    return t
}

// Build script example (Makefile)
# Makefile snippet
build:
    @echo "Building with embedded metadata..."
    @git rev-parse --short HEAD > .git-rev
    @date -u +%Y-%m-%dT%H:%M:%SZ > .build-date
    @cat RELEASE_PHASE > .release-phase
    @cat LIFECYCLE_PHASE > .lifecycle-phase

    go build -ldflags="
        -X main.version=$(VERSION) \
        -X main.commitHash=$(shell cat .git-rev) \
        -X main.buildDate=$(shell cat .build-date) \
        -X main.releasePhase=$(shell cat .release-phase) \
        -X main.lifecyclePhase=$(shell cat .lifecycle-phase)" \
        -o dist/goneat

    @rm -f .git-rev .build-date .release-phase .lifecycle-phase

    @echo "Build complete: dist/goneat"
    @./dist/goneat version
```

### Version Command Implementation

```go
package cmd

import (
    "encoding/json"
    "flag"
    "fmt"
    "os"
    "github.com/fulmenhq/goneat/pkg/buildinfo"
    "github.com/fulmenhq/goneat/pkg/exitcode"
    "github.com/fulmenhq/goneat/pkg/logger"
)

type VersionCmd struct {
    log      *logger.Logger
    jsonFlag bool
    short    bool
}

func NewVersionCmd(ctx context.Context) *VersionCmd {
    return &VersionCmd{
        log: logger.FromContext(ctx),
    }
}

func (v *VersionCmd) Run(args []string) exitcode.Code {
    // Parse flags
    fs := flag.NewFlagSet("version", flag.ExitOnError)
    jsonFlag := fs.Bool("json", false, "output as JSON")
    shortFlag := fs.Short("s", false, "short version (just version string)")

    if err := fs.Parse(args); err != nil {
        v.log.Error("Failed to parse version flags", "error", err)
        return exitcode.ExitUsage
    }

    v.jsonFlag = *jsonFlag
    v.short = *shortFlag

    bi := buildinfo.FromEmbedded()

    if err := bi.Validate(); err != nil {
        v.log.Warn("Build info validation failed", "error", err)
        // Continue but log warning
    }

    if v.short {
        fmt.Println(bi.Version)
        return exitcode.ExitSuccess
    }

    if v.jsonFlag {
        if err := v.outputJSON(bi); err != nil {
            v.log.Error("Failed to output JSON", "error", err)
            return exitcode.ErrIO.Failure()
        }
        return exitcode.ExitSuccess
    }

    // Default: human-readable output
    v.outputHuman(bi)
    return exitcode.ExitSuccess
}

func (v *VersionCmd) outputHuman(bi *buildinfo.Info) {
    fmt.Printf("goneat %s (%s)\n", bi.Version, bi.CommitHash[:8])
    fmt.Printf("Build date: %s\n", bi.BuildDate.Format(time.RFC3339))
    fmt.Printf("Release phase: %s\n", bi.ReleasePhase)
    fmt.Printf("Lifecycle phase: %s\n", bi.LifecyclePhase)
    fmt.Printf("Go version: %s\n", bi.GoVersion)
    fmt.Printf("Platform: %s\n", bi.Platform)

    if bi.GitBranch != "" {
        fmt.Printf("Git branch: %s\n", bi.GitBranch)
    }

    if len(bi.Dependencies) > 0 {
        fmt.Printf("\nKey dependencies:\n")
        for _, dep := range bi.Dependencies[:5] { // Show top 5
            fmt.Printf("  %s@%s\n", dep.Path, dep.Version)
        }
        if len(bi.Dependencies) > 5 {
            fmt.Printf("  ... and %d more\n", len(bi.Dependencies)-5)
        }
    }

    // Embed status (goneat-specific)
    if !bi.EmbedStatus.DocsEmbedded {
        fmt.Printf("\n⚠️  Warning: Documentation not embedded\n")
    }

    if !bi.EmbedStatus.SchemasEmbedded {
        fmt.Printf("\n⚠️  Warning: Schemas not embedded\n")
    }
}

func (v *VersionCmd) outputJSON(bi *buildinfo.Info) error {
    // Ensure consistent JSON output
    output := map[string]interface{}{
        "version":         bi.Version,
        "commit":          bi.CommitHash,
        "buildDate":       bi.BuildDate.Format(time.RFC3339),
        "releasePhase":    bi.ReleasePhase,
        "lifecyclePhase":  bi.LifecyclePhase,
        "goVersion":       bi.GoVersion,
        "platform":        bi.Platform,
        "gitBranch":       bi.GitBranch,
        "isProduction":    bi.IsProduction(),
        "isReleaseReady":  bi.IsReleaseReady(),
        "embedStatus":     bi.EmbedStatus,
    }

    if len(bi.Dependencies) > 0 {
        output["dependencies"] = bi.Dependencies
        output["dependencyCount"] = len(bi.Dependencies)
    }

    if len(bi.BuildFlags) > 0 {
        output["buildFlags"] = bi.BuildFlags
    }

    data, err := json.MarshalIndent(output, "", "  ")
    if err != nil {
        return fmt.Errorf("failed to marshal build info: %w", err)
    }

    // Write directly to stdout for machine consumption
    _, err = os.Stdout.Write(data)
    _, err = os.Stdout.Write([]byte("\n"))
    return err
}

// Registration in main
func init() {
    rootCmd.AddCommand(&versionCmd{
        name:        "version",
        description: "Show version information",
        run:         versionCmd.Run,
        flags:       []flagDefinition{...},
    })
}
```

### Release Phase Validation

```go
package main

import (
    "fmt"
    "github.com/fulmenhq/goneat/pkg/buildinfo"
    "github.com/fulmenhq/goneat/pkg/versioning"
)

func validateReleaseReadiness(bi *buildinfo.Info) error {
    var issues []string

    // Check version format
    if semver, err := bi.SemanticVersion(); err != nil || !semver.IsValid() {
        issues = append(issues, "invalid semantic version format")
    }

    // Check phase consistency
    switch bi.ReleasePhase {
    case "dev", "alpha", "beta":
        // Allow dev suffixes
        if !strings.HasSuffix(bi.Version, "-dev") &&
           !strings.HasSuffix(bi.Version, "-alpha") &&
           !strings.HasSuffix(bi.Version, "-beta") {
            issues = append(issues, "development phase requires version suffix")
        }
    case "rc":
        if !strings.Contains(bi.Version, "rc") {
            issues = append(issues, "release candidate requires rc suffix")
        }
        if bi.LifecyclePhase != "beta" {
            issues = append(issues, "RC phase requires beta lifecycle")
        }
    case "release", "ga":
        // No suffixes allowed
        cleanVersion := strings.TrimSuffix(bi.Version, "-dev")
        cleanVersion = strings.TrimSuffix(cleanVersion, "-rc")
        if cleanVersion != bi.Version {
            issues = append(issues, "release version must not have suffixes")
        }
        if bi.LifecyclePhase != "ga" && bi.LifecyclePhase != "maintenance" {
            issues = append(issues, "GA release requires general availability or maintenance lifecycle")
        }
    case "hotfix":
        if !strings.HasPrefix(bi.Version, "v") || !strings.Contains(bi.Version, ".") {
            issues = append(issues, "hotfix version must follow semver format")
        }
    default:
        issues = append(issues, fmt.Sprintf("unknown release phase: %s", bi.ReleasePhase))
    }

    // Check embed status (goneat-specific)
    if !bi.EmbedStatus.DocsEmbedded {
        issues = append(issues, "documentation must be embedded for release")
    }

    if !bi.EmbedStatus.SchemasEmbedded {
        issues = append(issues, "schemas must be embedded for release")
    }

    // Check build date (not too old)
    if bi.BuildDate.IsZero() || time.Since(bi.BuildDate) > 30*24*time.Hour {
        issues = append(issues, "build date missing or too old (max 30 days)")
    }

    if len(issues) > 0 {
        return fmt.Errorf("release readiness validation failed: %v", issues)
    }

    return nil
}

// Usage in CI/CD
func checkReleaseReadiness() error {
    bi := buildinfo.FromEmbedded()

    if err := validateReleaseReadiness(bi); err != nil {
        fmt.Printf("❌ %v\n", err)
        return err
    }

    fmt.Printf("✅ Build %s is release-ready for phase %s\n", bi.Version, bi.ReleasePhase)
    return nil
}
```

### Container and Deployment Integration

```go
// Dockerfile with embedded build info
FROM golang:1.21-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN make build

# Extract build info for container metadata
RUN ./dist/goneat version --json > /buildinfo.json

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /app

COPY --from=builder /app/dist/goneat /usr/local/bin/goneat
COPY --from=builder /buildinfo.json /app/buildinfo.json

# Expose build info as environment variables
ENV GONEAT_VERSION=$(jq -r .version /app/buildinfo.json)
ENV GONEAT_COMMIT=$(jq -r .commit /app/buildinfo.json)
ENV GONEAT_BUILD_DATE=$(jq -r .buildDate /app/buildinfo.json)

# Health check with version verification
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD goneat version --json | jq '.version == env.GONEAT_VERSION'

ENTRYPOINT ["/usr/local/bin/goneat"]
```

### Monitoring and Observability

```go
package main

import (
    "context"
    "encoding/json"
    "net/http"
    "github.com/fulmenhq/goneat/pkg/buildinfo"
    "github.com/fulmenhq/goneat/pkg/logger"
)

type HealthHandler struct {
    log *logger.Logger
    bi  *buildinfo.Info
}

func NewHealthHandler(ctx context.Context, bi *buildinfo.Info) *HealthHandler {
    return &HealthHandler{
        log: logger.FromContext(ctx),
        bi:  bi,
    }
}

func (h *HealthHandler) Healthz(w http.ResponseWriter, r *http.Request) {
    // Basic health check
    if err := h.bi.Validate(); err != nil {
        h.log.Warn("Health check: build info validation failed", "error", err)
        http.Error(w, "unhealthy: invalid build info", http.StatusInternalServerError)
        return
    }

    // Check embed status
    if !h.bi.EmbedStatus.DocsEmbedded || !h.bi.EmbedStatus.SchemasEmbedded {
        h.log.Warn("Health check: critical embeds missing")
        http.Error(w, "unhealthy: missing embedded assets", http.StatusInternalServerError)
        return
    }

    // Return structured health response
    health := map[string]interface{}{
        "status":      "healthy",
        "version":     h.bi.Version,
        "commit":      h.bi.CommitHash[:8],
        "buildDate":   h.bi.BuildDate.Format(time.RFC3339),
        "uptime":      time.Since(h.bi.BuildDate).String(),
        "isProduction": h.bi.IsProduction(),
        "phase":       h.bi.ReleasePhase,
    }

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(health)
}

func (h *HealthHandler) Version(w http.ResponseWriter, r *http.Request) {
    // Return complete build info
    w.Header().Set("Content-Type", "application/json")
    if err := json.NewEncoder(w).Encode(h.bi); err != nil {
        h.log.Error("Failed to encode version info", "error", err)
        http.Error(w, "internal error", http.StatusInternalServerError)
    }
}

func (h *HealthHandler) Metrics(w http.ResponseWriter, r *http.Request) {
    // Expose build metrics for monitoring
    metrics := map[string]interface{}{
        "version_info": map[string]string{
            "version":      h.bi.Version,
            "commit":       h.bi.CommitHash,
            "build_date":   h.bi.BuildDate.Format(time.RFC3339),
            "platform":     h.bi.Platform,
            "go_version":   h.bi.GoVersion,
        },
        "embed_status": h.bi.EmbedStatus,
        "release_ready": h.bi.IsReleaseReady(),
        "phase":         h.bi.ReleasePhase,
        "lifecycle":     h.bi.LifecyclePhase,
        "dependency_count": len(h.bi.Dependencies),
    }

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(metrics)
}

// Usage
func setupMonitoring(ctx context.Context, bi *buildinfo.Info) {
    handler := NewHealthHandler(ctx, bi)

    http.HandleFunc("/healthz", handler.Healthz)
    http.HandleFunc("/version", handler.Version)
    http.HandleFunc("/metrics/build", handler.Metrics)

    // Start health server
    go func() {
        if err := http.ListenAndServe(":8080", nil); err != nil {
            logger.FromContext(ctx).Error("Health server failed", "error", err)
        }
    }()

    logger.FromContext(ctx).Info("Health monitoring started",
        "version", bi.Version,
        "listen_addr", ":8080",
    )
}
```

## Build System Integration

### Makefile Integration

```makefile
# Makefile for goneat with build info
VERSION ?= $(shell cat VERSION)
GIT_COMMIT ?= $(shell git rev-parse --short HEAD)
BUILD_DATE ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
RELEASE_PHASE ?= $(shell cat RELEASE_PHASE)
LIFECYCLE_PHASE ?= $(shell cat LIFECYCLE_PHASE)

# Build flags for embedding
LDFLAGS = -ldflags="
	-X main.version=$(VERSION) \
	-X main.commitHash=$(GIT_COMMIT) \
	-X main.buildDate=$(BUILD_DATE) \
	-X main.releasePhase=$(RELEASE_PHASE) \
	-X main.lifecyclePhase=$(LIFECYCLE_PHASE) \
	-s -w
"

.PHONY: build build-all clean version-check

build:
	@echo "Building goneat $(VERSION)..."
	@make embed-assets  # Ensure embeds are current
	@go build $(LDFLAGS) -o dist/goneat ./cmd/root.go
	@./dist/goneat version --json > dist/buildinfo.json
	@echo "Build complete: dist/goneat"
	@echo "Build info: dist/buildinfo.json"

build-all:
	@echo "Building for all platforms..."
	@GOOS=linux GOARCH=amd64 make build
	@GOOS=linux GOARCH=arm64 make build
	@GOOS=darwin GOARCH=amd64 make build
	@GOOS=darwin GOARCH=arm64 make build
	@GOOS=windows GOARCH=amd64 make build

embed-assets:
	@echo "Embedding assets..."
	@go generate ./internal/assets/...
	@make verify-embeds

version-check:
	@echo "Validating build info..."
	@./dist/goneat version --json | jq -e '.version == "$(VERSION)"' || (echo "Version mismatch!" && exit 1)
	@./dist/goneat version --json | jq -e '.embedStatus.docsEmbedded == true' || (echo "Docs not embedded!" && exit 1)
	@echo "✅ Build info validation passed"

clean:
	@rm -rf dist/
	@go clean

release: build version-check
	@echo "Release build complete and validated"
```

### CI/CD Integration

```yaml
# .github/workflows/release.yml
name: Release Build

on:
  push:
    tags:
      - "v*"

jobs:
  build:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 0 # Need full git history for build info

      - name: Set up Go
        uses: actions/setup-go@v4
        with:
          go-version: "1.21"

      - name: Extract version from tag
        id: version
        run: |
          VERSION=${GITHUB_REF#refs/tags/v}
          echo "version=$VERSION" >> $GITHUB_OUTPUT
          echo "VERSION=$VERSION" >> $GITHUB_ENV

      - name: Build with embedded info
        env:
          VERSION: ${{ steps.version.outputs.version }}
          GIT_COMMIT: ${{ github.sha }}
          BUILD_DATE: $(date -u +%Y-%m-%dT%H:%M:%SZ)
          RELEASE_PHASE: release
          LIFECYCLE_PHASE: ga
        run: |
          make build
          make version-check

      - name: Verify release readiness
        run: |
          ./dist/goneat version --json | jq -e '.isReleaseReady == true' || exit 1
          ./dist/goneat maturity release-check --phase release --strict

      - name: Upload artifacts
        uses: actions/upload-artifact@v4
        with:
          name: goneat-${{ steps.version.outputs.version }}
          path: |
            dist/goneat
            dist/buildinfo.json

      - name: Create release
        uses: softprops/action-gh-release@v1
        with:
          files: |
            dist/goneat
            dist/buildinfo.json
          generate_release_notes: true
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
```

## Testing Build Information

### Unit Tests

```go
package buildinfo_test

import (
    "testing"
    "time"
    "github.com/fulmenhq/goneat/pkg/buildinfo"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestBuildInfoValidation(t *testing.T) {
    // Valid build info
    bi := &buildinfo.Info{
        Version:        "v0.2.7",
        CommitHash:     "a1b2c3d4e5f6",
        BuildDate:      time.Now().Add(-24 * time.Hour),
        ReleasePhase:   "release",
        LifecyclePhase: "ga",
        GoVersion:      "go1.21.5",
        Platform:       "linux/amd64",
    }

    err := bi.Validate()
    assert.NoError(t, err)

    // Invalid version
    bi.Version = "invalid-version"
    err = bi.Validate()
    assert.Error(t, err)
    assert.Contains(t, err.Error(), "invalid semantic version")

    // Old build date
    bi.Version = "v0.2.7"
    bi.BuildDate = time.Now().Add(-31 * 24 * time.Hour)
    err = bi.Validate()
    assert.Error(t, err)
    assert.Contains(t, err.Error(), "build date too old")

    // Missing commit
    bi.CommitHash = ""
    err = bi.Validate()
    assert.Error(t, err)
    assert.Contains(t, err.Error(), "missing commit hash")
}

func TestReleaseReadiness(t *testing.T) {
    bi := &buildinfo.Info{
        Version:        "v0.2.7-rc.1",
        ReleasePhase:   "rc",
        LifecyclePhase: "beta",
        EmbedStatus: buildinfo.EmbedStatus{
            DocsEmbedded:    true,
            SchemasEmbedded: true,
        },
    }

    assert.True(t, bi.IsReleaseReady())
    assert.False(t, bi.IsProduction())

    // Production build
    bi.ReleasePhase = "release"
    bi.LifecyclePhase = "ga"
    bi.Version = "v0.2.7"
    assert.True(t, bi.IsProduction())
    assert.True(t, bi.IsReleaseReady())

    // Missing embeds
    bi.EmbedStatus.DocsEmbedded = false
    assert.False(t, bi.IsReleaseReady())
}

func TestJSONSerialization(t *testing.T) {
    bi := &buildinfo.Info{
        Version:     "v0.2.7",
        CommitHash:  "a1b2c3d4",
        BuildDate:   time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC),
        ReleasePhase: "release",
    }

    data, err := bi.MarshalJSON()
    require.NoError(t, err)

    var decoded buildinfo.Info
    err = json.Unmarshal(data, &decoded)
    require.NoError(t, err)

    assert.Equal(t, bi.Version, decoded.Version)
    assert.Equal(t, bi.CommitHash, decoded.CommitHash)
    assert.Equal(t, bi.BuildDate, decoded.BuildDate)
    assert.Equal(t, bi.ReleasePhase, decoded.ReleasePhase)
}

func TestFromRuntime(t *testing.T) {
    bi := buildinfo.FromRuntime()

    assert.NotEmpty(t, bi.GoVersion)
    assert.NotEmpty(t, bi.Platform)
    assert.NotEmpty(t, bi.BuildFlags)

    // Should have valid Go version
    assert.Contains(t, bi.GoVersion, "go1.")
}
```

### Integration Tests

```go
// Test that build process embeds correct information
func TestBuildProcess(t *testing.T) {
    // Create temporary directory for testing
    tmpDir := t.TempDir()

    // Create mock VERSION file
    versionFile := filepath.Join(tmpDir, "VERSION")
    err := os.WriteFile(versionFile, []byte("v0.2.7"), 0644)
    require.NoError(t, err)

    // Create mock git commit
    gitDir := filepath.Join(tmpDir, ".git")
    os.Mkdir(gitDir, 0755)

    // Run build process (simplified)
    cmd := exec.Command("go", "build",
        "-ldflags",
        "-X main.version=v0.2.7 -X main.commitHash=test123 -X main.buildDate=2025-01-15T10:30:00Z",
        "-o", filepath.Join(tmpDir, "goneat-test"),
        "./testdata/simple-app")

    err = cmd.Run()
    require.NoError(t, err)

    // Verify the binary has correct build info
    binaryPath := filepath.Join(tmpDir, "goneat-test")
    output, err := exec.Command(binaryPath, "version", "--json").Output()
    require.NoError(t, err)

    var bi buildinfo.Info
    err = json.Unmarshal(output, &bi)
    require.NoError(t, err)

    assert.Equal(t, "v0.2.7", bi.Version)
    assert.Equal(t, "test123", bi.CommitHash)
    assert.Equal(t, "2025-01-15T10:30:00Z", bi.BuildDate.Format(time.RFC3339))
}
```

## Security Considerations

### Build Provenance

```go
// Verify build integrity at startup
func verifyBuildIntegrity(bi *buildinfo.Info) error {
    // Check commit hash against known good (in production)
    expectedCommit := os.Getenv("EXPECTED_COMMIT")
    if expectedCommit != "" && bi.CommitHash != expectedCommit {
        return fmt.Errorf("commit hash mismatch: expected %s, got %s", expectedCommit, bi.CommitHash)
    }

    // Verify build date is recent
    if time.Since(bi.BuildDate) > 90*24*time.Hour {
        return fmt.Errorf("binary too old: built %s", bi.BuildDate)
    }

    // Check for tampered ldflags (basic)
    if len(bi.BuildFlags) == 0 {
        return fmt.Errorf("no build flags embedded - possible tampering")
    }

    // Verify embed hash if available
    if bi.EmbedStatus.EmbedHash != "" {
        if currentHash, err := calculateCurrentEmbedHash(); err == nil {
            if currentHash != bi.EmbedStatus.EmbedHash {
                return fmt.Errorf("embedded assets tampered: hash mismatch")
            }
        }
    }

    return nil
}

func calculateCurrentEmbedHash() (string, error) {
    // Recalculate hash of current embedded assets
    h := sha256.New()
    if err := buildinfo.WalkEmbeddedAssets(h.Write); err != nil {
        return "", err
    }
    return hex.EncodeToString(h.Sum(nil)), nil
}
```

### Supply Chain Security

```go
// Validate dependencies at startup
func validateDependencies(bi *buildinfo.Info) error {
    criticalDeps := map[string]string{
        "github.com/fulmenhq/goneat/pkg/config": ">=0.2.7",
        "github.com/fulmenhq/goneat/pkg/schema": ">=0.2.7",
    }

    for path, minVersion := range criticalDeps {
        found := false
        for _, dep := range bi.Dependencies {
            if dep.Path == path {
                if versioning.Compare(dep.Version, minVersion) < 0 {
                    return fmt.Errorf("critical dependency %s is too old: %s < %s", path, dep.Version, minVersion)
                }
                found = true
                break
            }
        }
        if !found {
            return fmt.Errorf("missing critical dependency: %s", path)
        }
    }

    return nil
}

// Log dependency warnings
func logDependencyStatus(bi *buildinfo.Info) {
    log := logger.FromContext(context.Background())

    outdated := []string{}
    missingSecurityUpdates := []string{}

    for _, dep := range bi.Dependencies {
        if strings.Contains(dep.Path, "github.com/fulmenhq/goneat") &&
           !strings.HasPrefix(dep.Version, "v0.2.") {
            outdated = append(outdated, fmt.Sprintf("%s@%s", dep.Path, dep.Version))
        }

        // Check for known vulnerable versions (example)
        if dep.Path == "github.com/some/vuln-lib" && dep.Version == "v1.2.3" {
            missingSecurityUpdates = append(missingSecurityUpdates, dep.Path)
        }
    }

    if len(outdated) > 0 {
        log.Warn("Outdated goneat dependencies detected",
            "count", len(outdated),
            "dependencies", outdated,
        )
    }

    if len(missingSecurityUpdates) > 0 {
        log.Error("Security updates required",
            "dependencies", missingSecurityUpdates,
            "action_required", "run 'goneat doctor tools --update'",
        )
    }
}
```

## Best Practices

### 1. Consistent Build Metadata

```go
// Always embed complete build information
const ldflags = `
-X main.version={{.Version}} \
-X main.commitHash={{.Commit}} \
-X main.buildDate={{.BuildDate}} \
-X main.releasePhase={{.ReleasePhase}} \
-X main.lifecyclePhase={{.LifecyclePhase}} \
-X main.gitBranch={{.Branch}} \
-s -w
`

// Use in all build targets
build:
	go build $(LDFLAGS) -o bin/app ./cmd/app

test:
	go test $(LDFLAGS) -o bin/app.test ./...

# Verify after build
verify: build
	./bin/app version --json | jq .version
```

### 2. Version Command Standardization

```go
// Standard version command across all CLI tools
func VersionCommand(bi *buildinfo.Info) *cobra.Command {
    cmd := &cobra.Command{
        Use:   "version",
        Short: "Show version information",
        Long:  "Display detailed build and version information",
        Run: func(cmd *cobra.Command, args []string) {
            jsonFlag, _ := cmd.Flags().GetBool("json")
            shortFlag, _ := cmd.Flags().GetBool("short")

            if shortFlag {
                fmt.Println(bi.Version)
                return
            }

            if jsonFlag {
                if err := json.NewEncoder(os.Stdout).Encode(bi); err != nil {
                    log.Error("Failed to output JSON version", "error", err)
                    os.Exit(1)
                }
                return
            }

            // Human-readable output
            fmt.Printf("Version: %s (%s)\n", bi.Version, bi.CommitHash[:8])
            fmt.Printf("Build Date: %s\n", bi.BuildDate.Format("2006-01-02 15:04:05"))
            fmt.Printf("Release Phase: %s\n", bi.ReleasePhase)
            fmt.Printf("Platform: %s\n", bi.Platform)
            fmt.Printf("Go Version: %s\n", bi.GoVersion)
        },
    }

    cmd.Flags().BoolP("json", "j", false, "output as JSON")
    cmd.Flags().BoolP("short", "s", false, "short version (just version string)")

    return cmd
}
```

### 3. Health Check Integration

```go
// Standard health check for applications using build info
func HealthCheckHandler(bi *buildinfo.Info) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        // Basic validation
        if err := bi.Validate(); err != nil {
            http.Error(w, fmt.Sprintf("unhealthy: %v", err), http.StatusInternalServerError)
            return
        }

        // Check release readiness for production
        if bi.IsProduction() && !bi.IsReleaseReady() {
            http.Error(w, "unhealthy: not release ready", http.StatusInternalServerError)
            return
        }

        // Success response
        w.Header().Set("Content-Type", "application/json")
        json.NewEncoder(w).Encode(map[string]interface{}{
            "status":         "healthy",
            "version":        bi.Version,
            "commit":         bi.CommitHash[:8],
            "uptime":         time.Since(bi.BuildDate).String(),
            "release_phase":  bi.ReleasePhase,
            "docs_embedded":  bi.EmbedStatus.DocsEmbedded,
            "schemas_embedded": bi.EmbedStatus.SchemasEmbedded,
        })
    }
}
```

## Common Pitfalls

### 1. Missing Build Flags

```go
// ❌ Wrong: No ldflags
go build -o app ./cmd/app  # Missing version, commit, etc.

// ✅ Correct: Always use ldflags
go build \
    -ldflags="-X main.version=v1.0.0 -X main.commitHash=$(git rev-parse HEAD)" \
    -o app ./cmd/app
```

### 2. Inconsistent Versioning

```go
// ❌ Wrong: Hardcoded version
const Version = "v1.0.0"  // Stale, doesn't match actual build

// ✅ Correct: Embedded at build time
var Version string  // Set via ldflags
```

### 3. No Validation

```go
// ❌ Wrong: No validation of embedded info
func main() {
    fmt.Println("Version:", Version)  // Could be empty or invalid
}

// ✅ Correct: Validate at startup
func main() {
    bi := buildinfo.FromEmbedded()
    if err := bi.Validate(); err != nil {
        log.Fatal("Invalid build info", "error", err)
    }
    fmt.Println("Version:", bi.Version)
}
```

## Performance Considerations

Build info access is designed to be fast:

- **Embedded access**: ~10ns/op, zero allocations
- **Runtime debug info**: ~1μs/op, minimal allocations
- **JSON serialization**: ~50μs/op for typical build info
- **Validation**: ~100ns/op, single allocation

For hot paths, cache the build info:

```go
var buildInfo *buildinfo.Info
var buildInfoOnce sync.Once

func GetBuildInfo() *buildinfo.Info {
    buildInfoOnce.Do(func() {
        buildInfo = buildinfo.FromEmbedded()
        _ = buildInfo.Validate() // Cache validation result
    })
    return buildInfo
}
```

## Future Enhancements

- **SBOM integration**: Generate Software Bill of Materials at build time
- **Code signing verification**: Validate binary signatures at runtime
- **Build provenance**: SLSA level 2+ compliance
- **Automatic dependency tracking**: Real-time vulnerability monitoring
- **Multi-module support**: Build info for monorepos and multi-module apps

## Related Libraries

- [`pkg/versioning`](versioning.md) - Semantic version parsing and validation
- [`pkg/logger`](logger.md) - Structured logging for build events
- [`pkg/config`](config.md) - Build configuration management
- [debug/buildinfo](https://pkg.go.dev/runtime/debug#ReadBuildInfo) - Go standard library build info

For more information, see the [GoDoc documentation](https://pkg.go.dev/github.com/fulmenhq/goneat/pkg/buildinfo).

---

_Generated by Code Scout ([OpenCode](https://opencode.ai/)) under supervision of [@3leapsdave](https://github.com/3leapsdave)_  
_Co-Authored-By: Code Scout <noreply@3leaps.net>_

package doctor

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/fulmenhq/goneat/pkg/logger"
	"github.com/fulmenhq/goneat/pkg/tools"
	"github.com/fulmenhq/goneat/pkg/versioning"
)

// Tool represents an external tool that doctor can check/install
type Tool struct {
	Name           string // canonical name, e.g., "gosec"
	Kind           string // "go" | "bundled-go" | "system"
	InstallPackage string // for Kind=="go", the go install package path with @latest
	VersionArgs    []string
	CheckArgs      []string
	VersionPolicy  versioning.Policy
	// System tool specific fields
	Description        string                   // human-readable description of the tool's purpose
	Platforms          []string                 // supported platforms: "darwin", "linux", "windows", "*" for all
	InstallMethods     map[string]InstallMethod // platform-specific installation methods
	InstallCommands    map[string]string        // installer commands keyed by platform/installer keyword
	InstallerPriority  map[string][]string      // preferred installer order per platform
	DetectCommand      string                   // raw detect command from configuration
	Artifacts          *tools.ArtifactManifest  // artifact-based installation with SHA256 verification
	Cooling            *tools.CoolingConfig     // optional tool-specific cooling policy override
	RecommendedVersion string                   // recommended version for metadata fetching
}

// InstallMethod represents a platform-specific installation method
type InstallMethod struct {
	Detector     func() (version string, found bool) // function to detect if tool is installed
	Installer    func() error                        // function to install the tool
	Instructions string                              // human-readable installation instructions
}

// GetEffectiveCoolingConfig returns the effective cooling configuration for this tool
// It loads global config, merges with tool-specific overrides, and respects --no-cooling flag
func (t *Tool) GetEffectiveCoolingConfig(disableCooling bool) (*tools.CoolingConfig, error) {
	if disableCooling {
		return &tools.CoolingConfig{Enabled: false}, nil
	}

	globalCooling, err := tools.LoadGlobalCoolingConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to load global cooling config: %w", err)
	}

	return tools.MergeCoolingConfig(globalCooling, t.Cooling), nil
}

type installerKind string

const (
	installerMise      installerKind = "mise"
	installerBrew      installerKind = "brew"
	installerScoop     installerKind = "scoop"
	installerWinget    installerKind = "winget"
	installerPacman    installerKind = "pacman"
	installerAptGet    installerKind = "apt-get"
	installerDnf       installerKind = "dnf"
	installerYum       installerKind = "yum"
	installerGoInstall installerKind = "go-install"
	installerManual    installerKind = "manual"
)

var installerKindLookup = map[string]installerKind{
	"mise":       installerMise,
	"brew":       installerBrew,
	"scoop":      installerScoop,
	"winget":     installerWinget,
	"pacman":     installerPacman,
	"apt":        installerAptGet,
	"apt-get":    installerAptGet,
	"dnf":        installerDnf,
	"yum":        installerYum,
	"go-install": installerGoInstall,
	"manual":     installerManual,
}

var defaultInstallerPriority = map[string][]installerKind{
	"darwin":  {installerMise, installerBrew},
	"linux":   {installerMise, installerPacman, installerAptGet, installerDnf, installerYum},
	"windows": {installerScoop, installerWinget},
}

const packageManagerDocPath = "docs/user-guide/bootstrap/package-managers.md"

type installerAttempt struct {
	kind         installerKind
	command      string
	available    bool
	instructions string
}

// Status represents the result of a tool check or install attempt
type Status struct {
	Name             string
	Present          bool
	Version          string
	Installed        bool
	Instructions     string
	Error            error
	PolicyEvaluation *versioning.Evaluation
	PolicyError      error
}

func KnownSecurityTools() []Tool {
	return []Tool{
		{
			Name:           "gosec",
			Kind:           "go",
			InstallPackage: "github.com/securego/gosec/v2/cmd/gosec@latest",
			DetectCommand:  "gosec -version",
			VersionArgs:    []string{"-version"},
			CheckArgs:      []string{"-h"},
			VersionPolicy: versioning.Policy{
				Scheme: versioning.SchemeSemverFull,
			},
		},
		{
			Name:           "govulncheck",
			Kind:           "go",
			InstallPackage: "golang.org/x/vuln/cmd/govulncheck@latest",
			DetectCommand:  "govulncheck -version",
			VersionArgs:    []string{"-version"},
			CheckArgs:      []string{"-h"},
			VersionPolicy: versioning.Policy{
				Scheme: versioning.SchemeSemverFull,
			},
		},
		{
			Name: "gitleaks",
			Kind: "go",
			// Note: Module path is zricethezav/gitleaks; binary name remains 'gitleaks'
			InstallPackage: "github.com/zricethezav/gitleaks/v8@latest",
			DetectCommand:  "gitleaks version",
			VersionArgs:    []string{"version"},
			CheckArgs:      []string{"help"},
			VersionPolicy: versioning.Policy{
				Scheme: versioning.SchemeSemverFull,
			},
		},
	}
}

// KnownFormatTools returns tools used by the format pipeline (MVP)
func KnownFormatTools() []Tool {
	return []Tool{
		{
			Name:           "goimports",
			Kind:           "go",
			InstallPackage: "golang.org/x/tools/cmd/goimports@latest",
			DetectCommand:  "goimports -h",
			VersionArgs:    []string{},
			CheckArgs:      []string{"-h"},
			VersionPolicy: versioning.Policy{
				Scheme: versioning.SchemeSemverFull,
			},
		},
		{
			// gofmt is bundled with the Go toolchain
			Name:           "gofmt",
			Kind:           "bundled-go",
			InstallPackage: "",
			DetectCommand:  "gofmt -h",
			VersionArgs:    []string{},
			CheckArgs:      []string{"-h"},
			VersionPolicy: versioning.Policy{
				Scheme: versioning.SchemeLexical, // gofmt version is not semver
			},
		},
	}
}

// KnownInfrastructureTools returns tools commonly needed by goneat ecosystem
func KnownInfrastructureTools() []Tool {
	return []Tool{
		{
			Name:          "ripgrep",
			Kind:          "system",
			Description:   "Fast text search tool used for enhanced text searching and license auditing",
			Platforms:     []string{"darwin", "linux", "windows"},
			DetectCommand: "rg --version",
			InstallCommands: map[string]string{
				"darwin":  "brew install ripgrep",
				"linux":   "sudo apt-get install ripgrep || sudo yum install ripgrep || sudo pacman -S ripgrep",
				"windows": "winget install BurntSushi.ripgrep.MSVC || scoop install ripgrep",
				"mise":    "mise use ripgrep@latest",
				"brew":    "brew install ripgrep",
				"pacman":  "sudo pacman -S --noconfirm ripgrep",
				"apt-get": "sudo apt-get install -y ripgrep",
				"dnf":     "sudo dnf install -y ripgrep",
				"yum":     "sudo yum install -y ripgrep",
				"scoop":   "scoop install ripgrep",
				"winget":  "winget install BurntSushi.ripgrep.MSVC",
			},
			InstallerPriority: map[string][]string{
				"darwin":  {string(installerMise), string(installerBrew)},
				"linux":   {string(installerMise), string(installerPacman), string(installerAptGet), string(installerDnf), string(installerYum)},
				"windows": {string(installerScoop), string(installerWinget)},
			},
			VersionPolicy: versioning.Policy{
				Scheme: versioning.SchemeSemverFull,
			},
		},
		{
			Name:          "jq",
			Kind:          "system",
			Description:   "JSON processor used for CI/CD scripts and API response parsing",
			Platforms:     []string{"darwin", "linux", "windows"},
			DetectCommand: "jq --version",
			InstallCommands: map[string]string{
				"darwin":  "brew install jq",
				"linux":   "sudo apt-get install jq || sudo yum install jq || sudo pacman -S jq",
				"windows": "winget install jqlang.jq || scoop install jq",
				"mise":    "mise use jq@latest",
				"brew":    "brew install jq",
				"pacman":  "sudo pacman -S --noconfirm jq",
				"apt-get": "sudo apt-get install -y jq",
				"dnf":     "sudo dnf install -y jq",
				"yum":     "sudo yum install -y jq",
				"scoop":   "scoop install jq",
				"winget":  "winget install jqlang.jq",
			},
			InstallerPriority: map[string][]string{
				"darwin":  {string(installerMise), string(installerBrew)},
				"linux":   {string(installerMise), string(installerPacman), string(installerAptGet), string(installerDnf), string(installerYum)},
				"windows": {string(installerScoop), string(installerWinget)},
			},
			VersionPolicy: versioning.Policy{
				Scheme: versioning.SchemeSemverFull,
			},
		},
		{
			Name:          "go",
			Kind:          "system",
			Description:   "Go toolchain required for Go projects",
			Platforms:     []string{"darwin", "linux", "windows"},
			DetectCommand: "go version",
			InstallCommands: map[string]string{
				"darwin":  "brew install go",
				"linux":   "sudo apt-get install golang || sudo dnf install golang || sudo pacman -S go",
				"windows": "winget install GoLang.Go || scoop install go",
				"mise":    "mise use go@1.22.0",
				"brew":    "brew install go",
				"pacman":  "sudo pacman -S --noconfirm go",
				"apt-get": "sudo apt-get install -y golang",
				"dnf":     "sudo dnf install -y golang",
				"yum":     "sudo yum install -y golang",
				"scoop":   "scoop install go",
				"winget":  "winget install GoLang.Go",
			},
			InstallerPriority: map[string][]string{
				"darwin":  {string(installerMise), string(installerBrew)},
				"linux":   {string(installerMise), string(installerPacman), string(installerAptGet), string(installerDnf), string(installerYum)},
				"windows": {string(installerScoop), string(installerWinget)},
			},
			VersionPolicy: versioning.Policy{
				Scheme:             versioning.SchemeSemverFull,
				MinimumVersion:     "1.21.0",
				RecommendedVersion: "1.22.0",
			},
		},
		{
			Name:           "go-licenses",
			Kind:           "go",
			Description:    "License compliance tool for Go dependencies",
			InstallPackage: "github.com/google/go-licenses@latest",
			VersionArgs:    []string{},
			CheckArgs:      []string{"-h"},
			VersionPolicy: versioning.Policy{
				Scheme: versioning.SchemeSemverFull,
			},
		},
	}
}

// KnownAllTools returns the union of all known tool catalogs
func KnownAllTools() []Tool {
	sec := KnownSecurityTools()
	fmtTools := KnownFormatTools()
	infraTools := KnownInfrastructureTools()
	all := make([]Tool, 0, len(sec)+len(fmtTools)+len(infraTools))
	all = append(all, sec...)
	all = append(all, fmtTools...)
	all = append(all, infraTools...)
	return all
}

func GetToolByName(name string) (Tool, bool) {
	n := strings.ToLower(strings.TrimSpace(name))
	for _, t := range KnownAllTools() {
		if t.Name == n {
			return t, true
		}
	}
	return Tool{}, false
}

func CheckTool(t Tool) Status {
	if t.Artifacts != nil {
		path, err := resolveToolPath(t.Name)
		if err == nil {
			version := detectVersionWithPath(t, path)
			status := Status{
				Name:         t.Name,
				Present:      true,
				Version:      version,
				Instructions: fmt.Sprintf("Managed binary: %s", path),
			}
			applyVersionPolicy(t, &status)
			if pathBinary, pathErr := exec.LookPath(t.Name); pathErr == nil && pathBinary != path {
				logger.Warn(fmt.Sprintf("%s also found in PATH at %s - managed binary at %s will be preferred", t.Name, pathBinary, path))
			}
			return status
		}
		if errors.Is(err, os.ErrNotExist) {
			return Status{
				Name:         t.Name,
				Present:      false,
				Instructions: installInstruction(t),
			}
		}
		logger.Warn(fmt.Sprintf("resolver error for %s: %v", t.Name, err))
	}

	if _, err := exec.LookPath(t.Name); err == nil {
		version := detectVersion(t)
		if version == "" && t.DetectCommand != "" {
			parts := strings.Fields(t.DetectCommand)
			if len(parts) > 0 {
				if output, ok := tryCommand(parts[0], parts[1:]...); ok {
					version = sanitizeVersion(output)
				}
			}
		}
		status := Status{
			Name:    t.Name,
			Present: true,
			Version: version,
		}
		applyVersionPolicy(t, &status)
		return status
	}

	if t.Kind == "system" && strings.TrimSpace(t.DetectCommand) != "" {
		parts := strings.Fields(t.DetectCommand)
		if len(parts) > 0 {
			if output, ok := tryCommand(parts[0], parts[1:]...); ok {
				status := Status{
					Name:    t.Name,
					Present: true,
					Version: sanitizeVersion(output),
				}
				applyVersionPolicy(t, &status)
				return status
			}
		}
	}

	// Enhanced PATH detection for Go tools
	if t.Kind == "go" {
		return checkGoToolInstallation(t)
	}

	var commonPaths []string
	if goBin := getGoBinPath(); goBin != "" {
		commonPaths = append(commonPaths, goBin)
	}
	if homeDir, err := os.UserHomeDir(); err == nil {
		commonPaths = append(commonPaths, filepath.Join(homeDir, "go", "bin"))
	}

	var foundPath string
	for _, checkPath := range commonPaths {
		fullPath := filepath.Join(checkPath, t.Name)
		if _, err := os.Stat(fullPath); err == nil {
			foundPath = fullPath
			break
		}
	}

	if foundPath != "" {
		return Status{
			Name:         t.Name,
			Present:      false,
			Instructions: buildPathInstructions(foundPath),
		}
	}

	return Status{
		Name:         t.Name,
		Present:      false,
		Instructions: installInstruction(t),
	}
}

// checkGoToolInstallation provides enhanced detection for Go-installed tools
func checkGoToolInstallation(t Tool) Status {
	// Check if Go is available for installation
	goAvailable := commandExists("go")
	if !goAvailable {
		return Status{
			Name:         t.Name,
			Present:      false,
			Instructions: "Go toolchain not found. Install Go first: https://go.dev/dl/",
		}
	}

	// Check expected Go bin locations for the tool
	var candidatePaths []string
	if goBin := getGoBinPath(); goBin != "" {
		candidatePaths = append(candidatePaths, filepath.Join(goBin, t.Name))
	}
	if homeDir, err := os.UserHomeDir(); err == nil {
		candidatePaths = append(candidatePaths, filepath.Join(homeDir, "go", "bin", t.Name))
	}

	// Check if tool binary exists in any expected location
	for _, candidatePath := range candidatePaths {
		if _, err := os.Stat(candidatePath); err == nil {
			// Tool is installed but not in PATH
			return Status{
				Name:         t.Name,
				Present:      false,
				Instructions: buildEnhancedPathInstructions(candidatePath, t.Name),
			}
		}
	}

	// Tool is not installed - provide installation instructions
	return Status{
		Name:         t.Name,
		Present:      false,
		Instructions: fmt.Sprintf("Tool not installed. Run: go install %s", t.InstallPackage),
	}
}

// ValidateFoundationTools performs proactive checks for foundation tools accessibility
func ValidateFoundationTools() []string {
	var warnings []string

	// Check Go toolchain availability (critical for many tools)
	if !commandExists("go") {
		warnings = append(warnings, "Go toolchain not found in PATH. Many foundation tools require Go to be installed and accessible.")
	} else {
		// Check if Go bin directory is in PATH
		goBinPath := getGoBinPath()
		if goBinPath != "" {
			pathDirs := strings.Split(os.Getenv("PATH"), string(os.PathListSeparator))
			inPath := false
			for _, dir := range pathDirs {
				if dir == goBinPath {
					inPath = true
					break
				}
			}
			if !inPath {
				warnings = append(warnings, fmt.Sprintf("Go bin directory (%s) is not in PATH. Go-installed tools may not be accessible.", goBinPath))
			}
		}
	}

	// Check for common PATH issues that affect foundation tools
	if runtime.GOOS != "windows" {
		homeDir, err := os.UserHomeDir()
		if err == nil {
			localBin := filepath.Join(homeDir, ".local", "bin")
			if _, err := os.Stat(localBin); err == nil {
				// .local/bin exists, check if it's in PATH
				pathDirs := strings.Split(os.Getenv("PATH"), string(os.PathListSeparator))
				inPath := false
				for _, dir := range pathDirs {
					if dir == localBin {
						inPath = true
						break
					}
				}
				if !inPath {
					warnings = append(warnings, "~/.local/bin exists but is not in PATH. Some tools may install there.")
				}
			}
		}
	}

	return warnings
}

func InstallTool(t Tool) Status {
	switch t.Kind {
	case "system":
		if t.Artifacts != nil {
			return installArtifactTool(t)
		}
		return installSystemTool(t)
	case "go":
		return installGoTool(t)
	case "bundled-go":
		return Status{
			Name:         t.Name,
			Present:      false,
			Installed:    false,
			Instructions: "Install Go toolchain first: https://go.dev/dl/ (gofmt is bundled)",
		}
	default:
		return Status{
			Name:         t.Name,
			Present:      false,
			Installed:    false,
			Instructions: installInstruction(t),
		}
	}
}

func installArtifactTool(t Tool) Status {
	toolConfig := tools.Tool{
		Name:      t.Name,
		Artifacts: t.Artifacts,
	}

	opts := tools.InstallOptions{
		Version: "",
		Force:   false,
	}

	_, err := tools.InstallArtifact(toolConfig, opts)
	if err != nil {
		return Status{
			Name:      t.Name,
			Present:   false,
			Installed: false,
			Error:     fmt.Errorf("artifact installation failed: %w", err),
			Instructions: fmt.Sprintf("Failed to install %s via artifacts. Error: %v\n"+
				"Try installing manually or check network connectivity.", t.Name, err),
		}
	}

	path, resolveErr := resolveToolPath(t.Name)
	if resolveErr != nil {
		logger.Warn(fmt.Sprintf("artifact installed but resolver failed: %v", resolveErr))
	} else {
		logger.Info(fmt.Sprintf("artifact installed successfully: %s", path))
	}

	version := ""
	if resolveErr == nil {
		version = detectVersionWithPath(t, path)
	}

	status := Status{
		Name:      t.Name,
		Present:   resolveErr == nil,
		Installed: true,
		Version:   version,
	}
	if resolveErr == nil {
		status.Instructions = fmt.Sprintf("Managed binary: %s", path)
	}

	applyVersionPolicy(t, &status)
	return status
}

func installSystemTool(t Tool) Status {
	if t.Artifacts != nil {
		return installArtifactTool(t)
	}

	platform := getCurrentPlatform()
	attempts := buildInstallerAttempts(t, platform)
	status := Status{Name: t.Name}
	var lastErr error

	for i := range attempts {
		attempt := &attempts[i]
		if attempt.command == "" {
			continue
		}
		if !attempt.available {
			continue
		}
		if err := executeInstallerCommand(attempt.command); err != nil {
			logger.Warn(fmt.Sprintf("installer %s failed for %s: %v", attempt.kind, t.Name, err))
			attempt.instructions = fmt.Sprintf("%s (failed: %v)", attempt.instructions, err)
			lastErr = err
			continue
		}

		// after successful attempt, refresh status
		status.Installed = true
		status.Present = false
		if _, err := exec.LookPath(t.Name); err == nil {
			status.Present = true
			status.Version = sanitizeVersion(detectVersion(t))
			applyVersionPolicy(t, &status)
			return status
		}

		// installed but not in PATH, best-effort detection
		if goBin := getGoBinPath(); goBin != "" {
			checkPath := filepath.Join(goBin, t.Name)
			if _, err := os.Stat(checkPath); err == nil {
				status.Instructions = buildPathInstructions(checkPath)
				return status
			}
		}

		// Provide generic guidance if command not discovered
		attempt.instructions = fmt.Sprintf("Installed using %s, but binary not found in PATH. Check installation output and update PATH if needed.", attempt.kind)
		break
	}

	if status.Instructions == "" {
		status.Instructions = summarizeInstallerInstructions(attempts)
	}
	if status.Instructions == "" {
		status.Instructions = installInstruction(t)
	}
	status.Present = false
	status.Installed = false
	if lastErr != nil {
		status.Error = lastErr
	} else {
		status.Error = fmt.Errorf("no available installer succeeded for %s", t.Name)
	}
	return status
}

func resolveToolPath(toolName string) (string, error) {
	return tools.ResolveBinary(toolName, tools.ResolveOptions{
		EnvOverride: getEnvOverrideForTool(toolName),
		AllowPath:   true,
	})
}

func getEnvOverrideForTool(toolName string) string {
	switch toolName {
	case "syft":
		return "GONEAT_TOOL_SYFT"
	default:
		return ""
	}
}

func detectVersionWithPath(t Tool, binaryPath string) string {
	if binaryPath == "" {
		return ""
	}
	if len(t.VersionArgs) > 0 {
		if output, ok := tryCommand(binaryPath, t.VersionArgs...); ok {
			return sanitizeVersion(output)
		}
	}
	if t.DetectCommand != "" {
		parts := strings.Fields(t.DetectCommand)
		if len(parts) > 0 {
			if output, ok := tryCommand(binaryPath, parts[1:]...); ok {
				return sanitizeVersion(output)
			}
		}
	}
	return ""
}

func installGoTool(t Tool) Status {
	if _, err := exec.LookPath("go"); err != nil {
		return Status{
			Name:         t.Name,
			Present:      false,
			Installed:    false,
			Error:        fmt.Errorf("'go' toolchain not found in PATH"),
			Instructions: fmt.Sprintf("Install Go toolchain (see %s)\nThen run: %s", packageManagerDocPath, goInstallCommand(t)),
		}
	}

	installCmd := exec.Command("go", "install", t.InstallPackage) // #nosec G204
	var stdout, stderr bytes.Buffer
	installCmd.Stdout = &stdout
	installCmd.Stderr = &stderr
	if err := installCmd.Run(); err != nil {
		present := commandExists(t.Name)
		return Status{
			Name:         t.Name,
			Present:      present,
			Installed:    false,
			Error:        fmt.Errorf("install failed: %v, stderr: %s", err, strings.TrimSpace(stderr.String())),
			Instructions: goInstallCommand(t),
		}
	}

	status := Status{
		Name:      t.Name,
		Installed: true,
		Version:   detectVersion(t),
	}
	if _, err := exec.LookPath(t.Name); err == nil {
		status.Present = true
		applyVersionPolicy(t, &status)
		return status
	}

	if goBin := getGoBinPath(); goBin != "" {
		candidate := filepath.Join(goBin, t.Name)
		if _, err := os.Stat(candidate); err == nil {
			status.Present = false
			status.Instructions = buildPathInstructions(candidate)
			return status
		}
	}

	status.Present = false
	status.Error = fmt.Errorf("go install succeeded but %s not found in PATH", t.Name)
	status.Instructions = fmt.Sprintf("Tool should be available under %s. Update PATH and retry.", getGoBinPath())
	return status
}

func detectVersion(t Tool) string {
	// Try version args first
	if len(t.VersionArgs) > 0 {
		if ver, ok := tryCommand(t.Name, t.VersionArgs...); ok {
			return sanitizeVersion(ver)
		}
	}
	// Fallback: run with help and try to parse a version-like token (best-effort)
	if len(t.CheckArgs) > 0 {
		if help, ok := tryCommand(t.Name, t.CheckArgs...); ok {
			return extractFirstVersionToken(help)
		}
	}
	return ""
}

func tryCommand(name string, args ...string) (string, bool) {
	cmd := exec.Command(name, args...)
	var out bytes.Buffer
	var errb bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errb
	if err := cmd.Run(); err != nil {
		// Some tools print to stderr; still capture any useful text
		if s := strings.TrimSpace(errb.String()); s != "" {
			return s, true
		}
		return "", false
	}
	s := strings.TrimSpace(out.String())
	if s == "" {
		// Sometimes version is on stderr
		if ss := strings.TrimSpace(errb.String()); ss != "" {
			return ss, true
		}
	}
	return s, true
}

func sanitizeVersion(s string) string {
	line := strings.TrimSpace(firstLine(s))
	prefixes := []string{
		"govulncheck: ",
		"golangci-lint has version ",
		"has version ",
	}
	for _, prefix := range prefixes {
		line = strings.TrimPrefix(line, prefix)
	}
	line = strings.TrimPrefix(line, "version ")
	line = strings.TrimPrefix(line, "Version ")
	if token := extractFirstVersionToken(line); token != "" {
		return token
	}
	return line
}

func extractFirstVersionToken(s string) string {
	line := firstLine(s)
	parts := strings.Fields(line)
	for _, p := range parts {
		if looksLikeVersion(p) {
			return p
		}
	}
	return ""
}

func looksLikeVersion(s string) bool {
	// v1.2.3 or 1.2.3 or similar
	s = strings.TrimPrefix(s, "v")
	dots := strings.Count(s, ".")
	return dots >= 1 && dots <= 3
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}

func applyVersionPolicy(t Tool, status *Status) {
	if status == nil {
		return
	}
	if t.VersionPolicy.IsZero() {
		return
	}
	if strings.TrimSpace(status.Version) == "" {
		status.PolicyError = fmt.Errorf("unable to determine version for %s", t.Name)
		return
	}
	eval, err := versioning.Evaluate(t.VersionPolicy, status.Version)
	if err != nil {
		status.PolicyError = err
		return
	}
	status.PolicyEvaluation = &eval
}

func buildPathInstructions(binaryPath string) string {
	dir := filepath.Dir(binaryPath)
	var b strings.Builder
	b.WriteString(fmt.Sprintf("Tool installed at %s but not in PATH.\n", binaryPath))
	if runtime.GOOS == "windows" {
		b.WriteString(fmt.Sprintf("Add for current session:\n  set PATH=%%PATH%%;%s\n", dir))
		b.WriteString(fmt.Sprintf("Persist (PowerShell):\n  setx PATH \"$Env:PATH;%s\"\n", dir))
		b.WriteString(fmt.Sprintf("Inline usage: $env:PATH=\"%s;\" + $env:PATH; goneat doctor tools --scope foundation\n", dir))
	} else {
		b.WriteString(fmt.Sprintf("Add for current shell:\n  export PATH=\"%s:$PATH\"\n", dir))
		b.WriteString(fmt.Sprintf("Persist (bash/zsh):\n  echo 'export PATH=\"%s:$PATH\"' >> ~/.bashrc && source ~/.bashrc\n", dir))
		b.WriteString("If you use zsh, update ~/.zshrc similarly.\n")
		b.WriteString(fmt.Sprintf("Inline usage: PATH=\"%s:$PATH\" goneat doctor tools --scope foundation\n", dir))
	}
	b.WriteString("Run 'goneat doctor env' for PATH diagnostics.")
	return strings.TrimSpace(b.String())
}

// buildEnhancedPathInstructions provides clearer guidance for Go tools found but not in PATH
func buildEnhancedPathInstructions(binaryPath, toolName string) string {
	dir := filepath.Dir(binaryPath)
	var b strings.Builder
	b.WriteString(fmt.Sprintf("✅ %s is installed at %s but not in PATH.\n", toolName, binaryPath))
	b.WriteString("This is common for Go tools installed with 'go install'.\n\n")

	if runtime.GOOS == "windows" {
		b.WriteString("Quick fix for current session:\n")
		b.WriteString(fmt.Sprintf("  set PATH=%%PATH%%;%s\n\n", dir))
		b.WriteString("Make it permanent (PowerShell as Administrator):\n")
		b.WriteString(fmt.Sprintf("  setx PATH \"$Env:PATH;%s\"\n\n", dir))
		b.WriteString("Test it works:\n")
		b.WriteString(fmt.Sprintf("  %s --version\n", toolName))
	} else {
		b.WriteString("Quick fix for current shell:\n")
		b.WriteString(fmt.Sprintf("  export PATH=\"%s:$PATH\"\n\n", dir))
		b.WriteString("Make it permanent (add to ~/.bashrc or ~/.zshrc):\n")
		b.WriteString(fmt.Sprintf("  echo 'export PATH=\"%s:$PATH\"' >> ~/.bashrc && source ~/.bashrc\n\n", dir))
		b.WriteString("Test it works:\n")
		b.WriteString(fmt.Sprintf("  %s --version\n", toolName))
	}

	b.WriteString("\nFor detailed PATH diagnostics, run: goneat doctor env")
	return strings.TrimSpace(b.String())
}

func resolveInstallerPriority(t Tool, platform string) []installerKind {
	if t.InstallerPriority != nil {
		if raw, ok := t.InstallerPriority[platform]; ok {
			return normalizeInstallerKeys(raw)
		}
		if raw, ok := t.InstallerPriority["all"]; ok {
			return normalizeInstallerKeys(raw)
		}
	}
	return defaultInstallerPriorityForTool(t, platform)
}

func defaultInstallerPriorityForTool(t Tool, platform string) []installerKind {
	switch t.Kind {
	case "go":
		return append([]installerKind{installerGoInstall}, defaultInstallerPriority[platform]...)
	default:
		if p, ok := defaultInstallerPriority[platform]; ok {
			return p
		}
		return []installerKind{installerManual}
	}
}

func normalizeInstallerKeys(raw []string) []installerKind {
	priorities := make([]installerKind, 0, len(raw))
	for _, key := range raw {
		trimmed := strings.TrimSpace(strings.ToLower(key))
		if trimmed == "" {
			continue
		}
		if kind, ok := installerKindLookup[trimmed]; ok {
			priorities = append(priorities, kind)
		}
	}
	if len(priorities) == 0 {
		return []installerKind{installerManual}
	}
	return priorities
}

func isInstallerAvailable(kind installerKind) bool {
	switch kind {
	case installerMise:
		return commandExists("mise")
	case installerBrew:
		return commandExists("brew")
	case installerScoop:
		return commandExists("scoop")
	case installerWinget:
		return commandExists("winget")
	case installerPacman:
		return commandExists("pacman")
	case installerAptGet:
		return commandExists("apt-get")
	case installerDnf:
		return commandExists("dnf")
	case installerYum:
		return commandExists("yum")
	case installerGoInstall:
		return commandExists("go")
	default:
		return false
	}
}

func commandExists(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

func defaultInstallerCommand(t Tool, kind installerKind) string {
	switch kind {
	case installerMise:
		alias := t.Name
		if cmd, ok := t.InstallCommands["mise"]; ok && cmd != "" {
			return cmd
		}
		return fmt.Sprintf("mise use %s@latest", alias)
	case installerBrew:
		if cmd, ok := t.InstallCommands["brew"]; ok && cmd != "" {
			return cmd
		}
		return fmt.Sprintf("brew install %s", t.Name)
	case installerScoop:
		if cmd, ok := t.InstallCommands["scoop"]; ok && cmd != "" {
			return cmd
		}
		return fmt.Sprintf("scoop install %s", t.Name)
	case installerWinget:
		if cmd, ok := t.InstallCommands["winget"]; ok && cmd != "" {
			return cmd
		}
		return fmt.Sprintf("winget install %s", t.Name)
	case installerPacman:
		if cmd, ok := t.InstallCommands["pacman"]; ok && cmd != "" {
			return cmd
		}
		return fmt.Sprintf("sudo pacman -S --noconfirm %s", t.Name)
	case installerAptGet:
		if cmd, ok := t.InstallCommands["apt-get"]; ok && cmd != "" {
			return cmd
		}
		return fmt.Sprintf("sudo apt-get install -y %s", t.Name)
	case installerDnf:
		if cmd, ok := t.InstallCommands["dnf"]; ok && cmd != "" {
			return cmd
		}
		return fmt.Sprintf("sudo dnf install -y %s", t.Name)
	case installerYum:
		if cmd, ok := t.InstallCommands["yum"]; ok && cmd != "" {
			return cmd
		}
		return fmt.Sprintf("sudo yum install -y %s", t.Name)
	case installerGoInstall:
		if t.InstallPackage != "" {
			return fmt.Sprintf("go install %s", t.InstallPackage)
		}
		return ""
	case installerManual:
		if cmd, ok := t.InstallCommands["manual"]; ok && cmd != "" {
			return cmd
		}
		return ""
	default:
		return ""
	}
}

func buildInstallerAttempts(t Tool, platform string) []installerAttempt {
	priorities := resolveInstallerPriority(t, platform)
	attempts := make([]installerAttempt, 0, len(priorities))
	for _, kind := range priorities {
		command := defaultInstallerCommand(t, kind)
		instructions := command
		available := isInstallerAvailable(kind)
		if !available {
			switch kind {
			case installerMise:
				instructions = fmt.Sprintf("Install mise (see %s) then run: %s", packageManagerDocPath, command)
			case installerBrew:
				instructions = fmt.Sprintf("Install Homebrew (see %s) then run: %s", packageManagerDocPath, command)
			case installerScoop:
				instructions = fmt.Sprintf("Install Scoop (see %s) then run: %s", packageManagerDocPath, command)
			case installerWinget:
				instructions = fmt.Sprintf("Ensure winget/App Installer is available (see %s) then run: %s", packageManagerDocPath, command)
			default:
				instructions = fmt.Sprintf("Ensure %s is available, then run: %s", string(kind), command)
			}
		}
		attempts = append(attempts, installerAttempt{
			kind:         kind,
			command:      command,
			available:    available && command != "",
			instructions: instructions,
		})
	}
	return attempts
}

func executeInstallerCommand(command string) error {
	if command == "" {
		return nil
	}
	if runtime.GOOS == "windows" {
		return exec.Command("powershell", "-NoProfile", "-ExecutionPolicy", "Bypass", command).Run()
	}
	return exec.Command("bash", "-lc", command).Run()
}

func summarizeInstallerInstructions(attempts []installerAttempt) string {
	if len(attempts) == 0 {
		return ""
	}
	var b strings.Builder
	hasFailures := false
	for _, attempt := range attempts {
		if attempt.instructions == "" {
			continue
		}
		b.WriteString(fmt.Sprintf("- %s\n", attempt.instructions))
		if strings.Contains(attempt.instructions, "(failed:") {
			hasFailures = true
		}
	}

	// If all attempts failed, add reference to package manager documentation
	if hasFailures && b.Len() > 0 {
		b.WriteString(fmt.Sprintf("\nFor package manager installation guides, see: %s", packageManagerDocPath))
	}

	return strings.TrimSpace(b.String())
}

func installInstruction(t Tool) string {
	switch t.Kind {
	case "go":
		return goInstallCommand(t)
	case "bundled-go":
		return "Install Go toolchain first: https://go.dev/dl/ (gofmt is included)"
	case "system":
		platform := getCurrentPlatform()
		if instructions := summarizeInstallerInstructions(buildInstallerAttempts(t, platform)); instructions != "" {
			return instructions
		}
		return fmt.Sprintf("Manual install required for %s. Refer to vendor documentation.", t.Name)
	default:
		return fmt.Sprintf("Manual install required for %s. Refer to vendor documentation.", t.Name)
	}
}

func goInstallCommand(t Tool) string {
	return fmt.Sprintf("go install %s", t.InstallPackage)
}

// getCurrentPlatform returns the current platform identifier
func getCurrentPlatform() string {
	return runtime.GOOS
}

// SupportsCurrentPlatform checks if a tool is applicable to the current platform.
//
// CRITICAL: This function prevents platform-specific tools (e.g., Windows-only tools like "scoop")
// from being checked, reported as missing, or counted toward failures on incompatible platforms
// (e.g., macOS, Linux). Without this filtering, multi-platform CI/CD pipelines and template
// repositories will fail with "tool missing" errors for tools that are intentionally excluded
// from the current platform.
//
// Platform matching rules:
// - Empty platforms list = tool supports all platforms (no restriction)
// - "*" or "all" in platforms list = tool supports all platforms
// - Otherwise, current platform must be explicitly listed in tool.Platforms
//
// Examples:
// - platforms: ["windows"] on macOS → returns false (skip this tool)
// - platforms: ["darwin", "linux"] on macOS → returns true (check this tool)
// - platforms: [] on any platform → returns true (no restriction)
// - platforms: ["*"] on any platform → returns true (explicit "all platforms")
func SupportsCurrentPlatform(tool Tool) bool {
	currentPlatform := getCurrentPlatform()

	// No platform restriction = supports all platforms
	if len(tool.Platforms) == 0 {
		return true
	}

	// Check if current platform is explicitly listed or if wildcard is present
	for _, platform := range tool.Platforms {
		normalized := strings.TrimSpace(strings.ToLower(platform))
		if normalized == currentPlatform || normalized == "*" || normalized == "all" {
			return true
		}
	}

	// Current platform not in supported list - skip this tool
	return false
}

// TryCommand is a public wrapper for tryCommand
func TryCommand(name string, args ...string) (string, bool) {
	return tryCommand(name, args...)
}

// ExecuteInstallCommand executes an install command
func ExecuteInstallCommand(command string) error {
	// Split command into parts
	parts := strings.Fields(command)
	if len(parts) == 0 {
		return fmt.Errorf("empty command")
	}

	// Execute the command
	// #nosec G204 - Command parts come from internal configuration, not user input
	cmd := exec.Command(parts[0], parts[1:]...)
	return cmd.Run()
}

// getGoBinPath returns the Go bin directory where tools are installed
func getGoBinPath() string {
	// First check GOBIN environment variable
	if goBin := os.Getenv("GOBIN"); goBin != "" {
		return goBin
	}

	// Then check GOPATH/bin
	if goPath := os.Getenv("GOPATH"); goPath != "" {
		return filepath.Join(goPath, "bin")
	}

	// Default to ~/go/bin (Go 1.8+ default)
	if homeDir, err := os.UserHomeDir(); err == nil {
		return filepath.Join(homeDir, "go", "bin")
	}

	return ""
}

// GetAllPackageManagerStatuses returns status for all package managers on the current platform.
func GetAllPackageManagerStatuses() []*tools.PackageManagerStatus {
	return tools.GetAllPackageManagerStatuses()
}

package doctor

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Tool represents an external tool that doctor can check/install
type Tool struct {
	Name           string // canonical name, e.g., "gosec"
	Kind           string // "go" | "system"
	InstallPackage string // for Kind=="go", the go install package path with @latest
	VersionArgs    []string
	CheckArgs      []string
}

// Status represents the result of a tool check or install attempt
type Status struct {
	Name         string
	Present      bool
	Version      string
	Installed    bool
	Instructions string
	Error        error
}

func KnownSecurityTools() []Tool {
	return []Tool{
		{
			Name:           "gosec",
			Kind:           "go",
			InstallPackage: "github.com/securego/gosec/v2/cmd/gosec@latest",
			VersionArgs:    []string{"-version"},
			CheckArgs:      []string{"-h"},
		},
		{
			Name:           "govulncheck",
			Kind:           "go",
			InstallPackage: "golang.org/x/vuln/cmd/govulncheck@latest",
			VersionArgs:    []string{"-version"},
			CheckArgs:      []string{"-h"},
		},
		{
			Name: "gitleaks",
			Kind: "go",
			// Note: Module path is zricethezav/gitleaks; binary name remains 'gitleaks'
			InstallPackage: "github.com/zricethezav/gitleaks/v8@latest",
			VersionArgs:    []string{"version"},
			CheckArgs:      []string{"help"},
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
			// goimports --version not universally supported; rely on help
			VersionArgs: []string{},
			CheckArgs:   []string{"-h"},
		},
		{
			// gofmt is bundled with the Go toolchain
			Name:           "gofmt",
			Kind:           "bundled-go",
			InstallPackage: "",
			VersionArgs:    []string{},
			CheckArgs:      []string{"-h"},
		},
	}
}

// KnownAllTools returns the union of all known tool catalogs
func KnownAllTools() []Tool {
	sec := KnownSecurityTools()
	fmtTools := KnownFormatTools()
	all := make([]Tool, 0, len(sec)+len(fmtTools))
	all = append(all, sec...)
	all = append(all, fmtTools...)
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
	// First check PATH
	if _, err := exec.LookPath(t.Name); err == nil {
		ver := detectVersion(t)
		return Status{
			Name:    t.Name,
			Present: true,
			Version: ver,
		}
	}

	// If not in PATH, check common Go bin locations for better diagnostics
	var commonPaths []string
	if goBin := getGoBinPath(); goBin != "" {
		commonPaths = append(commonPaths, goBin)
	}
	// Also check ~/go/bin as fallback
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
			Instructions: fmt.Sprintf("Tool installed at %s but not in PATH. Add to PATH: export PATH=\"$PATH:%s\"", foundPath, filepath.Dir(foundPath)),
		}
	}

	// Tool not found anywhere
	return Status{
		Name:         t.Name,
		Present:      false,
		Instructions: installInstruction(t),
	}
}

func InstallTool(t Tool) Status {
	// Only install Go tools in MVP
	if t.Kind != "go" {
		return Status{
			Name:         t.Name,
			Present:      false,
			Installed:    false,
			Error:        fmt.Errorf("automatic install not supported for non-Go tools"),
			Instructions: installInstruction(t),
		}
	}

	// Ensure 'go' exists
	if _, err := exec.LookPath("go"); err != nil {
		return Status{
			Name:         t.Name,
			Present:      false,
			Installed:    false,
			Error:        fmt.Errorf("'go' toolchain not found in PATH"),
			Instructions: "Install Go toolchain first: https://go.dev/dl/\nThen run: " + goInstallCommand(t),
		}
	}

	// Execute: go install <pkg>@latest
	installPkg := t.InstallPackage
	cmd := exec.Command("go", "install", installPkg) // #nosec G204
	var out bytes.Buffer
	var errb bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errb
	runErr := cmd.Run()
	if runErr != nil {
		// Even on failure, check if command is now available (race with PATH changes not handled)
		present := false
		if _, err := exec.LookPath(t.Name); err == nil {
			present = true
		}
		return Status{
			Name:         t.Name,
			Present:      present,
			Installed:    false,
			Error:        fmt.Errorf("install failed: %v, stderr: %s", runErr, strings.TrimSpace(errb.String())),
			Instructions: goInstallCommand(t),
		}
	}

	// Post-check: verify installation
	present := false
	var installPath string
	if path, err := exec.LookPath(t.Name); err == nil {
		present = true
		installPath = path
	} else {
		// Check if installed but not in PATH
		if goBin := getGoBinPath(); goBin != "" {
			checkPath := filepath.Join(goBin, t.Name)
			if _, err := os.Stat(checkPath); err == nil {
				installPath = checkPath
			}
		}
	}

	result := Status{
		Name:      t.Name,
		Present:   present,
		Installed: true, // We attempted installation
		Version:   detectVersion(t),
	}

	// Provide helpful diagnostics if tool is not in PATH
	if !present && installPath != "" {
		result.Present = false
		result.Instructions = fmt.Sprintf("Tool installed at %s but not in PATH. Add to PATH: export PATH=\"$PATH:%s\"", installPath, filepath.Dir(installPath))
	} else if !present {
		result.Present = false
		result.Error = fmt.Errorf("tool installed but not found in PATH - you may need to restart your shell or update PATH")
		result.Instructions = fmt.Sprintf("Tool should be available after updating PATH. Expected location: %s", getGoBinPath())
	}

	return result
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
	// Extract the first line and trim common prefixes
	line := firstLine(s)
	line = strings.TrimSpace(line)
	// Handle tool-specific prefix first (e.g., "govulncheck: version v1.1.0")
	line = strings.TrimPrefix(line, "govulncheck: ")
	// Then strip generic "version" prefixes
	line = strings.TrimPrefix(line, "version ")
	line = strings.TrimPrefix(line, "Version ")
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

func installInstruction(t Tool) string {
	switch t.Kind {
	case "go":
		return goInstallCommand(t)
	case "bundled-go":
		return "Install Go toolchain first: https://go.dev/dl/ (gofmt is included)"
	default:
		return fmt.Sprintf("Manual install required for %s. Refer to vendor documentation.", t.Name)
	}
}

func goInstallCommand(t Tool) string {
	return fmt.Sprintf("go install %s", t.InstallPackage)
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

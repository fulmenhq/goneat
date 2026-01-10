/*
Copyright © 2025 3 Leaps <info@3leaps.net>
*/
package dependencies

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/fulmenhq/goneat/pkg/logger"
)

// CargoDenyMinVersion is the minimum supported cargo-deny version
const CargoDenyMinVersion = "0.14.0"

// CargoDenyCheckType represents the type of cargo-deny check
type CargoDenyCheckType string

const (
	// CargoDenyCheckLicenses runs license compliance checks
	CargoDenyCheckLicenses CargoDenyCheckType = "licenses"
	// CargoDenyCheckBans runs banned crate checks
	CargoDenyCheckBans CargoDenyCheckType = "bans"
	// CargoDenyCheckAdvisories runs security advisory checks
	CargoDenyCheckAdvisories CargoDenyCheckType = "advisories"
	// CargoDenyCheckSources runs source verification checks
	CargoDenyCheckSources CargoDenyCheckType = "sources"
)

// CargoDenyFinding represents a finding from cargo-deny
type CargoDenyFinding struct {
	Type     string // "license", "licenses", "ban", "bans", "advisory", "advisories", "sources"
	Severity string // "error", "warning", "note", "help"
	Message  string
	ID       string // Advisory ID (e.g., RUSTSEC-2024-0001) or license/ban identifier
	URL      string // URL to advisory or documentation
	Code     string // cargo-deny diagnostic code
}

// CargoDenyResult contains the results of running cargo-deny
type CargoDenyResult struct {
	Findings   []CargoDenyFinding
	RootPath   string        // Directory where cargo-deny was executed
	ReportFile string        // File to attribute findings to (typically Cargo.toml)
	Duration   time.Duration // How long the check took
}

// cargoDenyEntry represents the JSON structure returned by cargo-deny (NDJSON format)
// cargo-deny outputs diagnostic entries with type: "diagnostic" and a summary with type: "summary"
type cargoDenyEntry struct {
	Type   string           `json:"type"`   // "diagnostic" or "summary"
	Fields *cargoDenyFields `json:"fields"` // Present for diagnostic entries
}

// cargoDenyFields contains the actual diagnostic information
type cargoDenyFields struct {
	Code     string             `json:"code"`     // e.g., "license-not-encountered", "duplicate", "banned"
	Severity string             `json:"severity"` // "error", "warning", "note", "help"
	Message  string             `json:"message"`
	Labels   []cargoDenyLabel   `json:"labels,omitempty"`
	Advisory *cargoDenyAdvisory `json:"advisory,omitempty"`
}

// cargoDenyLabel contains span information for diagnostics
type cargoDenyLabel struct {
	Message string `json:"message"`
	Span    string `json:"span"`
	Line    int    `json:"line"`
	Column  int    `json:"column"`
}

// cargoDenyAdvisory contains security advisory information
type cargoDenyAdvisory struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Severity string `json:"severity"`
	URL      string `json:"url"`
}

// RustProject represents a detected Rust project with workspace information
type RustProject struct {
	// CargoTomlPath is the path to the Cargo.toml file
	CargoTomlPath string
	// RootPath is the directory containing the Cargo.toml
	RootPath string
	// IsWorkspace indicates if this is a workspace root
	IsWorkspace bool
	// IsWorkspaceMember indicates if this is a workspace member (not root)
	IsWorkspaceMember bool
	// WorkspaceRootPath is the path to the workspace root (if member)
	WorkspaceRootPath string
}

// EffectiveRoot returns the path where Rust tools should be executed.
// For workspace members, this returns the workspace root.
// For standalone crates or workspace roots, this returns the project root.
func (p *RustProject) EffectiveRoot() string {
	if p.IsWorkspaceMember && p.WorkspaceRootPath != "" {
		return p.WorkspaceRootPath
	}
	return p.RootPath
}

// RunCargoDeny executes cargo-deny with the specified checks and returns findings.
// checkTypes specifies which checks to run (e.g., licenses, bans, advisories, sources).
// If checkTypes is empty, defaults to licenses and bans for dependency analysis.
func RunCargoDeny(ctx context.Context, target string, checkTypes []CargoDenyCheckType, timeout time.Duration) (*CargoDenyResult, error) {
	if !IsCargoAvailable() {
		return nil, fmt.Errorf("cargo is not available")
	}

	project := DetectRustProject(target)
	if project == nil || project.CargoTomlPath == "" {
		return nil, nil // Not a Rust project
	}

	presence := CheckCargoDenyPresence()
	if !presence.Present {
		return nil, fmt.Errorf("cargo-deny is not installed. Install with: cargo install cargo-deny")
	}
	if !presence.MeetsMin && presence.Version != "" {
		logger.Warn(fmt.Sprintf("cargo-deny %s below minimum %s; results may be unreliable", presence.Version, CargoDenyMinVersion))
	}

	root := project.EffectiveRoot()
	if root == "" {
		root = target
	}

	// Build check arguments
	if len(checkTypes) == 0 {
		checkTypes = []CargoDenyCheckType{CargoDenyCheckLicenses, CargoDenyCheckBans}
	}

	// Note: --format must come before 'check' subcommand
	args := []string{"deny", "--format", "json", "check"}
	for _, ct := range checkTypes {
		args = append(args, string(ct))
	}

	start := time.Now()
	out, err := runCargoDenyCommand(ctx, root, args, timeout)
	duration := time.Since(start)

	if err != nil {
		// cargo-deny returns non-zero on findings, but we still get JSON output
		// Only treat as error if we got no output at all
		if len(bytes.TrimSpace(out)) == 0 {
			return nil, fmt.Errorf("cargo deny failed: %w", err)
		}
	}

	if len(bytes.TrimSpace(out)) == 0 {
		return &CargoDenyResult{
			Findings:   []CargoDenyFinding{},
			RootPath:   root,
			ReportFile: rustIssueFile(project),
			Duration:   duration,
		}, nil
	}

	entries, perr := parseCargoDenyEntries(out)
	if perr != nil {
		return nil, perr
	}

	findings := make([]CargoDenyFinding, 0, len(entries))
	for _, entry := range entries {
		// Skip summary entries, only process diagnostics
		if entry.Type != "diagnostic" || entry.Fields == nil {
			continue
		}

		fields := entry.Fields
		finding := CargoDenyFinding{
			Type:     mapCodeToType(fields.Code),
			Severity: fields.Severity,
			Message:  strings.TrimSpace(fields.Message),
			Code:     fields.Code,
		}

		// Extract label info if present
		if len(fields.Labels) > 0 {
			// Use first label's span as ID if applicable
			if finding.Message == "" {
				finding.Message = fields.Labels[0].Message
			}
		}

		// Extract advisory info if present
		if fields.Advisory != nil {
			finding.ID = fields.Advisory.ID
			if finding.Message == "" {
				finding.Message = fields.Advisory.Title
			}
			if finding.Severity == "" {
				finding.Severity = fields.Advisory.Severity
			}
			finding.URL = fields.Advisory.URL
		}

		if finding.Message == "" {
			finding.Message = "cargo-deny finding"
		}

		findings = append(findings, finding)
	}

	logger.Debug(fmt.Sprintf("cargo-deny found %d findings", len(findings)))

	return &CargoDenyResult{
		Findings:   findings,
		RootPath:   root,
		ReportFile: rustIssueFile(project),
		Duration:   duration,
	}, nil
}

// runCargoDenyCommand executes cargo deny with the given arguments
// NOTE: cargo-deny (as of v0.14+) outputs JSON diagnostics to STDERR, not stdout.
// This is unusual but intentional - stdout is reserved for machine-readable output
// while stderr gets the human-readable format. With --format json, the JSON goes to stderr.
func runCargoDenyCommand(ctx context.Context, dir string, args []string, timeout time.Duration) ([]byte, error) {
	if timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	cmd := exec.CommandContext(ctx, "cargo", args...) // #nosec G204 -- args from controlled input
	cmd.Dir = dir

	// cargo-deny outputs JSON to STDERR (not stdout!) when using --format json
	// This is intentional per cargo-deny design - stdout is for piped output, stderr for diagnostics
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	// Use stderr output as the JSON source (cargo-deny's actual output location)
	output := stderr.Bytes()

	if err != nil {
		// Non-zero exit is expected when issues are found
		if _, ok := err.(*exec.ExitError); ok { //nolint:errorlint // checking for specific type
			// cargo-deny returns non-zero on findings but still outputs valid JSON
			return output, nil
		}
		return nil, fmt.Errorf("cargo execution failed: %w", err)
	}
	return output, nil
}

// parseCargoDenyEntries parses cargo-deny NDJSON output
func parseCargoDenyEntries(out []byte) ([]cargoDenyEntry, error) {
	var entries []cargoDenyEntry

	// cargo-deny outputs newline-delimited JSON (NDJSON)
	scanner := bufio.NewScanner(bytes.NewReader(out))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var entry cargoDenyEntry
		if err := json.Unmarshal([]byte(line), &entry); err == nil && entry.Type != "" {
			entries = append(entries, entry)
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("failed to parse cargo-deny output: %w", err)
	}

	// Return entries even if empty (no findings = success)
	return entries, nil
}

// rustIssueFile returns the file to attribute Rust issues to
func rustIssueFile(project *RustProject) string {
	if project == nil {
		return "Cargo.toml"
	}
	if project.CargoTomlPath != "" {
		return filepath.ToSlash(project.CargoTomlPath)
	}
	return filepath.ToSlash(filepath.Join(project.RootPath, "Cargo.toml"))
}

// CargoDenyPresence represents the presence and version of cargo-deny
type CargoDenyPresence struct {
	Present    bool
	Version    string
	MeetsMin   bool
	MinVersion string
}

// CheckCargoDenyPresence checks if cargo-deny is available and meets minimum version
func CheckCargoDenyPresence() CargoDenyPresence {
	result := CargoDenyPresence{
		MinVersion: CargoDenyMinVersion,
	}

	cmd := exec.Command("cargo", "deny", "--version") // #nosec G204 -- hardcoded args
	output, err := cmd.Output()
	if err != nil {
		return result
	}

	result.Present = true
	result.Version = parseVersionFromOutput(string(output))

	if result.Version != "" {
		result.MeetsMin = compareVersions(result.Version, CargoDenyMinVersion) >= 0
	} else {
		result.MeetsMin = true // Assume OK if we can't parse
	}

	return result
}

// IsCargoAvailable checks if the cargo command is available
func IsCargoAvailable() bool {
	_, err := exec.LookPath("cargo")
	return err == nil
}

// DetectRustProject detects a Rust project at the given target path.
// Returns nil if no Rust project is detected.
func DetectRustProject(target string) *RustProject {
	// Primary detection: check for Cargo.toml
	cargoPath := filepath.Join(target, "Cargo.toml")
	if _, err := os.Stat(cargoPath); err == nil {
		return analyzeCargoToml(cargoPath, target)
	}

	// Check parent directories for Cargo.toml (we might be in a subdirectory)
	if project := findCargoInParents(target); project != nil {
		return project
	}

	// Secondary detection: check for .rs files (non-Cargo Rust, rare)
	if hasRustFiles(target) {
		logger.Debug(fmt.Sprintf("Rust files found without Cargo.toml at %s", target))
		return &RustProject{
			RootPath: target,
		}
	}

	return nil
}

// WorkspacePattern matches [workspace] section in Cargo.toml
var workspacePattern = regexp.MustCompile(`(?m)^\s*\[workspace\]`)

// WorkspaceMemberPattern matches workspace = "..." in [package] section
var workspaceMemberPattern = regexp.MustCompile(`(?m)^\s*workspace\s*=`)

// analyzeCargoToml analyzes a Cargo.toml to determine workspace status
func analyzeCargoToml(cargoPath, rootPath string) *RustProject {
	content, err := os.ReadFile(cargoPath) // #nosec G304 -- cargoPath derived from target directory
	if err != nil {
		logger.Debug(fmt.Sprintf("Failed to read Cargo.toml at %s: %v", cargoPath, err))
		return &RustProject{
			CargoTomlPath: cargoPath,
			RootPath:      rootPath,
		}
	}

	project := &RustProject{
		CargoTomlPath: cargoPath,
		RootPath:      rootPath,
	}

	// Check if this is a workspace root
	if workspacePattern.Match(content) {
		project.IsWorkspace = true
		return project
	}

	// Check if this is a workspace member
	if workspaceMemberPattern.Match(content) {
		project.IsWorkspaceMember = true
		// Find the workspace root
		if wsRoot := findWorkspaceRoot(rootPath); wsRoot != "" {
			project.WorkspaceRootPath = wsRoot
		}
	}

	return project
}

// findCargoInParents walks up the directory tree looking for Cargo.toml
func findCargoInParents(startPath string) *RustProject {
	absPath, err := filepath.Abs(startPath)
	if err != nil {
		return nil
	}

	var firstProject *RustProject

	current := absPath
	for i := 0; i < 10; i++ {
		parent := filepath.Dir(current)
		if parent == current {
			break
		}

		cargoPath := filepath.Join(parent, "Cargo.toml")
		if _, err := os.Stat(cargoPath); err == nil {
			project := analyzeCargoToml(cargoPath, parent)

			if project.IsWorkspace {
				return project
			}

			if !project.IsWorkspaceMember {
				return project
			}

			if firstProject == nil {
				firstProject = project
			}
		}

		current = parent
	}

	return firstProject
}

// findWorkspaceRoot finds the workspace root for a workspace member
func findWorkspaceRoot(memberPath string) string {
	absPath, err := filepath.Abs(memberPath)
	if err != nil {
		return ""
	}

	current := filepath.Dir(absPath)
	for i := 0; i < 10; i++ {
		parent := filepath.Dir(current)
		if parent == current {
			break
		}

		cargoPath := filepath.Join(current, "Cargo.toml")
		// #nosec G304 -- cargoPath is filepath.Join(current, "Cargo.toml")
		if content, err := os.ReadFile(cargoPath); err == nil {
			if workspacePattern.Match(content) {
				return current
			}
		}

		current = parent
	}

	return ""
}

// hasRustFiles checks if the target directory contains any .rs files
func hasRustFiles(target string) bool {
	found := false
	_ = filepath.WalkDir(target, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			name := d.Name()
			if name == "target" || name == ".git" || name == "node_modules" || name == "vendor" {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(path, ".rs") {
			found = true
			return filepath.SkipAll
		}
		return nil
	})
	return found
}

// parseVersionFromOutput extracts a version number from tool output
func parseVersionFromOutput(output string) string {
	versionPattern := regexp.MustCompile(`(\d+\.\d+\.\d+)`)
	matches := versionPattern.FindStringSubmatch(output)
	if len(matches) >= 2 {
		return matches[1]
	}
	return ""
}

// compareVersions compares two semver versions.
// Returns: -1 if a < b, 0 if a == b, 1 if a > b
func compareVersions(a, b string) int {
	partsA := strings.Split(a, ".")
	partsB := strings.Split(b, ".")

	for len(partsA) < 3 {
		partsA = append(partsA, "0")
	}
	for len(partsB) < 3 {
		partsB = append(partsB, "0")
	}

	for i := 0; i < 3; i++ {
		numA := parseVersionPart(partsA[i])
		numB := parseVersionPart(partsB[i])
		if numA < numB {
			return -1
		}
		if numA > numB {
			return 1
		}
	}
	return 0
}

// parseVersionPart parses a version part to an integer
func parseVersionPart(part string) int {
	if idx := strings.IndexAny(part, "-+"); idx != -1 {
		part = part[:idx]
	}
	var num int
	for _, c := range part {
		if c >= '0' && c <= '9' {
			num = num*10 + int(c-'0')
		} else {
			break
		}
	}
	return num
}

// mapCodeToType maps cargo-deny diagnostic codes to finding types
func mapCodeToType(code string) string {
	code = strings.ToLower(code)
	switch {
	case strings.HasPrefix(code, "license"):
		return "license"
	case strings.HasPrefix(code, "ban"), code == "duplicate":
		return "bans"
	case strings.HasPrefix(code, "advisory"), strings.Contains(code, "vulnerability"):
		return "advisory"
	case strings.HasPrefix(code, "source"):
		return "sources"
	default:
		return code
	}
}

// isInformationalCode returns true if the cargo-deny code is informational
// (not an actual policy violation). These should be low severity.
func isInformationalCode(code string) bool {
	code = strings.ToLower(code)
	// "license-not-encountered" means an allowed license wasn't used - not a violation
	// "duplicate" is a warning about multiple versions, not necessarily a ban violation
	return code == "license-not-encountered"
}

// FindingSeverityLevel maps cargo-deny severity to a numeric level
// Higher values indicate more severe issues
func (f *CargoDenyFinding) SeverityLevel() int {
	switch strings.ToLower(f.Severity) {
	case "critical":
		return 4
	case "high", "error":
		return 3
	case "medium", "moderate", "warning":
		return 2
	case "low":
		return 1
	default:
		return 2
	}
}

// IsLicenseFinding returns true if this is a license-related finding
func (f *CargoDenyFinding) IsLicenseFinding() bool {
	t := strings.ToLower(f.Type)
	return t == "license" || t == "licenses"
}

// IsBanFinding returns true if this is a banned crate finding
func (f *CargoDenyFinding) IsBanFinding() bool {
	t := strings.ToLower(f.Type)
	return t == "ban" || t == "bans"
}

// IsAdvisoryFinding returns true if this is a security advisory finding
func (f *CargoDenyFinding) IsAdvisoryFinding() bool {
	t := strings.ToLower(f.Type)
	return t == "advisory" || t == "advisories"
}

// FormatMessage returns a formatted message including the ID if present
func (f *CargoDenyFinding) FormatMessage() string {
	msg := f.Message
	if f.Type != "" {
		msg = fmt.Sprintf("%s: %s", f.Type, msg)
	}
	if f.ID != "" {
		return fmt.Sprintf("cargo-deny(%s): %s", f.ID, msg)
	}
	return fmt.Sprintf("cargo-deny: %s", msg)
}

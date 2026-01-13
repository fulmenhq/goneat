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
	ID       string           // Advisory ID (e.g., RUSTSEC-2024-0001) or license/ban identifier
	URL      string           // URL to advisory or documentation
	Code     string           // cargo-deny diagnostic code
	Labels   []CargoDenyLabel // Span information with file:line references and context
}

// CargoDenyLabel contains span information for diagnostics.
// This provides rich context like specific license names, deny.toml line refs,
// and crate version details that cargo-deny includes in its JSON output.
type CargoDenyLabel struct {
	Message string // Label message (e.g., "unmatched license allowance", "windows-sys v0.52.0")
	Span    string // File span (e.g., "deny.toml:53:6")
	Line    int    // Line number in the file
	Column  int    // Column number in the file
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

		// Extract label info if present - labels contain rich context like:
		// - Specific license names (e.g., "0BSD" with "unmatched license allowance")
		// - deny.toml file:line references for configuration issues
		// - Crate version details for duplicate/ban findings
		if len(fields.Labels) > 0 {
			finding.Labels = make([]CargoDenyLabel, len(fields.Labels))
			for i, label := range fields.Labels {
				// Direct type conversion since CargoDenyLabel and cargoDenyLabel have identical fields
				finding.Labels[i] = CargoDenyLabel(label)
			}
			// Use first label's message as fallback if main message is empty
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

// IsInformationalCode returns true if the cargo-deny code is informational
// (not an actual policy violation). These should be low severity.
// Exported for use by internal/assess/rust_cargo_deny.go
func IsInformationalCode(code string) bool {
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
	case "low", "note", "help":
		// "note" and "help" are informational cargo-deny severities
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

// FormatMessage returns a formatted message including the ID and label context.
// This provides rich context from cargo-deny's diagnostic output:
// - For license issues: specific license names and whether it's an unmatched allowance
// - For bans/duplicates: crate version details
// - File:line references for deny.toml configuration issues
func (f *CargoDenyFinding) FormatMessage() string {
	msg := f.Message
	if f.Type != "" {
		msg = fmt.Sprintf("%s: %s", f.Type, msg)
	}

	// Enrich message with label context
	labelContext := f.formatLabelContext()
	if labelContext != "" {
		msg = fmt.Sprintf("%s [%s]", msg, labelContext)
	}

	if f.ID != "" {
		return fmt.Sprintf("cargo-deny(%s): %s", f.ID, msg)
	}
	return fmt.Sprintf("cargo-deny: %s", msg)
}

// formatLabelContext extracts meaningful context from labels.
// Returns a string with relevant details like:
// - Specific license/crate names from label messages
// - File:line references for configuration issues
// - Version information for duplicate crate findings
func (f *CargoDenyFinding) formatLabelContext() string {
	if len(f.Labels) == 0 {
		return ""
	}

	var parts []string

	for _, label := range f.Labels {
		// Extract meaningful context from label
		if label.Message != "" {
			// For duplicate findings, label.Message often contains version info like "windows-sys v0.52.0"
			// For license findings, it contains context like "unmatched license allowance"
			parts = append(parts, label.Message)
		}

		// Include file:line reference for configuration issues (typically deny.toml refs)
		if label.Span != "" && strings.Contains(label.Span, "deny.toml") {
			// Span format is usually "deny.toml:53:6" - include it for actionable context
			parts = append(parts, fmt.Sprintf("at %s", label.Span))
		}
	}

	if len(parts) == 0 {
		return ""
	}

	// Deduplicate and join - cargo-deny can have redundant labels
	seen := make(map[string]bool)
	var unique []string
	for _, p := range parts {
		if !seen[p] {
			seen[p] = true
			unique = append(unique, p)
		}
	}

	return strings.Join(unique, "; ")
}

// CargoDenyListResult contains the results of running cargo deny list
type CargoDenyListResult struct {
	Dependencies []CargoCrateLicense
	RootPath     string
	Duration     time.Duration
}

// CargoCrateLicense represents a crate with its license information from cargo deny list
type CargoCrateLicense struct {
	Name     string   // Crate name
	Version  string   // Crate version
	Licenses []string // List of licenses (e.g., ["MIT", "Apache-2.0"])
}

// RunCargoDenyList executes cargo deny list to get dependency license information.
// This provides the equivalent of `goneat dependencies --licenses` for Rust projects,
// producing a list of all dependencies with their detected licenses.
func RunCargoDenyList(ctx context.Context, target string, timeout time.Duration) (*CargoDenyListResult, error) {
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

	root := project.EffectiveRoot()
	if root == "" {
		root = target
	}

	start := time.Now()

	// Run cargo deny list - outputs a table of crates with licenses
	// Format: "crate_name version license1 OR license2"
	args := []string{"deny", "list"}
	out, err := runCargoDenyListCommand(ctx, root, args, timeout)
	duration := time.Since(start)

	if err != nil {
		return nil, fmt.Errorf("cargo deny list failed: %w", err)
	}

	deps := parseCargoDenyList(out)

	logger.Debug(fmt.Sprintf("cargo deny list found %d dependencies", len(deps)))

	return &CargoDenyListResult{
		Dependencies: deps,
		RootPath:     root,
		Duration:     duration,
	}, nil
}

// runCargoDenyListCommand executes cargo deny list with the given arguments
func runCargoDenyListCommand(ctx context.Context, dir string, args []string, timeout time.Duration) ([]byte, error) {
	if timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	cmd := exec.CommandContext(ctx, "cargo", args...) // #nosec G204 -- args from controlled input
	cmd.Dir = dir

	// cargo deny list outputs to stdout (unlike check which uses stderr for JSON)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		// cargo deny list should not fail on normal operation
		if _, ok := err.(*exec.ExitError); ok { //nolint:errorlint // checking for specific type
			// May have warnings but still output valid data
			if stdout.Len() > 0 {
				return stdout.Bytes(), nil
			}
		}
		return nil, fmt.Errorf("cargo deny list execution failed: %w", err)
	}

	return stdout.Bytes(), nil
}

// parseCargoDenyList parses the output of cargo deny list.
// The output format is a table with columns: Name, Version, License
// Example:
//
//	Name            Version License
//	----            ------- -------
//	aho-corasick    1.1.3   Unlicense OR MIT
//	anstream        0.6.18  MIT OR Apache-2.0
func parseCargoDenyList(out []byte) []CargoCrateLicense {
	var deps []CargoCrateLicense

	scanner := bufio.NewScanner(bytes.NewReader(out))
	headerSeen := false
	separatorSeen := false

	for scanner.Scan() {
		line := scanner.Text()

		// Skip empty lines
		if strings.TrimSpace(line) == "" {
			continue
		}

		// Skip header line (contains "Name" and "Version")
		if !headerSeen && strings.Contains(line, "Name") && strings.Contains(line, "Version") {
			headerSeen = true
			continue
		}

		// Skip separator line (contains "----")
		if headerSeen && !separatorSeen && strings.Contains(line, "----") {
			separatorSeen = true
			continue
		}

		// Parse data lines after header and separator
		if headerSeen && separatorSeen {
			dep := parseCargoDenyListLine(line)
			if dep.Name != "" {
				deps = append(deps, dep)
			}
		}
	}

	return deps
}

// parseCargoDenyListLine parses a single line from cargo deny list output.
// Lines are whitespace-delimited: "crate-name    1.0.0   MIT OR Apache-2.0"
func parseCargoDenyListLine(line string) CargoCrateLicense {
	// Split on whitespace, but the license field may contain spaces (e.g., "MIT OR Apache-2.0")
	// The format is: name version license...
	// We split into at most 3 parts: name, version, rest (license expression)
	fields := strings.Fields(line)
	if len(fields) < 2 {
		return CargoCrateLicense{}
	}

	name := fields[0]
	version := fields[1]

	// Everything after name and version is the license expression
	var licenseExpr string
	if len(fields) > 2 {
		licenseExpr = strings.Join(fields[2:], " ")
	}

	// Parse license expression - can be "MIT", "MIT OR Apache-2.0", etc.
	licenses := parseLicenseExpression(licenseExpr)

	return CargoCrateLicense{
		Name:     name,
		Version:  version,
		Licenses: licenses,
	}
}

// parseLicenseExpression parses an SPDX-like license expression into individual licenses.
// Examples:
//   - "MIT" -> ["MIT"]
//   - "MIT OR Apache-2.0" -> ["MIT", "Apache-2.0"]
//   - "MIT AND Apache-2.0" -> ["MIT", "Apache-2.0"]
//   - "(MIT OR Apache-2.0) AND BSD-3-Clause" -> ["MIT", "Apache-2.0", "BSD-3-Clause"]
func parseLicenseExpression(expr string) []string {
	if expr == "" {
		return nil
	}

	// Remove parentheses
	expr = strings.ReplaceAll(expr, "(", "")
	expr = strings.ReplaceAll(expr, ")", "")

	// Split on OR and AND (case insensitive)
	// Use regex to handle " OR " and " AND " with surrounding spaces
	separatorPattern := regexp.MustCompile(`\s+(?:OR|AND)\s+`)
	parts := separatorPattern.Split(expr, -1)

	var licenses []string
	seen := make(map[string]bool)
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" && !seen[p] {
			seen[p] = true
			licenses = append(licenses, p)
		}
	}

	return licenses
}

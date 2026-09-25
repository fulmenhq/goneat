package assess

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	projectconfig "github.com/fulmenhq/goneat/pkg/config"
	"github.com/fulmenhq/goneat/pkg/logger"
)

// ErrRustfmtUnavailable reports that cargo fmt (rustfmt) could not be run
// for the selected toolchain.
var ErrRustfmtUnavailable = errors.New("rustfmt unavailable")

// RustfmtGuidance is the install hint for a missing rustfmt component.
const RustfmtGuidance = "Install with: rustup component add rustfmt"

// RustFormatRequest describes one cargo fmt run over a Cargo workspace.
type RustFormatRequest struct {
	Root      string // Cargo workspace or package root
	Toolchain string // optional rustup toolchain (+<toolchain>)
	Check     bool   // true: report files (--check -l); false: rewrite in place
	Timeout   time.Duration
}

// RustFormatResult lists the files cargo fmt reported (check) as needing formatting.
type RustFormatResult struct {
	Files []string
}

// RustFormatSettings resolves format.rust from the project .goneat.yaml.
func RustFormatSettings(cfg *projectconfig.Config) (enabled bool, toolchain string, err error) {
	if cfg == nil {
		return true, "", nil
	}
	enabled = cfg.Format.Rust.IsEnabled()
	toolchain = cfg.Format.Rust.Toolchain
	if toolchain != "" && !rustToolchainToken.MatchString(toolchain) {
		return enabled, "", fmt.Errorf("invalid format.rust.toolchain value %q", toolchain)
	}
	return enabled, toolchain, nil
}

// FindRustWorkspaceRoot returns the Cargo root for dir, or "" when dir is not
// inside a Cargo project.
func FindRustWorkspaceRoot(dir string) string {
	project := DetectRustProject(dir)
	if project == nil || project.CargoTomlPath == "" {
		return ""
	}
	return project.EffectiveRootOr(dir)
}

// RunCargoFmt runs `cargo fmt --all` for the workspace at req.Root. In check
// mode it lists the files rustfmt would change. cargo fmt always operates on
// the whole workspace; callers that were given a file subset must say so.
func RunCargoFmt(req RustFormatRequest) (RustFormatResult, error) {
	prefix := []string{}
	if req.Toolchain != "" {
		prefix = append(prefix, "+"+req.Toolchain)
	}

	versionArgs := append(append([]string{}, prefix...), "fmt", "--version")
	if run, err := runToolSplitEnv(req.Root, "cargo", versionArgs, req.Timeout, rustupNoAutoInstallEnv); err != nil || run.ExitCode != 0 {
		detail := ""
		if tail := run.stderrTail(3); tail != "" {
			detail = ": " + tail
		}
		if req.Toolchain != "" {
			return RustFormatResult{}, fmt.Errorf("%w for toolchain %q%s (%s; goneat does not install toolchains)", ErrRustfmtUnavailable, req.Toolchain, detail, RustfmtGuidance)
		}
		return RustFormatResult{}, fmt.Errorf("%w%s (%s)", ErrRustfmtUnavailable, detail, RustfmtGuidance)
	}

	args := append(append([]string{}, prefix...), "fmt", "--all")
	if req.Check {
		args = append(args, "--", "--check", "-l")
	}
	command := "cargo " + strings.Join(args, " ")
	run, err := runToolSplitEnv(req.Root, "cargo", args, req.Timeout, rustupNoAutoInstallEnv)
	if err != nil {
		return RustFormatResult{}, fmt.Errorf("%s: %w", command, err)
	}

	var files []string
	for _, line := range strings.Split(string(run.Stdout), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if !filepath.IsAbs(line) {
			line = filepath.Join(req.Root, line)
		}
		files = append(files, filepath.Clean(line))
	}

	if run.ExitCode == 0 {
		return RustFormatResult{Files: files}, nil
	}
	// In check mode rustfmt exits 1 and lists the files that differ. It also
	// exits 1 on a parse error, bad manifest or rustfmt.toml error; those
	// print an "error" line on stderr and did not complete a format check.
	// "Warning:" lines (e.g. nightly-only options on stable) are not failures.
	if req.Check && run.ExitCode == 1 && len(files) > 0 && !hasErrorLine(run.Stderr) {
		return RustFormatResult{Files: files}, nil
	}
	msg := fmt.Sprintf("%s exited %d", command, run.ExitCode)
	if tail := run.stderrTail(10); tail != "" {
		msg += ":\n" + tail
	}
	return RustFormatResult{Files: files}, errors.New(msg)
}

func hasErrorLine(stderr []byte) bool {
	for _, line := range strings.Split(string(stderr), "\n") {
		if strings.HasPrefix(strings.ToLower(strings.TrimSpace(line)), "error") {
			return true
		}
	}
	return false
}

// runRustFormatAssessment is the assess format-category Rust check. When a
// Cargo project is in scope and cargo/rustfmt is unavailable, the check
// fails rather than passing unchecked Rust; set format.rust.enabled: false in
// .goneat.yaml to opt out.
func runRustFormatAssessment(target string, config AssessmentConfig) ([]Issue, error) {
	scoped := len(config.IncludeFiles) > 0 && hasActualFiles(config.IncludeFiles)
	if scoped && len(filterByExtensions(config.IncludeFiles, []string{".rs"})) == 0 {
		return nil, nil
	}
	root := FindRustWorkspaceRoot(target)
	if root == "" {
		return nil, nil
	}

	// Project config belongs to the assessment target, not the working
	// directory. With Rust in scope, a present but invalid config must not
	// silently fall back to defaults (for example an ignored
	// format.rust.toolchain).
	cfg, err := projectconfig.LoadProjectConfigAt(target)
	if err != nil {
		return nil, fmt.Errorf("cannot apply format.rust settings: %w", err)
	}
	enabled, toolchain, err := RustFormatSettings(cfg)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", filepath.Join(target, ".goneat.yaml"), err)
	}
	if !enabled {
		logger.Debug("Rust formatting disabled by format.rust.enabled")
		return nil, nil
	}

	if config.Mode == AssessmentModeNoOp {
		logger.Info("[NO-OP] Would run cargo fmt --all -- --check")
		return nil, nil
	}

	fix := config.Mode == AssessmentModeFix
	if fix && scoped {
		logger.Warn("cargo fmt formats the whole Cargo workspace; files outside the selected set may also be rewritten")
	}

	res, err := RunCargoFmt(RustFormatRequest{Root: root, Toolchain: toolchain, Check: !fix, Timeout: config.Timeout})
	if errors.Is(err, ErrRustfmtUnavailable) {
		return nil, fmt.Errorf("%w; set format.rust.enabled: false in .goneat.yaml to skip Rust formatting", err)
	}
	if err != nil {
		return nil, err
	}
	if fix {
		return nil, nil
	}

	files := res.Files
	if scoped {
		files = filterPathsToFiles(files, target, config.IncludeFiles, ".rs")
	}
	issues := make([]Issue, 0, len(files))
	for _, f := range files {
		issues = append(issues, Issue{
			File:          f,
			Severity:      SeverityLow,
			Message:       "Rust file not formatted (cargo fmt)",
			Category:      CategoryFormat,
			SubCategory:   "rust:rustfmt",
			AutoFixable:   true,
			EstimatedTime: HumanReadableDuration(30 * time.Second),
		})
	}
	return issues, nil
}

// filterPathsToFiles keeps absolute paths that match one of files (resolved
// against baseDir) with the given extension.
func filterPathsToFiles(paths []string, baseDir string, files []string, ext string) []string {
	want := make(map[string]struct{})
	for _, f := range filterByExtensions(files, []string{ext}) {
		p := f
		if !filepath.IsAbs(p) {
			p = filepath.Join(baseDir, p)
		}
		want[canonicalPath(p)] = struct{}{}
	}
	var out []string
	for _, p := range paths {
		if _, ok := want[canonicalPath(p)]; ok {
			out = append(out, p)
		}
	}
	return out
}

// canonicalPath resolves symlinks (e.g. /var -> /private/var on macOS) so
// paths reported by cargo compare equal to user-supplied paths.
func canonicalPath(p string) string {
	if abs, err := filepath.Abs(p); err == nil {
		p = abs
	}
	if resolved, err := filepath.EvalSymlinks(p); err == nil {
		p = resolved
	}
	return filepath.Clean(p)
}

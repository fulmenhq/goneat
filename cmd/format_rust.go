package cmd

import (
	"errors"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/fulmenhq/goneat/internal/assess"
	"github.com/fulmenhq/goneat/pkg/config"
	formatpkg "github.com/fulmenhq/goneat/pkg/format"
	"github.com/fulmenhq/goneat/pkg/logger"
)

// rustFormatScope is the set of Cargo workspaces a format run covers.
type rustFormatScope struct {
	roots     []string
	toolchain string
	// subset is true when the run was given explicit/staged files: cargo fmt
	// still operates on the whole workspace, so fix mode must say so.
	subset bool
	// handled holds selected .rs files covered by a workspace in roots; they
	// are formatted by cargo fmt, not by the per-file formatter.
	handled map[string]bool
	// skipReason explains why selected .rs files are not formatted (Rust
	// disabled or not selected by --types); empty when Rust is in scope.
	skipReason string
}

// withoutRustFiles removes every selected .rs file from the per-file list:
// the per-file formatter has no Rust support. Files covered by a workspace
// in scope are formatted by the cargo fmt step; the rest are skipped with
// the reason logged.
func (s rustFormatScope) withoutRustFiles(files []string) []string {
	out := make([]string, 0, len(files))
	for _, f := range files {
		if !strings.EqualFold(filepath.Ext(f), ".rs") {
			out = append(out, f)
			continue
		}
		if s.handled[f] {
			continue
		}
		reason := s.skipReason
		if reason == "" {
			reason = "not inside a Cargo project"
		}
		logger.Info(fmt.Sprintf("Skipping %s: %s", f, reason))
	}
	return out
}

// resolveRustFormatScope decides which Cargo workspaces to run cargo fmt in.
// Discovery runs cover the Cargo project containing each discovery path;
// explicit or staged runs cover the workspaces of the selected .rs files.
func resolveRustFormatScope(cfg *config.Config, discovery bool, discoveryPaths, files, contentTypes []string) (rustFormatScope, error) {
	enabled, toolchain, err := assess.RustFormatSettings(cfg)
	if err != nil {
		return rustFormatScope{}, err
	}
	if !enabled {
		return rustFormatScope{skipReason: "Rust formatting disabled by format.rust.enabled"}, nil
	}
	if len(contentTypes) > 0 {
		selected := false
		for _, t := range contentTypes {
			if strings.EqualFold(strings.TrimSpace(t), "rust") {
				selected = true
			}
		}
		if !selected {
			return rustFormatScope{skipReason: "rust not selected by --types"}, nil
		}
	}

	seen := make(map[string]struct{})
	var roots []string
	addRoot := func(dir string) bool {
		root := assess.FindRustWorkspaceRoot(dir)
		if root == "" {
			return false
		}
		if abs, err := filepath.Abs(root); err == nil {
			root = abs
		}
		if _, ok := seen[root]; !ok {
			seen[root] = struct{}{}
			roots = append(roots, root)
		}
		return true
	}

	handled := make(map[string]bool)
	if discovery {
		for _, dir := range discoveryPaths {
			addRoot(dir)
		}
	} else {
		for _, f := range files {
			if strings.EqualFold(filepath.Ext(f), ".rs") && addRoot(filepath.Dir(f)) {
				handled[f] = true
			}
		}
	}
	sort.Strings(roots)
	return rustFormatScope{roots: roots, toolchain: toolchain, subset: !discovery, handled: handled}, nil
}

// runRustFormatStep runs cargo fmt for each workspace in scope. Missing
// rustfmt fails closed unless --ignore-missing-tools, matching the other
// external formatters of goneat format. A configured toolchain that lacks
// rustfmt always fails.
func runRustFormatStep(scope rustFormatScope, checkOnly, ignoreMissingTools, quiet bool) *formatExecutionError {
	result := &formatExecutionError{}
	if !checkOnly && scope.subset {
		logger.Warn("cargo fmt formats the whole Cargo workspace; files outside the selected set may also be rewritten")
	}
	for _, root := range scope.roots {
		res, err := assess.RunCargoFmt(assess.RustFormatRequest{Root: root, Toolchain: scope.toolchain, Check: checkOnly})
		switch {
		case errors.Is(err, assess.ErrRustfmtUnavailable):
			// --ignore-missing-tools covers an absent default rustfmt only; a
			// toolchain named in config must exist.
			if ignoreMissingTools && scope.toolchain == "" {
				logger.Warn(fmt.Sprintf("rustfmt unavailable for %s; skipping Rust formatting due to --ignore-missing-tools", root),
					logger.String("result_class", string(formatpkg.ResultToolUnavailable)),
					logger.String("tool", "rustfmt"))
				continue
			}
			logger.Error(fmt.Sprintf("rustfmt unavailable for %s", root), logger.Err(err),
				logger.String("result_class", string(formatpkg.ResultToolUnavailable)))
			result.toolUnavailable++
			result.causes = append(result.causes, formatpkg.NewResultError(formatpkg.ResultToolUnavailable, root, "rustfmt", err))
			continue
		case err != nil:
			logger.Error(fmt.Sprintf("cargo fmt failed for %s", root), logger.Err(err),
				logger.String("result_class", string(formatpkg.ResultToolExecution)))
			result.toolExecution++
			result.causes = append(result.causes, formatpkg.NewResultError(formatpkg.ResultToolExecution, root, "rustfmt", err))
			continue
		}
		if !checkOnly {
			if !quiet {
				logger.Info(fmt.Sprintf("Formatted Rust workspace %s (cargo fmt --all)", root))
			}
			continue
		}
		for _, f := range res.Files {
			drift := formatpkg.FormatDrift(f)
			logger.Error(fmt.Sprintf("Formatting differs for %s", f), logger.Err(drift),
				logger.String("result_class", string(formatpkg.ResultFormatDrift)))
			result.formatDrift++
			result.causes = append(result.causes, drift)
		}
		if len(res.Files) == 0 && !quiet {
			logger.Info(fmt.Sprintf("Rust workspace %s is properly formatted", root))
		}
	}
	return result
}

// mergeFormatErrors folds the Rust step result into the file-based result.
func mergeFormatErrors(mainErr error, rust *formatExecutionError) error {
	if rust == nil || len(rust.causes) == 0 {
		return mainErr
	}
	if mainErr == nil {
		return rust
	}
	var merged *formatExecutionError
	if errors.As(mainErr, &merged) {
		merged.formatDrift += rust.formatDrift
		merged.toolUnavailable += rust.toolUnavailable
		merged.toolExecution += rust.toolExecution
		merged.fileIO += rust.fileIO
		merged.other += rust.other
		merged.causes = append(merged.causes, rust.causes...)
		return merged
	}
	return errors.Join(mainErr, rust)
}

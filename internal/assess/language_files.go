package assess

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type detectedFiles struct {
	Python []string
	JS     []string
	TS     []string
}

func collectLanguageFiles(target string, config AssessmentConfig) (detectedFiles, error) {
	// If assess is scoped to explicit include files (staged-only mode), prefer those.
	// We only use this path if IncludeFiles contains actual files (not directories).
	if len(config.IncludeFiles) > 0 && hasActualFiles(config.IncludeFiles) {
		return detectedFiles{
			Python: filterByExtensions(config.IncludeFiles, []string{".py"}),
			JS:     filterByExtensions(config.IncludeFiles, []string{".js", ".jsx"}),
			TS:     filterByExtensions(config.IncludeFiles, []string{".ts", ".tsx"}),
		}, nil
	}

	py, err := collectFilesWithScope(target, []string{"**/*.py"}, config.ExcludeFiles, config)
	if err != nil {
		return detectedFiles{}, err
	}

	js, err := collectFilesWithScope(target, []string{"**/*.js", "**/*.jsx"}, config.ExcludeFiles, config)
	if err != nil {
		return detectedFiles{}, err
	}

	ts, err := collectFilesWithScope(target, []string{"**/*.ts", "**/*.tsx"}, config.ExcludeFiles, config)
	if err != nil {
		return detectedFiles{}, err
	}

	return detectedFiles{Python: py, JS: js, TS: ts}, nil
}

func filterByExtensions(files []string, exts []string) []string {
	if len(files) == 0 {
		return nil
	}
	allowed := make(map[string]struct{}, len(exts))
	for _, e := range exts {
		allowed[strings.ToLower(e)] = struct{}{}
	}

	out := make([]string, 0, len(files))
	seen := make(map[string]struct{}, len(files))
	for _, raw := range files {
		p := filepath.ToSlash(strings.TrimSpace(raw))
		if p == "" {
			continue
		}
		if _, ok := allowed[strings.ToLower(filepath.Ext(p))]; !ok {
			continue
		}
		if _, ok := seen[p]; ok {
			continue
		}
		seen[p] = struct{}{}
		out = append(out, p)
	}

	sort.Strings(out)
	return out
}

// hasActualFiles returns true if any of the paths are regular files (not directories).
// This is used to distinguish staged-only mode (with actual file paths) from
// the default mode where IncludeFiles might contain the target directory.
func hasActualFiles(paths []string) bool {
	for _, p := range paths {
		info, err := os.Stat(p)
		if err != nil {
			// Path doesn't exist or can't be stat'd - might be a relative file path,
			// treat as a file if it has an extension
			if filepath.Ext(p) != "" {
				return true
			}
			continue
		}
		if !info.IsDir() {
			return true
		}
	}
	return false
}

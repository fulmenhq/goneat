package sbom

import (
	"bufio"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"runtime"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/bmatcuk/doublestar/v4"
)

// SourceOptions selects a scoped source-tree inventory, not a release-artifact
// inventory. ForceInclude contains literal root-relative file/directory paths.
// NoIgnore clears exclusions, but does not disable source capture safeguards.
type SourceOptions struct {
	NoIgnore        bool
	ForceInclude    []string
	ExcludePatterns []string
}

var sourceDefaultPatterns = []string{
	".git/**", "node_modules/**", ".scratchpad/**", ".cache/**",
	"bin/**", "dist/**", "sbom/**", "vendor/**",
}

type sourceEntry struct {
	name      string
	directory bool
}

type sourcePattern struct {
	pattern       string
	anchored      bool
	directoryOnly bool
}

// parseSourcePattern intentionally implements a documented root-only subset of
// ignore syntax. It is not full Git ignore processing (notably, no negation).
func parseSourcePattern(line string) (*sourcePattern, error) {
	line = trimSourcePatternLine(line)
	if line == "" || strings.HasPrefix(line, "#") {
		return nil, nil
	}
	if strings.HasPrefix(line, "!") {
		return nil, fmt.Errorf("negation is unsupported")
	}
	if !utf8.ValidString(line) || strings.ContainsAny(line, "\x00\r\n:") || strings.HasPrefix(line, "//") {
		return nil, fmt.Errorf("invalid or host-absolute pattern")
	}
	p := &sourcePattern{directoryOnly: strings.HasSuffix(line, "/")}
	if strings.HasPrefix(line, "/") {
		p.anchored = true
		line = strings.TrimPrefix(line, "/")
	} else if strings.HasPrefix(line, "./") {
		p.anchored = true
		line = strings.TrimPrefix(line, "./")
	}
	line = strings.TrimSuffix(line, "/")
	if line == "" || !doublestar.ValidatePattern(line) {
		return nil, fmt.Errorf("malformed glob")
	}
	for _, segment := range strings.Split(line, "/") {
		if segment == "" || segment == "." || segment == ".." {
			return nil, fmt.Errorf("empty or traversing path segment")
		}
		inClass := false
		for i := 0; i < len(segment); i++ {
			switch segment[i] {
			case '\\':
				i++
				if i >= len(segment) || !strings.ContainsRune("*?[]#! \t\\", rune(segment[i])) {
					return nil, fmt.Errorf("unsupported escape")
				}
			case '[':
				inClass = true
			case ']':
				inClass = false
			case '{', '}':
				if !inClass {
					return nil, fmt.Errorf("brace alternatives are unsupported")
				}
			case '*':
				if !inClass && i+1 < len(segment) && segment[i+1] == '*' && segment != "**" {
					return nil, fmt.Errorf("** must occupy a whole path segment")
				}
			}
		}
	}
	p.pattern = line
	p.anchored = p.anchored || strings.Contains(line, "/")
	return p, nil
}

func trimSourcePatternLine(line string) string {
	line = strings.TrimLeft(line, " \t")
	for len(line) > 0 && (line[len(line)-1] == ' ' || line[len(line)-1] == '\t') {
		backslashes := 0
		for i := len(line) - 2; i >= 0 && line[i] == '\\'; i-- {
			backslashes++
		}
		if backslashes%2 != 0 {
			break
		}
		line = line[:len(line)-1]
	}
	return line
}

func loadSourcePatterns(fsys fs.FS, opts SourceOptions) ([]sourcePattern, error) {
	if opts.NoIgnore {
		return nil, nil
	}
	var patterns []sourcePattern
	add := func(line, file string, number int) error {
		p, err := parseSourcePattern(line)
		if err != nil {
			return fmt.Errorf("source SBOM ignore %s:%d pattern %q: %w", file, number, line, err)
		}
		if p != nil {
			patterns = append(patterns, *p)
		}
		return nil
	}
	for _, line := range sourceDefaultPatterns {
		if err := add(line, "defaults", 0); err != nil {
			return nil, err
		}
	}
	for i, line := range opts.ExcludePatterns {
		if err := add(line, "configured", i+1); err != nil {
			return nil, err
		}
	}
	for _, name := range []string{".gitignore", ".goneatignore"} {
		f, err := fsys.Open(name)
		if err != nil {
			if !errors.Is(err, fs.ErrNotExist) {
				return nil, fmt.Errorf("read source SBOM ignore %s: %w", name, err)
			}
			continue
		}
		scanner := bufio.NewScanner(f)
		line := 0
		for scanner.Scan() {
			line++
			if err := add(scanner.Text(), name, line); err != nil {
				_ = f.Close()
				return nil, err
			}
		}
		scanErr, closeErr := scanner.Err(), f.Close()
		if scanErr != nil {
			return nil, fmt.Errorf("read source SBOM ignore %s:%d: %w", name, line+1, scanErr)
		}
		if closeErr != nil {
			return nil, fmt.Errorf("close source SBOM ignore %s: %w", name, closeErr)
		}
	}
	return patterns, nil
}

func (p sourcePattern) excludes(entry sourceEntry) bool {
	name, directory := entry.name, entry.directory
	for name != "." {
		candidate := name
		if !p.anchored {
			candidate = path.Base(name)
		}
		if (!p.directoryOnly || directory) && doublestar.MatchUnvalidated(p.pattern, candidate) {
			return true
		}
		name, directory = path.Dir(name), true
	}
	return false
}

func sourceForcePath(value string) (string, error) {
	if runtime.GOOS == "windows" {
		value = strings.ReplaceAll(value, "\\", "/")
	}
	if value == "" || !utf8.ValidString(value) || strings.ContainsAny(value, "*?[]{}\\:\x00\r\n") || strings.HasPrefix(value, "/") {
		return "", fmt.Errorf("source SBOM force-include %q must be a literal relative path", value)
	}
	for _, segment := range strings.Split(value, "/") {
		if segment == ".." {
			return "", fmt.Errorf("source SBOM force-include %q traverses outside its subject", value)
		}
	}
	return path.Clean(value), nil
}

func literalSourceExclude(name string) (string, error) {
	if !fs.ValidPath(name) || !utf8.ValidString(name) || strings.ContainsAny(name, "\\\x00\r\n:") {
		return "", fmt.Errorf("source SBOM cannot exactly exclude path %q", name)
	}
	var escaped strings.Builder
	escaped.WriteString("./")
	for _, ch := range name {
		if strings.ContainsRune("*?[]{}", ch) {
			escaped.WriteByte('\\')
		}
		escaped.WriteRune(ch)
	}
	pattern := escaped.String()
	// Prove this encoding at the matcher boundary; real Syft controls also
	// exercise escaped names before this planner is accepted as a collector.
	matched, err := doublestar.Match(pattern, "./"+name)
	if err != nil || !matched {
		return "", fmt.Errorf("source SBOM cannot exactly encode path %q", name)
	}
	return pattern, nil
}

func compileSourceExcludes(entries []sourceEntry, patterns []sourcePattern, opts SourceOptions) ([]string, error) {
	byName := map[string]sourceEntry{".": {name: ".", directory: true}}
	for _, entry := range entries {
		if !fs.ValidPath(entry.name) || entry.name == "." {
			return nil, fmt.Errorf("invalid source SBOM manifest path %q", entry.name)
		}
		if _, exists := byName[entry.name]; exists {
			return nil, fmt.Errorf("duplicate source SBOM manifest path %q", entry.name)
		}
		byName[entry.name] = entry
	}
	var forced []sourceEntry
	for _, value := range opts.ForceInclude {
		name, err := sourceForcePath(value)
		if err != nil {
			return nil, err
		}
		entry, exists := byName[name]
		if !exists {
			return nil, fmt.Errorf("source SBOM force-include %q does not exist in the captured subject", value)
		}
		forced = append(forced, entry)
	}
	var excludes []string
	for _, entry := range entries {
		if entry.directory || opts.NoIgnore {
			continue
		}
		include := false
		for _, force := range forced {
			if entry.name == force.name || (force.directory && (force.name == "." || strings.HasPrefix(entry.name, force.name+"/"))) {
				include = true
				break
			}
		}
		if include {
			continue
		}
		for _, pattern := range patterns {
			if pattern.excludes(entry) {
				literal, err := literalSourceExclude(entry.name)
				if err != nil {
					return nil, err
				}
				excludes = append(excludes, literal)
				break
			}
		}
	}
	sort.Strings(excludes)
	return excludes, nil
}

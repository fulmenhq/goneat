package cargo

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// FindLock returns dir/Cargo.lock if that file exists.
func FindLock(dir string) string {
	if dir == "" {
		return ""
	}
	p := filepath.Join(dir, "Cargo.lock")
	if _, err := os.Stat(p); err == nil {
		return p
	}
	return ""
}

// ParseLockFile reads and parses a Cargo.lock file.
func ParseLockFile(path string) ([]Package, error) {
	data, err := os.ReadFile(path) // #nosec G304 -- caller-supplied lock path
	if err != nil {
		return nil, err
	}
	return ParseLock(data)
}

// ParseLock parses Cargo.lock [[package]] tables with a boring line scanner.
// No third-party TOML helper — Entarch confirmed PARSE in-tree so this
// package stays extractable (e.g. gofulmen / pkg/cargolock later).
func ParseLock(data []byte) ([]Package, error) {
	if data == nil {
		return nil, fmt.Errorf("parse Cargo.lock: empty input")
	}

	var out []Package
	var cur *Package
	inPackage := false
	seen := map[string]bool{}

	flush := func() {
		if !inPackage || cur == nil || cur.Name == "" {
			cur = nil
			inPackage = false
			return
		}
		key := cur.Name + "@" + cur.Version
		if !seen[key] {
			seen[key] = true
			cur.Source = ClassifySource(cur.RawSource)
			out = append(out, *cur)
		}
		cur = nil
		inPackage = false
	}

	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "[") {
			flush()
			if line == "[[package]]" {
				inPackage = true
				cur = &Package{}
			}
			continue
		}
		if !inPackage || cur == nil {
			continue
		}
		key, val, ok := parseTOMLStringField(line)
		if !ok {
			continue
		}
		switch key {
		case "name":
			cur.Name = val
		case "version":
			cur.Version = val
		case "source":
			cur.RawSource = val
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("parse Cargo.lock: %w", err)
	}
	flush()
	return out, nil
}

func parseTOMLStringField(line string) (key, val string, ok bool) {
	eq := strings.Index(line, "=")
	if eq < 0 {
		return "", "", false
	}
	key = strings.TrimSpace(line[:eq])
	raw := strings.TrimSpace(line[eq+1:])
	if raw == "" {
		return key, "", true
	}
	if raw[0] == '"' || raw[0] == '\'' {
		if unq, err := strconv.Unquote(raw); err == nil {
			return key, unq, true
		}
		return key, strings.Trim(raw, `"'`), true
	}
	return key, raw, true
}

package cargo

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/pelletier/go-toml/v2"
)

// lockFile is the Cargo.lock TOML shape we care about.
type lockFile struct {
	Package []lockPackage `toml:"package"`
}

type lockPackage struct {
	Name    string `toml:"name"`
	Version string `toml:"version"`
	Source  string `toml:"source"`
}

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

// ParseLock parses Cargo.lock bytes via pelletier/go-toml/v2 (MIT, already
// a goneat dependency). No cargo-deny CLI is involved.
func ParseLock(data []byte) ([]Package, error) {
	var lf lockFile
	if err := toml.Unmarshal(data, &lf); err != nil {
		return nil, fmt.Errorf("parse Cargo.lock: %w", err)
	}
	out := make([]Package, 0, len(lf.Package))
	seen := map[string]bool{}
	for _, p := range lf.Package {
		if p.Name == "" {
			continue
		}
		key := p.Name + "@" + p.Version
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, Package{
			Name:      p.Name,
			Version:   p.Version,
			Source:    ClassifySource(p.Source),
			RawSource: p.Source,
		})
	}
	return out, nil
}

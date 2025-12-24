package schema

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/fulmenhq/goneat/pkg/safeio"
)

// IDIndexEntry represents one schema discovered in a ref-dir scan.
// It is keyed by canonical schema $id.
type IDIndexEntry struct {
	ID         string
	Path       string
	Normalized []byte
}

// IDIndex is an offline registry of schemas keyed by canonical $id.
//
// This enables no-network CI validation where schema IDs are canonical URLs
// but the registry host is offline or not deployed yet.
type IDIndex struct {
	entries map[string]IDIndexEntry
}

func newIDIndex() *IDIndex {
	return &IDIndex{entries: make(map[string]IDIndexEntry)}
}

// Get returns the entry for the given canonical $id.
func (i *IDIndex) Get(id string) (IDIndexEntry, bool) {
	if i == nil {
		return IDIndexEntry{}, false
	}
	entry, ok := i.entries[id]
	return entry, ok
}

// Len returns the number of unique $id entries.
func (i *IDIndex) Len() int {
	if i == nil {
		return 0
	}
	return len(i.entries)
}

// BuildIDIndexFromRefDirs scans one or more ref-dir roots and builds an offline $id index.
//
// Files that cannot be parsed as schemas are ignored (to allow mixed trees).
// Duplicate $id values are allowed only when the normalized schema bytes are identical.
func BuildIDIndexFromRefDirs(refDirs []string) (*IDIndex, error) {
	idx := newIDIndex()
	if len(refDirs) == 0 {
		return idx, nil
	}

	stripSchema := true

	for _, dir := range refDirs {
		cleanDir, err := safeio.CleanUserPath(dir)
		if err != nil {
			return nil, fmt.Errorf("invalid ref-dir %s: %w", dir, err)
		}
		info, err := os.Stat(cleanDir)
		if err != nil {
			return nil, fmt.Errorf("ref-dir %s: %w", cleanDir, err)
		}
		if !info.IsDir() {
			return nil, fmt.Errorf("ref-dir %s is not a directory", cleanDir)
		}

		err = filepath.WalkDir(cleanDir, func(path string, d fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if d.IsDir() {
				return nil
			}

			ext := strings.ToLower(filepath.Ext(path))
			switch ext {
			case ".json", ".yaml", ".yml":
				// ok
			default:
				return nil
			}

			fileBytes, err := os.ReadFile(path) // #nosec G304 -- path discovered by walking a sanitized directory
			if err != nil {
				return fmt.Errorf("read ref schema %s: %w", path, err)
			}

			id, normalized, err := extractAndNormalizeSchema(fileBytes, stripSchema)
			if err != nil {
				// Ignore non-schema files in the directory tree (e.g., *.data.json).
				return nil
			}
			if id == "" {
				return nil
			}

			if existing, ok := idx.entries[id]; ok {
				if bytes.Equal(existing.Normalized, normalized) {
					return nil
				}
				return fmt.Errorf("duplicate schema $id %q: %s differs from %s", id, path, existing.Path)
			}

			idx.entries[id] = IDIndexEntry{ID: id, Path: path, Normalized: normalized}
			return nil
		})
		if err != nil {
			return nil, err
		}
	}

	return idx, nil
}

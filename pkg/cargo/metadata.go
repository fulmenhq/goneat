package cargo

import (
	"encoding/json"
	"fmt"
)

// metadataFile is the stable `cargo metadata --format-version 1` subset.
type metadataFile struct {
	Packages []metadataPackage `json:"packages"`
}

type metadataPackage struct {
	Name    string  `json:"name"`
	Version string  `json:"version"`
	Source  *string `json:"source"`
}

// ParseMetadataJSON parses `cargo metadata --format-version 1` output.
// The parser is bytes-only; invoking cargo is the caller's job.
func ParseMetadataJSON(data []byte) ([]Package, error) {
	var meta metadataFile
	if err := json.Unmarshal(data, &meta); err != nil {
		return nil, fmt.Errorf("parse cargo metadata JSON: %w", err)
	}
	out := make([]Package, 0, len(meta.Packages))
	seen := map[string]bool{}
	for _, p := range meta.Packages {
		if p.Name == "" {
			continue
		}
		raw := ""
		if p.Source != nil {
			raw = *p.Source
		}
		key := p.Name + "@" + p.Version
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, Package{
			Name:      p.Name,
			Version:   p.Version,
			Source:    ClassifySource(raw),
			RawSource: raw,
		})
	}
	return out, nil
}

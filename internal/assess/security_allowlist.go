package assess

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// loadSecurityAllowlist reads exact advisory IDs independently of the grype
// enabled/fail_on settings. Those settings cannot disable the security gate.
func loadSecurityAllowlist(root string, now time.Time) (map[string]bool, error) {
	path := filepath.Join(root, ".goneat", "dependencies.yaml")
	data, err := os.ReadFile(path) // #nosec G304 -- repository policy under the assessment root
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read vulnerability allowlist: %w", err)
	}
	var policy struct {
		Vulnerabilities struct {
			Allow []struct {
				ID    string    `yaml:"id"`
				Until yaml.Node `yaml:"until"`
			} `yaml:"allow"`
		} `yaml:"vulnerabilities"`
	}
	if err := yaml.Unmarshal(data, &policy); err != nil {
		return nil, fmt.Errorf("parse vulnerability allowlist: %w", err)
	}
	allowed := make(map[string]bool)
	for _, entry := range policy.Vulnerabilities.Allow {
		id := strings.TrimSpace(entry.ID)
		if id == "" {
			return nil, errors.New("vulnerability allowlist entry requires a non-empty id")
		}
		if _, exists := allowed[id]; exists {
			return nil, fmt.Errorf("duplicate vulnerability allowlist id %q", id)
		}
		active := true
		if entry.Until.Kind != 0 {
			node := entry.Until
			if node.Kind == yaml.AliasNode && node.Alias != nil {
				node = *node.Alias
			}
			if node.Kind != yaml.ScalarNode || (node.Tag != "!!str" && node.Tag != "!!timestamp") {
				return nil, fmt.Errorf("vulnerability allowlist %s: until must be a YYYY-MM-DD date", id)
			}
			expiry, err := time.Parse("2006-01-02", node.Value)
			if err != nil || expiry.Format("2006-01-02") != node.Value {
				return nil, fmt.Errorf("vulnerability allowlist %s: until must be a YYYY-MM-DD date", id)
			}
			active = now.Before(expiry)
		}
		allowed[id] = active
	}
	return allowed, nil
}

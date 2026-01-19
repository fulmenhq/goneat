package sbom

import (
	"encoding/json"
	"fmt"
	"os"
)

func CountCycloneDXPackagesFromFile(path string) (int, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, fmt.Errorf("read sbom: %w", err)
	}
	var cdx struct {
		Components []interface{} `json:"components"`
	}
	if err := json.Unmarshal(data, &cdx); err != nil {
		return 0, fmt.Errorf("parse cyclonedx sbom: %w", err)
	}
	return len(cdx.Components), nil
}

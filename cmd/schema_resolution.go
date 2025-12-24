package cmd

import (
	"fmt"
	"strings"
)

type schemaResolutionMode string

const (
	schemaResolutionPreferID schemaResolutionMode = "prefer-id"
	schemaResolutionIDStrict schemaResolutionMode = "id-strict"
	schemaResolutionPathOnly schemaResolutionMode = "path-only"
)

func validateSchemaResolution(mode string) error {
	switch schemaResolutionMode(strings.ToLower(strings.TrimSpace(mode))) {
	case schemaResolutionPreferID, schemaResolutionIDStrict, schemaResolutionPathOnly:
		return nil
	default:
		return fmt.Errorf("invalid --schema-resolution: %s (use prefer-id, id-strict, path-only)", mode)
	}
}

func isSchemaIDURL(id string) bool {
	id = strings.ToLower(strings.TrimSpace(id))
	return strings.HasPrefix(id, "https://") || strings.HasPrefix(id, "http://")
}

package doctor

import (
	_ "embed"
)

//go:embed tools-defaults.yaml
var defaultToolsConfigYAML []byte

// GetDefaultToolsConfig returns the embedded default tools configuration
//
// Deprecated: This function uses the legacy tools-defaults.yaml which is no longer
// used at runtime. Use LoadToolsDefaultsConfig() instead to load from
// config/tools/foundation-tools-defaults.yaml.
func GetDefaultToolsConfig() []byte {
	return defaultToolsConfigYAML
}

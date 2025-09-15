package doctor

import (
	_ "embed"
)

//go:embed tools-defaults.yaml
var defaultToolsConfigYAML []byte

// GetDefaultToolsConfig returns the embedded default tools configuration
func GetDefaultToolsConfig() []byte {
	return defaultToolsConfigYAML
}

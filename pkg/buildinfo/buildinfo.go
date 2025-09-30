package buildinfo

import "runtime/debug"

// BinaryVersion is set at build time via -ldflags. Defaults to "dev".
var BinaryVersion = "dev"

// BuildTime is set at build time via -ldflags. Defaults to "unknown".
var BuildTime = "unknown"

// GitCommit is set at build time via -ldflags. Defaults to "unknown".
var GitCommit = "unknown"

// ModuleVersion returns the module version embedded by the Go toolchain (when available).
func ModuleVersion() string {
	if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" {
		return info.Main.Version
	}
	return ""
}

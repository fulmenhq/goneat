package buildinfo

import "runtime/debug"

// BinaryVersion is set at build time via -ldflags. Defaults to "dev".
var BinaryVersion = "dev"

// ModuleVersion returns the module version embedded by the Go toolchain (when available).
func ModuleVersion() string {
    if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" {
        return info.Main.Version
    }
    return ""
}


package doctor

import (
	gobuildinfo "debug/buildinfo"
	"runtime/debug"
	"strings"
)

// detectGoBinaryVersion reads the module version embedded in a Go binary.
// It is a fallback for Go tools that do not expose a usable version command.
func detectGoBinaryVersion(binaryPath string) string {
	if strings.TrimSpace(binaryPath) == "" {
		return ""
	}

	info, err := gobuildinfo.ReadFile(binaryPath)
	if err != nil {
		return ""
	}

	return goBinaryVersion(info)
}

func goBinaryVersion(info *debug.BuildInfo) string {
	if info == nil {
		return ""
	}

	version := strings.TrimSpace(info.Main.Version)
	if version == "" || version == "(devel)" {
		return ""
	}

	return version
}

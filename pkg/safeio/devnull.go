package safeio

import (
	"io"
	"runtime"
	"strings"
)

// IsNullDevice reports whether path refers to a platform null device.
// It recognises /dev/null (Unix canonical) and NUL (Windows canonical)
// on every OS so that a user may pass either form from any platform.
func IsNullDevice(path string) bool {
	if path == "/dev/null" {
		return true
	}
	// On Windows the null device is "NUL" (case-insensitive).
	// Accept it on all platforms for portability; on Unix a file literally
	// named "NUL" is theoretically possible but vanishingly unlikely and
	// never a reasonable --output target.
	if runtime.GOOS == "windows" {
		return strings.EqualFold(path, "NUL")
	}
	return path == "NUL"
}

// nullWriteCloser wraps io.Discard with a no-op Close.
type nullWriteCloser struct{}

func (nullWriteCloser) Write(p []byte) (int, error) { return io.Discard.Write(p) }
func (nullWriteCloser) Close() error                { return nil }

// NullWriter returns an io.WriteCloser that silently discards all data.
// Use it as a drop-in replacement for an *os.File when the caller wants
// to suppress output (e.g. --output /dev/null).
func NullWriter() io.WriteCloser {
	return nullWriteCloser{}
}

// Package exitcode provides standardized exit codes for goneat
package exitcode

// Exit codes for goneat CLI
const (
	Success           = 0
	GeneralError      = 1
	ConfigError       = 2
	ValidationError   = 3
	FileSystemError   = 4
	NetworkError      = 5
	PermissionError   = 6
	TimeoutError      = 7
	UnsupportedFormat = 8
	ToolNotFound      = 9
)

// String returns a human-readable description of the exit code
func String(code int) string {
	switch code {
	case Success:
		return "Success"
	case GeneralError:
		return "General error"
	case ConfigError:
		return "Configuration error"
	case ValidationError:
		return "Validation error"
	case FileSystemError:
		return "File system error"
	case NetworkError:
		return "Network error"
	case PermissionError:
		return "Permission error"
	case TimeoutError:
		return "Timeout error"
	case UnsupportedFormat:
		return "Unsupported format"
	case ToolNotFound:
		return "Tool not found"
	default:
		return "Unknown error"
	}
}

package vulnerabilities

import "strings"

type Severity string

const (
	SeverityUnknown  Severity = "unknown"
	SeverityLow      Severity = "low"
	SeverityMedium   Severity = "medium"
	SeverityHigh     Severity = "high"
	SeverityCritical Severity = "critical"
)

func NormalizeSeverity(input string) Severity {
	s := strings.ToLower(strings.TrimSpace(input))
	switch s {
	case "critical":
		return SeverityCritical
	case "high":
		return SeverityHigh
	case "medium", "med":
		return SeverityMedium
	case "low", "negligible", "none":
		return SeverityLow
	case "unknown", "":
		return SeverityUnknown
	default:
		return SeverityUnknown
	}
}

func SeverityMeetsOrExceeds(sev Severity, threshold Severity) bool {
	order := map[Severity]int{
		SeverityUnknown:  0,
		SeverityLow:      1,
		SeverityMedium:   2,
		SeverityHigh:     3,
		SeverityCritical: 4,
	}
	return order[sev] >= order[threshold]
}

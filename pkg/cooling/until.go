package cooling

import (
	"fmt"
	"strings"
	"time"
)

// ExceptionUntilLayout is the only accepted format for a cooling exception
// `until` value. The date is exclusive: the exception stops applying at
// 00:00 UTC on that day.
const ExceptionUntilLayout = "2006-01-02"

// ParseExceptionUntil parses a cooling exception `until` value.
// An empty or malformed value is an error. Callers must treat the error as
// "the exception does not apply" so a bad date can never become a permanent
// exemption.
func ParseExceptionUntil(raw string) (time.Time, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return time.Time{}, fmt.Errorf("until is empty; expected a %s date", ExceptionUntilLayout)
	}
	until, err := time.Parse(ExceptionUntilLayout, trimmed)
	if err != nil {
		return time.Time{}, fmt.Errorf("until %q is not a valid YYYY-MM-DD date", raw)
	}
	return until, nil
}

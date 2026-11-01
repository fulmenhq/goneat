package policy

import (
	"fmt"
	"math"
	"time"

	"github.com/fulmenhq/goneat/pkg/cooling"
)

// readUntil reads an exception `until` value from parsed YAML/JSON policy data.
//
// Callers pass a value only when the key is present; an absent key means no
// expiry and is handled by the caller. A present null (until: null, ~ or a
// bare until:) is an error, never "no expiry".
// A string must be a strict YYYY-MM-DD date. A time.Time (programmatic input;
// policy files are decoded with UnmarshalPolicyYAML and arrive as strings) is
// accepted only as a UTC date with no time of day.
// Everything else, including an explicit empty string and wrong types, is an
// error. Callers must not apply an exception whose until failed to read.
func readUntil(value interface{}) (string, error) {
	switch v := value.(type) {
	case nil:
		return "", fmt.Errorf("until is null; omit the key for an exception without expiry, or use a %s date", cooling.ExceptionUntilLayout)
	case string:
		if _, err := cooling.ParseExceptionUntil(v); err != nil {
			return "", err
		}
		return v, nil
	case time.Time:
		utc := v.UTC()
		if v.Location() != time.UTC || !utc.Equal(time.Date(utc.Year(), utc.Month(), utc.Day(), 0, 0, 0, 0, time.UTC)) {
			return "", fmt.Errorf("until %s has a time of day; expected a %s date", v.Format(time.RFC3339), cooling.ExceptionUntilLayout)
		}
		return utc.Format(cooling.ExceptionUntilLayout), nil
	default:
		return "", fmt.Errorf("until must be a %s date, got %T", cooling.ExceptionUntilLayout, value)
	}
}

// readInt reads a numeric threshold from parsed YAML/JSON policy data.
// It accepts int, int64, uint64, and float64 holding a whole number.
// Anything else (strings, bools, fractional floats, out-of-range values) is
// an error.
func readInt(value interface{}) (int, error) {
	switch v := value.(type) {
	case int:
		return v, nil
	case int64:
		if v < math.MinInt || v > math.MaxInt {
			return 0, fmt.Errorf("value %d is out of range", v)
		}
		return int(v), nil
	case uint64:
		if v > math.MaxInt {
			return 0, fmt.Errorf("value %d is out of range", v)
		}
		return int(v), nil
	case float64:
		if math.IsNaN(v) || math.IsInf(v, 0) || v != math.Trunc(v) {
			return 0, fmt.Errorf("value %v is not a whole number", v)
		}
		// float64(math.MaxInt) rounds up to 2^63, so use a strict upper bound.
		if v < float64(math.MinInt) || v >= float64(math.MaxInt) {
			return 0, fmt.Errorf("value %v is out of range", v)
		}
		return int(v), nil
	default:
		return 0, fmt.Errorf("expected a whole number, got %T", value)
	}
}

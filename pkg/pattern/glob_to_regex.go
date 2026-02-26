/*
Copyright © 2026 3 Leaps <info@3leaps.net>
*/
package pattern

import (
	"regexp"
	"strings"
)

const (
	ReasonEmptyPattern         = "empty_pattern"
	ReasonNegationNotSupported = "negation_not_supported"
	ReasonGlobToRegexError     = "glob_to_regex_error"
	ReasonRegexCompileError    = "regex_compile_error"
	ReasonDuplicatePattern     = "duplicate_pattern"
)

func GlobToRegexp(glob string) (string, error) {
	if glob == "" {
		return "", ErrEmptyPattern
	}

	if strings.HasPrefix(glob, "!") {
		return "", ErrNegationNotSupported
	}

	var result strings.Builder

	for i := 0; i < len(glob); i++ {
		c := glob[i]
		switch c {
		case '*':
			if i+1 < len(glob) && glob[i+1] == '*' {
				if i+2 < len(glob) && glob[i+2] == '/' {
					result.WriteString("(.*/)?")
					i += 2
				} else if i+2 >= len(glob) {
					result.WriteString(".*")
					i++
				} else {
					result.WriteString(".*")
					i++
				}
			} else {
				result.WriteString("[^/]*")
			}
		case '?':
			result.WriteString("[^/]")
		case '.':
			result.WriteString(`\.`)
		case '+':
			result.WriteString(`\+`)
		case '(':
			result.WriteString(`\(`)
		case ')':
			result.WriteString(`\)`)
		case '[':
			result.WriteString(`\[`)
		case ']':
			result.WriteString(`\]`)
		case '{':
			result.WriteString(`\{`)
		case '}':
			result.WriteString(`\}`)
		case '^':
			result.WriteString(`\^`)
		case '$':
			result.WriteString(`\$`)
		case '|':
			result.WriteString(`\|`)
		case '\\':
			result.WriteString(`\\`)
		default:
			result.WriteByte(c)
		}
	}

	return result.String(), nil
}

var (
	ErrEmptyPattern         = errorString("empty pattern")
	ErrNegationNotSupported = errorString("negation patterns not supported")
)

type errorString string

func (e errorString) Error() string { return string(e) }

// GosecExcludeConversion captures the outcome of converting one raw ignore
// pattern into a gosec-compatible regex.
type GosecExcludeConversion struct {
	Raw        string
	Normalized string
	Regex      string
	Accepted   bool
	Reason     string
}

func normalizePattern(raw string) string {
	normalized := strings.TrimSpace(raw)
	normalized = strings.TrimSuffix(normalized, "/")
	return normalized
}

func ToGosecExcludeRegex(raw string) (regex string, ok bool, reason string) {
	normalized := normalizePattern(raw)

	if normalized == "" {
		return "", false, ReasonEmptyPattern
	}

	if strings.HasPrefix(normalized, "!") {
		return "", false, ReasonNegationNotSupported
	}

	converted, err := GlobToRegexp(normalized)
	if err != nil {
		return "", false, ReasonGlobToRegexError
	}

	if _, err := regexp.Compile(converted); err != nil {
		return "", false, ReasonRegexCompileError
	}

	return converted, true, ""
}

// ToGosecExcludeRegexDecisions converts and validates a list of raw patterns,
// returning accepted regexes and a per-pattern decision list for diagnostics.
func ToGosecExcludeRegexDecisions(rawPatterns []string) (regexes []string, decisions []GosecExcludeConversion) {
	seen := make(map[string]bool)

	for _, raw := range rawPatterns {
		normalized := normalizePattern(raw)
		decision := GosecExcludeConversion{Raw: raw, Normalized: normalized}

		regex, ok, reason := ToGosecExcludeRegex(raw)
		if !ok {
			decision.Accepted = false
			decision.Reason = reason
			decisions = append(decisions, decision)
			continue
		}

		if seen[regex] {
			decision.Accepted = false
			decision.Regex = regex
			decision.Reason = ReasonDuplicatePattern
			decisions = append(decisions, decision)
			continue
		}

		seen[regex] = true
		decision.Accepted = true
		decision.Regex = regex
		decisions = append(decisions, decision)
		regexes = append(regexes, regex)
	}

	return regexes, decisions
}

func ToGosecExcludeRegexes(rawPatterns []string) (regexes []string, skipCount int, skipReasons []string) {
	regexes, decisions := ToGosecExcludeRegexDecisions(rawPatterns)
	var reasons []string
	for _, d := range decisions {
		if d.Accepted {
			continue
		}
		skipCount++
		if d.Reason != "" {
			reasons = append(reasons, d.Reason)
		}
	}

	return regexes, skipCount, reasons
}

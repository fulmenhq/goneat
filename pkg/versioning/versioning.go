package versioning

import (
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// Scheme describes how to compare version strings.
type Scheme string

const (
	// SchemeSemverFull enforces full Semantic Versioning (SemVer 2.0.0).
	SchemeSemverFull Scheme = "semver-full"
	// SchemeSemverCompact enforces compact SemVer (numeric MAJOR.MINOR.PATCH only).
	SchemeSemverCompact Scheme = "semver-compact"
	// SchemeSemverLegacy keeps backwards compatibility with existing "semver" tokens.
	SchemeSemverLegacy Scheme = "semver"
	// SchemeCalver enforces calendar versioning (numeric YYYY.MM or YYYY.MM.DD).
	SchemeCalver Scheme = "calver"
	// SchemeLexical compares using lexical ordering only.
	SchemeLexical Scheme = "lexical"
)

type Comparison int

const (
	ComparisonUnknown Comparison = iota
	ComparisonLess
	ComparisonEqual
	ComparisonGreater
)

type Policy struct {
	Scheme             Scheme   `yaml:"version_scheme,omitempty" json:"version_scheme,omitempty"`
	MinimumVersion     string   `yaml:"minimum_version,omitempty" json:"minimum_version,omitempty"`
	RecommendedVersion string   `yaml:"recommended_version,omitempty" json:"recommended_version,omitempty"`
	DisallowedVersions []string `yaml:"disallowed_versions,omitempty" json:"disallowed_versions,omitempty"`
}

type Evaluation struct {
	Scheme             Scheme   `json:"scheme"`
	ActualVersion      string   `json:"actual_version"`
	MinimumVersion     string   `json:"minimum_version"`
	RecommendedVersion string   `json:"recommended_version"`
	DisallowedVersions []string `json:"disallowed_versions,omitempty"`

	MeetsMinimum     bool `json:"meets_minimum"`
	MeetsRecommended bool `json:"meets_recommended"`
	IsDisallowed     bool `json:"is_disallowed"`
}

var (
	semverPattern = regexp.MustCompile(`^(?:[vV])?(\d+)\.(\d+)\.(\d+)(?:-([0-9A-Za-z.-]+))?(?:\+([0-9A-Za-z.-]+))?$`)
	calverPattern = regexp.MustCompile(`^([0-9]{4})([._-])([0-9]{2})(?:([._-])([0-9]{2}))?$`)
)

// IsZero returns true when the policy contains no constraints.
func (p Policy) IsZero() bool {
	noConstraints := strings.TrimSpace(p.MinimumVersion) == "" && strings.TrimSpace(p.RecommendedVersion) == "" && len(p.DisallowedVersions) == 0
	if !noConstraints {
		return false
	}
	return p.Scheme == "" || p.Scheme == SchemeLexical
}

// Evaluate checks an actual version against the policy and reports compliance.
func Evaluate(policy Policy, actual string) (Evaluation, error) {
	normalizedScheme := schemeOrDefault(policy.Scheme)
	eval := Evaluation{
		Scheme:             normalizedScheme,
		ActualVersion:      strings.TrimSpace(actual),
		MinimumVersion:     strings.TrimSpace(policy.MinimumVersion),
		RecommendedVersion: strings.TrimSpace(policy.RecommendedVersion),
		DisallowedVersions: append([]string(nil), policy.DisallowedVersions...),
	}

	if eval.ActualVersion == "" {
		return eval, errors.New("actual version cannot be empty")
	}

	if policy.Scheme != "" && normalizedScheme == SchemeLexical && policy.Scheme != SchemeLexical {
		return eval, fmt.Errorf("unsupported version scheme: %s", policy.Scheme)
	}

	if matchString(eval.ActualVersion, eval.DisallowedVersions) {
		eval.IsDisallowed = true
	}

	if eval.MinimumVersion != "" {
		cmp, err := Compare(eval.Scheme, eval.ActualVersion, eval.MinimumVersion)
		if err != nil {
			return eval, fmt.Errorf("minimum comparison failed: %w", err)
		}
		if cmp == ComparisonGreater || cmp == ComparisonEqual {
			eval.MeetsMinimum = true
		}
	} else {
		eval.MeetsMinimum = true
	}

	if eval.RecommendedVersion != "" {
		cmp, err := Compare(eval.Scheme, eval.ActualVersion, eval.RecommendedVersion)
		if err != nil {
			return eval, fmt.Errorf("recommended comparison failed: %w", err)
		}
		if cmp == ComparisonGreater || cmp == ComparisonEqual {
			eval.MeetsRecommended = true
		}
	} else {
		eval.MeetsRecommended = true
	}

	return eval, nil
}

// Compare determines ordering between version a and b using the provided scheme.
func Compare(scheme Scheme, a, b string) (Comparison, error) {
	switch schemeOrDefault(scheme) {
	case SchemeSemverFull:
		return compareSemverFull(a, b)
	case SchemeSemverCompact:
		return compareSemverCompact(a, b)
	case SchemeCalver:
		return compareCalver(a, b)
	case SchemeLexical:
		fallthrough
	default:
		return compareLexical(a, b), nil
	}
}

func schemeOrDefault(s Scheme) Scheme {
	switch s {
	case SchemeSemverCompact:
		return SchemeSemverCompact
	case SchemeCalver:
		return SchemeCalver
	case SchemeLexical:
		return SchemeLexical
	case SchemeSemverFull, SchemeSemverLegacy:
		return SchemeSemverFull
	default:
		return SchemeLexical
	}
}

type semverIdentifier struct {
	raw     string
	numeric bool
	num     int
}

// Version represents a parsed semantic version
type Version struct {
	major      int
	minor      int
	patch      int
	pre        []semverIdentifier
	build      string
	raw        string // original string representation
	hasVPrefix bool   // whether the original version had a 'v' prefix
}

// semverVersion is the internal representation (kept for backwards compatibility)
type semverVersion struct {
	major int
	minor int
	patch int
	pre   []semverIdentifier
	build string
}

func compareSemverFull(a, b string) (Comparison, error) {
	av, err := parseSemverVersion(a)
	if err != nil {
		return ComparisonUnknown, fmt.Errorf("invalid semver '%s': %w", a, err)
	}
	bv, err := parseSemverVersion(b)
	if err != nil {
		return ComparisonUnknown, fmt.Errorf("invalid semver '%s': %w", b, err)
	}
	return compareSemverVersions(av, bv), nil
}

func compareSemverCompact(a, b string) (Comparison, error) {
	av, err := parseSemverVersion(a)
	if err != nil {
		return ComparisonUnknown, fmt.Errorf("invalid semver '%s': %w", a, err)
	}
	if len(av.pre) > 0 || av.build != "" {
		return ComparisonUnknown, fmt.Errorf("semver-compact forbids prerelease or build metadata: %s", a)
	}

	bv, err := parseSemverVersion(b)
	if err != nil {
		return ComparisonUnknown, fmt.Errorf("invalid semver '%s': %w", b, err)
	}
	if len(bv.pre) > 0 || bv.build != "" {
		return ComparisonUnknown, fmt.Errorf("semver-compact forbids prerelease or build metadata: %s", b)
	}

	return compareSemverVersions(av, bv), nil
}

func parseSemverVersion(input string) (*semverVersion, error) {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return nil, errors.New("empty version")
	}

	matches := semverPattern.FindStringSubmatch(trimmed)
	if len(matches) == 0 {
		return nil, fmt.Errorf("invalid format")
	}

	major, err := strconv.Atoi(matches[1])
	if err != nil {
		return nil, fmt.Errorf("segment '%s': %w", matches[1], err)
	}
	if len(matches[1]) > 1 && strings.HasPrefix(matches[1], "0") {
		return nil, fmt.Errorf("invalid major segment: leading zeros not allowed")
	}
	minor, err := strconv.Atoi(matches[2])
	if err != nil {
		return nil, fmt.Errorf("segment '%s': %w", matches[2], err)
	}
	if len(matches[2]) > 1 && strings.HasPrefix(matches[2], "0") {
		return nil, fmt.Errorf("invalid minor segment: leading zeros not allowed")
	}
	patch, err := strconv.Atoi(matches[3])
	if err != nil {
		return nil, fmt.Errorf("segment '%s': %w", matches[3], err)
	}
	if len(matches[3]) > 1 && strings.HasPrefix(matches[3], "0") {
		return nil, fmt.Errorf("invalid patch segment: leading zeros not allowed")
	}

	version := &semverVersion{
		major: major,
		minor: minor,
		patch: patch,
	}

	if prerelease := matches[4]; prerelease != "" {
		parts := strings.Split(prerelease, ".")
		version.pre = make([]semverIdentifier, len(parts))
		for i, part := range parts {
			if part == "" {
				return nil, fmt.Errorf("invalid prerelease identifier: empty segment")
			}
			if isNumeric(part) {
				if len(part) > 1 && strings.HasPrefix(part, "0") {
					return nil, fmt.Errorf("invalid prerelease identifier: leading zeros not allowed")
				}
				num, err := strconv.Atoi(part)
				if err != nil {
					return nil, fmt.Errorf("invalid prerelease identifier '%s': %w", part, err)
				}
				version.pre[i] = semverIdentifier{raw: part, numeric: true, num: num}
			} else {
				version.pre[i] = semverIdentifier{raw: part}
			}
		}
	}

	if build := matches[5]; build != "" {
		parts := strings.Split(build, ".")
		for _, part := range parts {
			if part == "" {
				return nil, fmt.Errorf("invalid build identifier: empty segment")
			}
			if isNumeric(part) && len(part) > 1 && strings.HasPrefix(part, "0") {
				return nil, fmt.Errorf("invalid build identifier: leading zeros not allowed")
			}
		}
		version.build = build
	}

	return version, nil
}

func compareSemverVersions(a, b *semverVersion) Comparison {
	if a.major != b.major {
		if a.major < b.major {
			return ComparisonLess
		}
		return ComparisonGreater
	}
	if a.minor != b.minor {
		if a.minor < b.minor {
			return ComparisonLess
		}
		return ComparisonGreater
	}
	if a.patch != b.patch {
		if a.patch < b.patch {
			return ComparisonLess
		}
		return ComparisonGreater
	}

	if len(a.pre) == 0 && len(b.pre) == 0 {
		return ComparisonEqual
	}
	if len(a.pre) == 0 {
		return ComparisonGreater
	}
	if len(b.pre) == 0 {
		return ComparisonLess
	}

	limit := len(a.pre)
	if len(b.pre) < limit {
		limit = len(b.pre)
	}

	for i := 0; i < limit; i++ {
		ai := a.pre[i]
		bi := b.pre[i]
		if ai.numeric && bi.numeric {
			if ai.num < bi.num {
				return ComparisonLess
			}
			if ai.num > bi.num {
				return ComparisonGreater
			}
			continue
		}
		if ai.numeric && !bi.numeric {
			return ComparisonLess
		}
		if !ai.numeric && bi.numeric {
			return ComparisonGreater
		}
		if cmp := strings.Compare(ai.raw, bi.raw); cmp != 0 {
			if cmp < 0 {
				return ComparisonLess
			}
			return ComparisonGreater
		}
	}

	if len(a.pre) < len(b.pre) {
		return ComparisonLess
	}
	if len(a.pre) > len(b.pre) {
		return ComparisonGreater
	}

	return ComparisonEqual
}

func compareCalver(a, b string) (Comparison, error) {
	aParts, err := parseCalver(a)
	if err != nil {
		return ComparisonUnknown, fmt.Errorf("invalid calver '%s': %w", a, err)
	}
	bParts, err := parseCalver(b)
	if err != nil {
		return ComparisonUnknown, fmt.Errorf("invalid calver '%s': %w", b, err)
	}

	longest := len(aParts)
	if len(bParts) > longest {
		longest = len(bParts)
	}

	for len(aParts) < longest {
		aParts = append(aParts, 0)
	}
	for len(bParts) < longest {
		bParts = append(bParts, 0)
	}

	for i := 0; i < longest; i++ {
		if aParts[i] < bParts[i] {
			return ComparisonLess, nil
		}
		if aParts[i] > bParts[i] {
			return ComparisonGreater, nil
		}
	}
	return ComparisonEqual, nil
}

func parseCalver(v string) ([]int, error) {
	trimmed := strings.TrimSpace(v)
	if trimmed == "" {
		return nil, errors.New("empty version")
	}

	matches := calverPattern.FindStringSubmatch(trimmed)
	if len(matches) == 0 {
		return nil, fmt.Errorf("calver requires strict format YYYY.MM or YYYY.MM.DD with consistent separators")
	}

	sep := matches[2]
	if matches[4] != "" && matches[4] != sep {
		return nil, fmt.Errorf("calver requires consistent separators")
	}

	year, err := strconv.Atoi(matches[1])
	if err != nil {
		return nil, fmt.Errorf("year '%s': %w", matches[1], err)
	}
	if year <= 0 {
		return nil, fmt.Errorf("invalid year %d", year)
	}

	month, err := strconv.Atoi(matches[3])
	if err != nil {
		return nil, fmt.Errorf("month '%s': %w", matches[3], err)
	}
	if month < 1 || month > 12 {
		return nil, fmt.Errorf("invalid month %d", month)
	}

	segments := []int{year, month}

	if matches[5] != "" {
		day, err := strconv.Atoi(matches[5])
		if err != nil {
			return nil, fmt.Errorf("day '%s': %w", matches[5], err)
		}
		if day < 1 || day > 31 {
			return nil, fmt.Errorf("invalid day %d", day)
		}
		segments = append(segments, day)
	}

	return segments, nil
}

func compareLexical(a, b string) Comparison {
	cmp := strings.Compare(strings.TrimSpace(a), strings.TrimSpace(b))
	if cmp < 0 {
		return ComparisonLess
	}
	if cmp > 0 {
		return ComparisonGreater
	}
	return ComparisonEqual
}

func matchString(target string, set []string) bool {
	if len(set) == 0 {
		return false
	}
	target = strings.TrimSpace(target)
	for _, candidate := range set {
		if target == strings.TrimSpace(candidate) {
			return true
		}
	}
	return false
}

// SortDisallowed returns a sorted copy of provided versions for consistent reporting.
func SortDisallowed(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	items := append([]string(nil), values...)
	sort.SliceStable(items, func(i, j int) bool {
		cmp, err := compareSemverFull(items[i], items[j])
		if err == nil {
			return cmp == ComparisonLess
		}
		return strings.TrimSpace(items[i]) < strings.TrimSpace(items[j])
	})
	return items
}

func isNumeric(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// ParseLenient parses a version string with lenient validation
func ParseLenient(input string) (*Version, error) {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return nil, errors.New("empty version")
	}

	matches := semverPattern.FindStringSubmatch(trimmed)
	if len(matches) == 0 {
		return nil, fmt.Errorf("invalid format")
	}

	major, err := strconv.Atoi(matches[1])
	if err != nil {
		return nil, fmt.Errorf("segment '%s': %w", matches[1], err)
	}
	if len(matches[1]) > 1 && strings.HasPrefix(matches[1], "0") {
		return nil, fmt.Errorf("invalid major segment: leading zeros not allowed")
	}
	minor, err := strconv.Atoi(matches[2])
	if err != nil {
		return nil, fmt.Errorf("segment '%s': %w", matches[2], err)
	}
	if len(matches[2]) > 1 && strings.HasPrefix(matches[2], "0") {
		return nil, fmt.Errorf("invalid minor segment: leading zeros not allowed")
	}
	patch, err := strconv.Atoi(matches[3])
	if err != nil {
		return nil, fmt.Errorf("segment '%s': %w", matches[3], err)
	}
	if len(matches[3]) > 1 && strings.HasPrefix(matches[3], "0") {
		return nil, fmt.Errorf("invalid patch segment: leading zeros not allowed")
	}

	version := &Version{
		major:      major,
		minor:      minor,
		patch:      patch,
		raw:        trimmed,
		hasVPrefix: strings.HasPrefix(trimmed, "v"),
	}

	if prerelease := matches[4]; prerelease != "" {
		parts := strings.Split(prerelease, ".")
		version.pre = make([]semverIdentifier, len(parts))
		for i, part := range parts {
			if part == "" {
				return nil, fmt.Errorf("invalid prerelease identifier: empty segment")
			}
			if isNumeric(part) {
				if len(part) > 1 && strings.HasPrefix(part, "0") {
					return nil, fmt.Errorf("invalid prerelease identifier: leading zeros not allowed")
				}
				num, err := strconv.Atoi(part)
				if err != nil {
					return nil, fmt.Errorf("invalid prerelease identifier '%s': %w", part, err)
				}
				version.pre[i] = semverIdentifier{raw: part, numeric: true, num: num}
			} else {
				version.pre[i] = semverIdentifier{raw: part}
			}
		}
	}

	if build := matches[5]; build != "" {
		parts := strings.Split(build, ".")
		for _, part := range parts {
			if part == "" {
				return nil, fmt.Errorf("invalid build identifier: empty segment")
			}
			if isNumeric(part) && len(part) > 1 && strings.HasPrefix(part, "0") {
				return nil, fmt.Errorf("invalid build identifier: leading zeros not allowed")
			}
		}
		version.build = build
	}

	return version, nil
}

// String returns the string representation of the version
func (v *Version) String() string {
	if v == nil {
		return ""
	}
	return v.raw
}

// BumpMajor increments the major version and resets minor and patch
func (v *Version) BumpMajor() *Version {
	if v == nil {
		return nil
	}
	newV := &Version{
		major:      v.major + 1,
		minor:      0,
		patch:      0,
		pre:        nil, // Clear prerelease on bump
		build:      "",
		raw:        "",
		hasVPrefix: v.hasVPrefix,
	}
	newV.updateRaw()
	return newV
}

// BumpMinor increments the minor version and resets patch
func (v *Version) BumpMinor() *Version {
	if v == nil {
		return nil
	}
	newV := &Version{
		major:      v.major,
		minor:      v.minor + 1,
		patch:      0,
		pre:        nil, // Clear prerelease on bump
		build:      "",
		raw:        "",
		hasVPrefix: v.hasVPrefix,
	}
	newV.updateRaw()
	return newV
}

// BumpPatch increments the patch version
func (v *Version) BumpPatch() *Version {
	if v == nil {
		return nil
	}
	newV := &Version{
		major:      v.major,
		minor:      v.minor,
		patch:      v.patch + 1,
		pre:        nil, // Clear prerelease on bump
		build:      "",
		raw:        "",
		hasVPrefix: v.hasVPrefix,
	}
	newV.updateRaw()
	return newV
}

// updateRaw updates the raw string representation
func (v *Version) updateRaw() {
	if v == nil {
		return
	}
	result := fmt.Sprintf("%d.%d.%d", v.major, v.minor, v.patch)
	if len(v.pre) > 0 {
		parts := make([]string, len(v.pre))
		for i, id := range v.pre {
			parts[i] = id.raw
		}
		result += "-" + strings.Join(parts, ".")
	}
	if v.build != "" {
		result += "+" + v.build
	}
	// Preserve 'v' prefix if original had it
	if v.hasVPrefix {
		result = "v" + result
	}
	v.raw = result
}

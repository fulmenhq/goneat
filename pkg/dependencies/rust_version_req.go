package dependencies

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"golang.org/x/mod/semver"
)

// Cargo version requirements, evaluated as the Rust semver crate (1.0.x)
// does: each comparator keeps its omitted components (they are not padded to
// a range), and a pre-release version only matches when some comparator on
// the same major.minor.patch names a pre-release. Unsupported syntax is an
// error so the license graph never guesses which declaration an edge belongs to.

type cargoOp int

const (
	cargoOpCaret cargoOp = iota
	cargoOpExact
	cargoOpGreater
	cargoOpGreaterEq
	cargoOpLess
	cargoOpLessEq
	cargoOpTilde
	cargoOpWildcard
)

type cargoComparator struct {
	op           cargoOp
	major        uint64
	minor, patch *uint64
	pre          string
}

type cargoVersion struct {
	major, minor, patch uint64
	pre                 string
}

var (
	cargoComparatorPattern = regexp.MustCompile(`^(\^|~|=|>=|<=|>|<)?\s*(\d+)(?:\.(\*|[xX]|\d+))?(?:\.(\*|[xX]|\d+))?(?:-([0-9A-Za-z.-]+))?(?:\+[0-9A-Za-z.-]+)?$`)
	cargoVersionPattern    = regexp.MustCompile(`^(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)(?:-([0-9A-Za-z.-]+))?(?:\+[0-9A-Za-z.-]+)?$`)
)

// parseCargoReq parses a requirement into comparators. A bare wildcard is the
// empty comparator list, like semver's STAR; an empty string is rejected, as
// semver does (cargo metadata reports unversioned dependencies as "*").
func parseCargoReq(req string) ([]cargoComparator, error) {
	req = strings.TrimSpace(req)
	if req == "*" || req == "x" || req == "X" {
		return nil, nil
	}
	if req == "" {
		return nil, fmt.Errorf("empty Cargo version requirement")
	}
	var comparators []cargoComparator
	for _, part := range strings.Split(req, ",") {
		part = strings.TrimSpace(part)
		m := cargoComparatorPattern.FindStringSubmatch(part)
		if m == nil {
			return nil, fmt.Errorf("unsupported Cargo version requirement %q", req)
		}
		c := cargoComparator{op: cargoOpCaret, pre: m[5]}
		switch m[1] {
		case "=":
			c.op = cargoOpExact
		case ">":
			c.op = cargoOpGreater
		case ">=":
			c.op = cargoOpGreaterEq
		case "<":
			c.op = cargoOpLess
		case "<=":
			c.op = cargoOpLessEq
		case "~":
			c.op = cargoOpTilde
		}
		defaultOp := m[1] == ""
		var err error
		if c.major, err = strconv.ParseUint(m[2], 10, 64); err != nil {
			return nil, fmt.Errorf("unsupported Cargo version requirement %q", req)
		}
		wildcard := false
		for i, field := range []string{m[3], m[4]} {
			switch {
			case field == "":
			case field == "*" || field == "x" || field == "X":
				wildcard = true
				if defaultOp {
					c.op = cargoOpWildcard
				}
			case wildcard:
				return nil, fmt.Errorf("unsupported Cargo version requirement %q", req)
			default:
				n, err := strconv.ParseUint(field, 10, 64)
				if err != nil {
					return nil, fmt.Errorf("unsupported Cargo version requirement %q", req)
				}
				if i == 0 {
					c.minor = &n
				} else {
					c.patch = &n
				}
			}
		}
		// semver only accepts pre-release/build after a full major.minor.patch.
		if (c.pre != "" || strings.Contains(part, "+")) && c.patch == nil {
			return nil, fmt.Errorf("unsupported Cargo version requirement %q", req)
		}
		if c.pre != "" && !semver.IsValid("v0.0.0-"+c.pre) {
			return nil, fmt.Errorf("unsupported Cargo version requirement %q", req)
		}
		comparators = append(comparators, c)
	}
	return comparators, nil
}

func parseCargoVersion(version string) (cargoVersion, error) {
	m := cargoVersionPattern.FindStringSubmatch(strings.TrimSpace(version))
	if m == nil || (m[4] != "" && !semver.IsValid("v0.0.0-"+m[4])) {
		return cargoVersion{}, fmt.Errorf("invalid Cargo package version %q", version)
	}
	var v cargoVersion
	var err error
	for i, dst := range []*uint64{&v.major, &v.minor, &v.patch} {
		if *dst, err = strconv.ParseUint(m[i+1], 10, 64); err != nil {
			return cargoVersion{}, fmt.Errorf("invalid Cargo package version %q", version)
		}
	}
	v.pre = m[4]
	return v, nil
}

// cargoVersionMatches reports whether version satisfies req (semver
// matches_req). An empty or bare-wildcard requirement matches every release
// but no pre-release, exactly as the semver crate does.
func cargoVersionMatches(req, version string) (bool, error) {
	comparators, err := parseCargoReq(req)
	if err != nil {
		return false, err
	}
	v, err := parseCargoVersion(version)
	if err != nil {
		return false, err
	}
	return cargoMatchesReq(comparators, v), nil
}

func cargoMatchesReq(comparators []cargoComparator, v cargoVersion) bool {
	for _, c := range comparators {
		if !cargoMatchesImpl(c, v) {
			return false
		}
	}
	if v.pre == "" {
		return true
	}
	for _, c := range comparators {
		if c.pre != "" && c.major == v.major && c.minor != nil && *c.minor == v.minor && c.patch != nil && *c.patch == v.patch {
			return true
		}
	}
	return false
}

// cargoPreCompare orders pre-release strings by SemVer precedence; the empty
// string (a release) sorts after every pre-release.
func cargoPreCompare(a, b string) int {
	switch {
	case a == b:
		return 0
	case a == "":
		return 1
	case b == "":
		return -1
	}
	return semver.Compare("v0.0.0-"+a, "v0.0.0-"+b)
}

func cargoMatchesImpl(c cargoComparator, v cargoVersion) bool {
	switch c.op {
	case cargoOpExact, cargoOpWildcard:
		return cargoMatchesExact(c, v)
	case cargoOpGreater:
		return cargoMatchesGreater(c, v)
	case cargoOpGreaterEq:
		return cargoMatchesExact(c, v) || cargoMatchesGreater(c, v)
	case cargoOpLess:
		return cargoMatchesLess(c, v)
	case cargoOpLessEq:
		return cargoMatchesExact(c, v) || cargoMatchesLess(c, v)
	case cargoOpTilde:
		return cargoMatchesTilde(c, v)
	default:
		return cargoMatchesCaret(c, v)
	}
}

func cargoMatchesExact(c cargoComparator, v cargoVersion) bool {
	if v.major != c.major || (c.minor != nil && v.minor != *c.minor) || (c.patch != nil && v.patch != *c.patch) {
		return false
	}
	return v.pre == c.pre
}

func cargoMatchesGreater(c cargoComparator, v cargoVersion) bool {
	if v.major != c.major {
		return v.major > c.major
	}
	if c.minor == nil {
		return false
	}
	if v.minor != *c.minor {
		return v.minor > *c.minor
	}
	if c.patch == nil {
		return false
	}
	if v.patch != *c.patch {
		return v.patch > *c.patch
	}
	return cargoPreCompare(v.pre, c.pre) > 0
}

func cargoMatchesLess(c cargoComparator, v cargoVersion) bool {
	if v.major != c.major {
		return v.major < c.major
	}
	if c.minor == nil {
		return false
	}
	if v.minor != *c.minor {
		return v.minor < *c.minor
	}
	if c.patch == nil {
		return false
	}
	if v.patch != *c.patch {
		return v.patch < *c.patch
	}
	return cargoPreCompare(v.pre, c.pre) < 0
}

func cargoMatchesTilde(c cargoComparator, v cargoVersion) bool {
	if v.major != c.major || (c.minor != nil && v.minor != *c.minor) {
		return false
	}
	if c.patch != nil && v.patch != *c.patch {
		return v.patch > *c.patch
	}
	return cargoPreCompare(v.pre, c.pre) >= 0
}

func cargoMatchesCaret(c cargoComparator, v cargoVersion) bool {
	if v.major != c.major {
		return false
	}
	if c.minor == nil {
		return true
	}
	minor := *c.minor
	if c.patch == nil {
		if c.major > 0 {
			return v.minor >= minor
		}
		return v.minor == minor
	}
	patch := *c.patch
	switch {
	case c.major > 0:
		if v.minor != minor {
			return v.minor > minor
		}
		if v.patch != patch {
			return v.patch > patch
		}
	case minor > 0:
		if v.minor != minor {
			return false
		}
		if v.patch != patch {
			return v.patch > patch
		}
	default:
		if v.minor != minor || v.patch != patch {
			return false
		}
	}
	return cargoPreCompare(v.pre, c.pre) >= 0
}

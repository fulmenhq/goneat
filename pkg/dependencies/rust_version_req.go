package dependencies

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"golang.org/x/mod/semver"
)

// cargoComparator matches one Cargo version-requirement comparator. Partial
// versions and wildcards are allowed; anything else is rejected so the license
// graph never guesses which declaration a resolved edge belongs to.
var cargoComparator = regexp.MustCompile(`^(\^|~|=|>=|<=|>|<)?\s*(\*|[xX]|\d+)(?:\.(\*|[xX]|\d+))?(?:\.(\*|[xX]|\d+))?(?:-([0-9A-Za-z.-]+))?(?:\+[0-9A-Za-z.-]+)?$`)

// cargoVersionMatches reports whether version satisfies a Cargo requirement
// string as written in cargo metadata (`dependencies[].req`). It follows
// Cargo's semver rules: caret by default, partial upper bounds widen, build
// metadata is ignored, and a pre-release version only matches a requirement
// that names a pre-release.
func cargoVersionMatches(req, version string) (bool, error) {
	v, err := cargoSemver(version)
	if err != nil {
		return false, err
	}
	var comparators []string
	for _, part := range strings.Split(req, ",") {
		if part = strings.TrimSpace(part); part != "" {
			comparators = append(comparators, part)
		}
	}
	if len(comparators) == 0 {
		return semver.Prerelease(v) == "", nil
	}
	// A pre-release version is admitted only if some comparator names a
	// pre-release on the same major.minor.patch (Cargo / semver crate rule);
	// ordering then follows SemVer identifier precedence (alpha.2 < alpha.10).
	if semver.Prerelease(v) != "" {
		core := strings.TrimSuffix(v, semver.Prerelease(v))
		admitted := false
		for _, comparator := range comparators {
			m := cargoComparator.FindStringSubmatch(comparator)
			if m == nil || m[5] == "" || m[3] == "" || m[4] == "" {
				continue
			}
			if fmt.Sprintf("v%s.%s.%s", m[2], m[3], m[4]) == semver.Canonical(core) {
				admitted = true
			}
		}
		if !admitted {
			return false, nil
		}
	}
	for _, comparator := range comparators {
		ok, err := cargoComparatorMatches(comparator, v)
		if err != nil || !ok {
			return false, err
		}
	}
	return true, nil
}

func cargoSemver(version string) (string, error) {
	v := "v" + strings.SplitN(strings.TrimSpace(version), "+", 2)[0]
	if !semver.IsValid(v) || semver.Canonical(v) != v {
		return "", fmt.Errorf("invalid Cargo package version %q", version)
	}
	return v, nil
}

func cargoComparatorMatches(comparator, v string) (bool, error) {
	m := cargoComparator.FindStringSubmatch(comparator)
	if m == nil {
		return false, fmt.Errorf("unsupported Cargo version requirement %q", comparator)
	}
	op, pre := m[1], m[5]
	var nums []int
	for _, field := range m[2:5] {
		if field == "" {
			break
		}
		if field == "*" || field == "x" || field == "X" {
			if pre != "" {
				return false, fmt.Errorf("unsupported Cargo version requirement %q", comparator)
			}
			if op == "" || op == "^" || op == "~" || op == "=" {
				op = "~"
			}
			break
		}
		n, err := strconv.Atoi(field)
		if err != nil {
			return false, fmt.Errorf("unsupported Cargo version requirement %q", comparator)
		}
		nums = append(nums, n)
	}
	if len(nums) == 0 {
		return true, nil // bare wildcard
	}
	if pre != "" && len(nums) < 3 {
		return false, fmt.Errorf("unsupported Cargo version requirement %q", comparator)
	}
	lower := cargoVersionString(nums, pre)
	// upper returns the first version excluded by bumping position i.
	upper := func(i int) string {
		bumped := make([]int, i+1)
		copy(bumped, nums[:i+1])
		bumped[i]++
		return cargoVersionString(bumped, "0")
	}
	cmp := semver.Compare(v, lower)
	n := len(nums)
	switch op {
	case "=":
		if n == 3 {
			return cmp == 0, nil
		}
		return cmp >= 0 && semver.Compare(v, upper(n-1)) < 0, nil
	case ">":
		if n < 3 {
			return semver.Compare(v, upper(n-1)) >= 0, nil
		}
		return cmp > 0, nil
	case ">=":
		return cmp >= 0, nil
	case "<":
		return cmp < 0, nil
	case "<=":
		if n < 3 {
			return semver.Compare(v, upper(n-1)) < 0, nil
		}
		return cmp <= 0, nil
	case "~":
		if n == 1 {
			return cmp >= 0 && semver.Compare(v, upper(0)) < 0, nil
		}
		return cmp >= 0 && semver.Compare(v, upper(1)) < 0, nil
	default: // caret
		switch {
		case nums[0] > 0 || n == 1:
			return cmp >= 0 && semver.Compare(v, upper(0)) < 0, nil
		case n == 2 || nums[1] > 0:
			return cmp >= 0 && semver.Compare(v, upper(1)) < 0, nil
		default:
			return cmp >= 0 && semver.Compare(v, upper(2)) < 0, nil
		}
	}
}

// cargoVersionString pads a partial version with zeros. The "0" pre-release
// on exclusive upper bounds keeps pre-releases of the bound itself excluded.
func cargoVersionString(nums []int, pre string) string {
	parts := []int{0, 0, 0}
	copy(parts, nums)
	s := fmt.Sprintf("v%d.%d.%d", parts[0], parts[1], parts[2])
	if pre != "" {
		s += "-" + pre
	}
	return s
}

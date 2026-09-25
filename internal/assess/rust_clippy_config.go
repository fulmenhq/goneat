package assess

import (
	"fmt"
	"regexp"
	"strings"
)

// rustLintOverrides is lint.rust in .goneat/assess.yaml.
type rustLintOverrides struct {
	Clippy *clippyOverrides `yaml:"clippy"`
}

// clippyOverrides is lint.rust.clippy in .goneat/assess.yaml. Every field is
// optional; unset fields keep the historical invocation
// (cargo clippy --message-format=json [--workspace]).
type clippyOverrides struct {
	Enabled           *bool    `yaml:"enabled"`
	Toolchain         string   `yaml:"toolchain"`
	AllTargets        *bool    `yaml:"all_targets"`
	AllFeatures       *bool    `yaml:"all_features"`
	Features          []string `yaml:"features"`
	NoDefaultFeatures *bool    `yaml:"no_default_features"`
	Locked            *bool    `yaml:"locked"`
	Packages          []string `yaml:"packages"`
	Targets           []string `yaml:"targets"`
}

func clippySettingsFrom(overrides *assessOverrides) clippyOverrides {
	if overrides == nil || overrides.Lint == nil || overrides.Lint.Rust == nil || overrides.Lint.Rust.Clippy == nil {
		return clippyOverrides{}
	}
	return *overrides.Lint.Rust.Clippy
}

func (c clippyOverrides) enabled() bool {
	return boolWithDefault(c.Enabled, true)
}

// Structured values become individual argv tokens (never a shell string).
// Each must look like the Cargo/rustup token it stands for and must not start
// with '-', so a config value cannot smuggle in an extra flag.
var (
	rustToolchainToken = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)
	rustTargetToken    = regexp.MustCompile(`^[A-Za-z0-9_][A-Za-z0-9_.-]*$`)
	rustFeatureToken   = regexp.MustCompile(`^[A-Za-z0-9_][A-Za-z0-9_+./-]*$`)
	rustPackageToken   = regexp.MustCompile(`^[A-Za-z0-9_][A-Za-z0-9_.:@/+-]*$`)
)

func validateRustTokens(field string, values []string, pattern *regexp.Regexp) error {
	for _, v := range values {
		if !pattern.MatchString(v) {
			return fmt.Errorf("invalid lint.rust.clippy.%s value %q", field, v)
		}
	}
	return nil
}

func (c clippyOverrides) validate() error {
	if c.Toolchain != "" {
		if err := validateRustTokens("toolchain", []string{c.Toolchain}, rustToolchainToken); err != nil {
			return err
		}
	}
	if err := validateRustTokens("features", c.Features, rustFeatureToken); err != nil {
		return err
	}
	if err := validateRustTokens("packages", c.Packages, rustPackageToken); err != nil {
		return err
	}
	return validateRustTokens("targets", c.Targets, rustTargetToken)
}

// invocations returns the cargo argv for each clippy run: one run per
// configured target triple, or a single run for the host target.
func (c clippyOverrides) invocations(project *RustProject) ([][]string, error) {
	if err := c.validate(); err != nil {
		return nil, err
	}

	var base []string
	if c.Toolchain != "" {
		base = append(base, "+"+c.Toolchain)
	}
	base = append(base, "clippy", "--message-format=json")
	if len(c.Packages) > 0 {
		for _, pkg := range c.Packages {
			base = append(base, "-p", pkg)
		}
	} else if project.IsWorkspace || project.IsWorkspaceMember {
		base = append(base, "--workspace")
	}
	if boolWithDefault(c.AllTargets, false) {
		base = append(base, "--all-targets")
	}
	if boolWithDefault(c.AllFeatures, false) {
		base = append(base, "--all-features")
	}
	if boolWithDefault(c.NoDefaultFeatures, false) {
		base = append(base, "--no-default-features")
	}
	if len(c.Features) > 0 {
		base = append(base, "--features", strings.Join(c.Features, ","))
	}
	if boolWithDefault(c.Locked, false) {
		base = append(base, "--locked")
	}

	if len(c.Targets) == 0 {
		return [][]string{base}, nil
	}
	runs := make([][]string, 0, len(c.Targets))
	for _, triple := range c.Targets {
		args := append(append([]string{}, base...), "--target", triple)
		runs = append(runs, args)
	}
	return runs, nil
}

package dependencies

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"github.com/pelletier/go-toml/v2"
)

// The license graph is deliberately bounded: all workspace roots and all
// targets, the configured feature selection, and no graph exclusions. A
// filtered target/exclusion graph needs its own reconciliation implementation.
type rustLicenseScope struct {
	AllFeatures       bool     `toml:"all-features"`
	NoDefaultFeatures bool     `toml:"no-default-features"`
	Features          []string `toml:"features"`
	IncludeDev        bool     `toml:"-"`
	IncludeBuild      bool     `toml:"-"`
}

type rustLicensePackage struct {
	ID           string  `json:"id"`
	Name         string  `json:"name"`
	Version      string  `json:"version"`
	Source       *string `json:"source"`
	License      *string `json:"license"`
	LicenseFile  *string `json:"license_file"`
	ManifestPath string  `json:"manifest_path"`
}

type rustLicenseMetadata struct {
	Packages         []rustLicensePackage `json:"packages"`
	WorkspaceMembers []string             `json:"workspace_members"`
	Resolve          *struct {
		Nodes []rustLicenseNode `json:"nodes"`
	} `json:"resolve"`
}

type rustLicenseNode struct {
	ID           string   `json:"id"`
	Dependencies []string `json:"dependencies"`
	Deps         []struct {
		Pkg      string `json:"pkg"`
		DepKinds []struct {
			Kind *string `json:"kind"`
		} `json:"dep_kinds"`
	} `json:"deps"`
}

type rustLicenseSnapshot struct {
	Path          string
	ConfigPath    string
	ConfigHash    [32]byte
	MetadataHash  [32]byte
	ExceptionPath string
	ExceptionHash [32]byte
	Scope         rustLicenseScope
	CLI           rustLicenseCLI
	Expected      map[string]rustLicensePackage
	cleanup       func()
}

// Before cargo-deny 0.20, graph flags were global but metadata/config belonged
// to list/check. Discover each flag's advertised location without abandoning
// the shared snapshot or assuming every flag moved with it.
type rustLicenseCLI struct {
	MetadataGlobal bool
	ConfigGlobal   bool
}

func rustLicenseHelpHasFlag(help, flag string) bool {
	for _, line := range strings.Split(help, "\n") {
		fields := strings.Fields(line)
		if len(fields) > 0 && fields[0] == flag {
			return true
		}
		if len(fields) > 1 && strings.HasPrefix(fields[0], "-") && strings.HasSuffix(fields[0], ",") && fields[1] == flag {
			return true
		}
	}
	return false
}

func rustLicenseCLIFromHelp(global, list, check string, scope rustLicenseScope) (rustLicenseCLI, error) {
	for _, flag := range []string{"--format", "--layout"} {
		if !rustLicenseHelpHasFlag(list, flag) {
			return rustLicenseCLI{}, fmt.Errorf("cargo-deny list does not advertise required %s; use a cargo-deny version supporting crate-oriented JSON inventory", flag)
		}
	}
	for _, flag := range append([]string{"--format", "--manifest-path", "--workspace"}, scope.featureArgs()...) {
		if strings.HasPrefix(flag, "--") && !rustLicenseHelpHasFlag(global, flag) {
			return rustLicenseCLI{}, fmt.Errorf("cargo-deny license CLI does not advertise required global %s; use a cargo-deny version supporting the complete configured snapshot scope", flag)
		}
	}
	var cli rustLicenseCLI
	for _, option := range []struct {
		flag   string
		global *bool
	}{
		{"--metadata-path", &cli.MetadataGlobal},
		{"--config", &cli.ConfigGlobal},
	} {
		*option.global = rustLicenseHelpHasFlag(global, option.flag)
		// A global option is the common evidenced position for both consumers
		// even if a subcommand also advertises it. Dual advertising alone is
		// not ambiguity (list's own --format has separate output semantics).
		for command, help := range map[string]string{"list": list, "check": check} {
			local := rustLicenseHelpHasFlag(help, option.flag)
			if !*option.global && !local {
				return cli, fmt.Errorf("cargo-deny %s does not advertise required %s globally or on the subcommand; use a cargo-deny version supporting shared metadata snapshots and explicit policy", command, option.flag)
			}
		}
	}
	return cli, nil
}

func detectRustLicenseCLI(ctx context.Context, root string, scope rustLicenseScope) (rustLicenseCLI, error) {
	help := make(map[string]string)
	for _, command := range []string{"", "list", "check"} {
		args := []string{"deny"}
		if command != "" {
			args = append(args, command)
		}
		cmd := exec.CommandContext(ctx, "cargo", append(args, "--help")...)
		cmd.Dir = root
		out, err := cmd.Output()
		if err != nil {
			return rustLicenseCLI{}, fmt.Errorf("cannot inspect cargo-deny %s license CLI: %w", command, err)
		}
		help[command] = string(out)
	}
	return rustLicenseCLIFromHelp(help[""], help["list"], help["check"], scope)
}

func licenseGraphConfig(data []byte, path string) (rustLicenseScope, error) {
	var config map[string]interface{}
	if err := toml.Unmarshal(data, &config); err != nil {
		return rustLicenseScope{}, fmt.Errorf("invalid Rust license configuration %s: %w", path, err)
	}
	graph := config
	if value, ok := config["graph"]; ok {
		var valid bool
		graph, valid = value.(map[string]interface{})
		if !valid {
			return rustLicenseScope{}, fmt.Errorf("invalid graph configuration in %s", path)
		}
		// Do not silently choose between legacy and current graph tables.
		for _, key := range []string{"targets", "exclude", "features", "all-features", "no-default-features", "exclude-dev", "exclude-unpublished"} {
			if _, ok := config[key]; ok {
				return rustLicenseScope{}, fmt.Errorf("incomplete Rust license scope: %s mixes graph.%s and legacy graph keys", path, key)
			}
		}
	}
	for _, key := range []string{"exclude-dev", "exclude-unpublished"} {
		if value, ok := graph[key]; ok && value != false {
			return rustLicenseScope{}, fmt.Errorf("incomplete Rust license scope: %s configures %s; graph filtering is not supported", path, key)
		}
	}
	if _, ok := config["graph"]; ok {
		for key := range graph {
			switch key {
			case "targets", "exclude", "features", "all-features", "no-default-features", "exclude-dev", "exclude-unpublished":
			default:
				return rustLicenseScope{}, fmt.Errorf("unsupported Rust license graph configuration %s: graph.%s", path, key)
			}
		}
	}
	for _, key := range []string{"targets", "exclude"} {
		if value, ok := graph[key]; ok {
			values, valid := value.([]interface{})
			if !valid || len(values) > 0 {
				return rustLicenseScope{}, fmt.Errorf("incomplete Rust license scope: %s configures graph %s; only all-target workspace graphs without exclusions are supported", path, key)
			}
		}
	}
	// Typed decoding validates feature types instead of guessing truth values.
	encoded, err := toml.Marshal(graph)
	if err != nil {
		return rustLicenseScope{}, fmt.Errorf("encode graph configuration %s: %w", path, err)
	}
	var scope rustLicenseScope
	if err := toml.Unmarshal(encoded, &scope); err != nil {
		return scope, fmt.Errorf("invalid graph feature selection in %s: %w", path, err)
	}
	// cargo-deny applies these license-stage filters after building the crate
	// graph. Never override them merely to make inventory reconciliation pass.
	scope.IncludeBuild = true
	if raw, ok := config["licenses"]; ok {
		licenses, ok := raw.(map[string]interface{})
		if !ok {
			return scope, fmt.Errorf("invalid licenses configuration in %s", path)
		}
		for key, dst := range map[string]*bool{"include-dev": &scope.IncludeDev, "include-build": &scope.IncludeBuild} {
			if raw, exists := licenses[key]; exists {
				value, ok := raw.(bool)
				if !ok {
					return scope, fmt.Errorf("invalid licenses.%s configuration in %s", key, path)
				}
				*dst = value
			}
		}
	}
	return scope, nil
}

func (s rustLicenseScope) featureArgs() []string {
	var args []string
	if s.AllFeatures {
		args = append(args, "--all-features")
	}
	if s.NoDefaultFeatures {
		args = append(args, "--no-default-features")
	}
	if len(s.Features) > 0 {
		args = append(args, "--features", strings.Join(s.Features, ","))
	}
	return args
}

func captureRustLicenseSnapshot(ctx context.Context, project *RustProject) (*rustLicenseSnapshot, error) {
	root, err := filepath.Abs(project.EffectiveRoot())
	if err != nil {
		return nil, err
	}
	// Always name the policy file on both consumers. No implicit search may
	// select a different graph configuration than the one inspected here.
	configPath := filepath.Join(root, "deny.toml")
	configData, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("read Rust license configuration %s: %w (create an explicit deny.toml at the workspace root)", configPath, err)
	}
	scope, err := licenseGraphConfig(configData, configPath)
	if err != nil {
		return nil, err
	}
	cli, err := detectRustLicenseCLI(ctx, root, scope)
	if err != nil {
		return nil, err
	}
	exceptionPath, exceptionHash, err := rustLicenseExceptions(root)
	if err != nil {
		return nil, err
	}
	args := []string{"metadata", "--format-version", "1", "--manifest-path", filepath.Join(root, "Cargo.toml")}
	args = append(args, scope.featureArgs()...)
	cmd := exec.CommandContext(ctx, "cargo", args...)
	cmd.Dir = root
	var stderr strings.Builder
	cmd.Stderr = &stderr
	data, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("rust license metadata capture failed: %w: %s", err, strings.TrimSpace(stderr.String()))
	}
	expected, err := expectedRustLicensePackages(data)
	if err != nil {
		return nil, err
	}
	licenseSet, err := expectedRustLicenseStage(data, scope)
	if err != nil {
		return nil, err
	}
	var omitted []string
	for key, pkg := range expected {
		if retained, ok := licenseSet[key]; !ok || retained.ID != pkg.ID {
			omitted = append(omitted, pkg.ID)
		}
	}
	if len(omitted) > 0 {
		sort.Strings(omitted)
		return nil, fmt.Errorf("incomplete Rust license scope: %s licenses.include-dev=%t and licenses.include-build=%t omit coverage for %s; filtered license-stage coverage is unsupported (use a policy covering the complete retained graph)", configPath, scope.IncludeDev, scope.IncludeBuild, strings.Join(omitted, ", "))
	}
	dir, err := os.MkdirTemp("", "goneat-rust-license-")
	if err != nil {
		return nil, err
	}
	cleanup := func() { _ = os.RemoveAll(dir) } // Only this newly-created private scratch directory.
	path := filepath.Join(dir, "metadata.json")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		cleanup()
		return nil, err
	}
	return &rustLicenseSnapshot{Path: path, ConfigPath: configPath, ConfigHash: sha256.Sum256(configData), MetadataHash: sha256.Sum256(data), ExceptionPath: exceptionPath, ExceptionHash: exceptionHash, Scope: scope, CLI: cli, Expected: expected, cleanup: cleanup}, nil
}

func (s *rustLicenseSnapshot) denyArgs(command string) []string {
	args := []string{"deny", "--format", "json", "--manifest-path", filepath.Join(filepath.Dir(s.ConfigPath), "Cargo.toml"), "--workspace"}
	args = append(args, s.Scope.featureArgs()...)
	var local []string
	for _, option := range []struct {
		flag, value string
		global      bool
	}{
		{"--metadata-path", s.Path, s.CLI.MetadataGlobal},
		{"--config", s.ConfigPath, s.CLI.ConfigGlobal},
	} {
		if option.global {
			args = append(args, option.flag, option.value)
		} else {
			local = append(local, option.flag, option.value)
		}
	}
	return append(append(args, command), local...)
}

func (s *rustLicenseSnapshot) verifyConfig() error {
	data, err := os.ReadFile(s.ConfigPath)
	if err != nil || sha256.Sum256(data) != s.ConfigHash {
		return fmt.Errorf("rust license configuration %s changed or became unreadable during analysis", s.ConfigPath)
	}
	data, err = os.ReadFile(s.Path)
	if err != nil || sha256.Sum256(data) != s.MetadataHash {
		return fmt.Errorf("rust license metadata snapshot changed or became unreadable during analysis")
	}
	path, hash, err := rustLicenseExceptions(filepath.Dir(s.ConfigPath))
	if err != nil {
		return err
	}
	if path != s.ExceptionPath || hash != s.ExceptionHash {
		return fmt.Errorf("rust license exception configuration changed during analysis")
	}
	return nil
}

// Match cargo-deny's ancestor search for optional local license exceptions.
// These files affect policy, not graph coverage, and must be stable too.
func rustLicenseExceptions(root string) (string, [32]byte, error) {
	for {
		for _, name := range []string{"deny.exceptions.toml", ".deny.exceptions.toml", ".cargo/deny.exceptions.toml"} {
			path := filepath.Join(root, name)
			data, err := os.ReadFile(path)
			if err == nil {
				return path, sha256.Sum256(data), nil
			}
			if !os.IsNotExist(err) {
				return "", [32]byte{}, fmt.Errorf("read Rust license exceptions %s: %w", path, err)
			}
		}
		parent := filepath.Dir(root)
		if parent == root {
			break
		}
		root = parent
	}
	return "", [32]byte{}, nil
}

func expectedRustLicensePackages(data []byte) (map[string]rustLicensePackage, error) {
	return expectedRustLicensePackagesWithKinds(data, true, true)
}

func expectedRustLicenseStage(data []byte, scope rustLicenseScope) (map[string]rustLicensePackage, error) {
	if scope.IncludeDev || scope.IncludeBuild {
		return expectedRustLicensePackagesWithKinds(data, scope.IncludeDev, scope.IncludeBuild)
	}
	// cargo-deny intersects separately dev-filtered and build-filtered crate
	// sets. Removing both edge kinds in one traversal is not equivalent when
	// a crate has distinct dev and build paths from workspace roots.
	withoutDev, err := expectedRustLicensePackagesWithKinds(data, false, true)
	if err != nil {
		return nil, err
	}
	withoutBuild, err := expectedRustLicensePackagesWithKinds(data, true, false)
	if err != nil {
		return nil, err
	}
	intersection := map[string]rustLicensePackage{}
	for key, pkg := range withoutDev {
		if other, ok := withoutBuild[key]; ok && other.ID == pkg.ID {
			intersection[key] = pkg
		}
	}
	return intersection, nil
}

func expectedRustLicensePackagesWithKinds(data []byte, includeDev, includeBuild bool) (map[string]rustLicensePackage, error) {
	var metadata rustLicenseMetadata
	if err := json.Unmarshal(data, &metadata); err != nil {
		return nil, fmt.Errorf("decode Rust license metadata: %w", err)
	}
	if len(metadata.Packages) == 0 || len(metadata.WorkspaceMembers) == 0 || metadata.Resolve == nil || len(metadata.Resolve.Nodes) == 0 {
		return nil, fmt.Errorf("rust license metadata has an empty or unresolved workspace graph")
	}
	packages := map[string]rustLicensePackage{}
	for _, pkg := range metadata.Packages {
		if pkg.ID == "" || pkg.Name == "" || pkg.Version == "" {
			return nil, fmt.Errorf("rust license metadata contains a package without complete identity")
		}
		if _, exists := packages[pkg.ID]; exists {
			return nil, fmt.Errorf("rust license metadata repeats package ID %q", pkg.ID)
		}
		packages[pkg.ID] = pkg
	}
	workspace := map[string]bool{}
	for _, id := range metadata.WorkspaceMembers {
		workspace[id] = true
	}
	nodes := map[string][]string{}
	for _, node := range metadata.Resolve.Nodes {
		if _, exists := nodes[node.ID]; exists {
			return nil, fmt.Errorf("rust license metadata repeats resolve node %q", node.ID)
		}
		// Match cargo-deny's default: dev edges are retained for workspace
		// packages only. Use resolved edge kinds, never manifest re-evaluation.
		allEdges := map[string]bool{}
		var retained []string
		for _, dep := range node.Deps {
			if dep.Pkg == "" || len(dep.DepKinds) == 0 {
				return nil, fmt.Errorf("incomplete Rust resolve edge for %q", node.ID)
			}
			allEdges[dep.Pkg] = true
			include := false
			for _, kind := range dep.DepKinds {
				if kind.Kind == nil {
					include = true
					continue
				}
				switch *kind.Kind {
				case "normal":
					include = true
				case "build":
					include = include || includeBuild
				case "dev":
					include = include || (workspace[node.ID] && includeDev)
				default:
					return nil, fmt.Errorf("unsupported Rust dependency kind %q for %q", *kind.Kind, node.ID)
				}
			}
			if include {
				retained = append(retained, dep.Pkg)
			}
		}
		if len(allEdges) != len(node.Dependencies) {
			return nil, fmt.Errorf("rust resolve edge coverage is incomplete for %q", node.ID)
		}
		for _, id := range node.Dependencies {
			if !allEdges[id] {
				return nil, fmt.Errorf("rust resolve edge %q from %q omitted its kind evidence", id, node.ID)
			}
		}
		nodes[node.ID] = retained
	}
	// Start at every workspace member, not every raw package. Only reachable
	// resolved dependencies belong to this all-workspace graph.
	queue := append([]string(nil), metadata.WorkspaceMembers...)
	visited := map[string]bool{}
	expected := map[string]rustLicensePackage{}
	for len(queue) > 0 {
		id := queue[0]
		queue = queue[1:]
		if visited[id] {
			continue
		}
		visited[id] = true
		pkg, exists := packages[id]
		deps, resolved := nodes[id]
		if !exists || !resolved {
			return nil, fmt.Errorf("rust license metadata omits package or resolve node %q", id)
		}
		key, err := rustLicenseCrateKey(pkg)
		if err != nil {
			return nil, err
		}
		if _, exists := expected[key]; exists {
			return nil, fmt.Errorf("rust license crate identity collision for %q", key)
		}
		expected[key] = pkg
		queue = append(queue, deps...)
	}
	return expected, nil
}

func rustLicenseCrateKey(pkg rustLicensePackage) (string, error) {
	// Modern Cargo package IDs retain the source before '#', even for path
	// crates whose source field is null. Do not guess from name/version alone.
	source, _, valid := strings.Cut(pkg.ID, "#")
	if !valid || source == "" {
		return "", fmt.Errorf("unsupported Cargo package ID %q: source identity cannot be reconciled", pkg.ID)
	}
	if pkg.Source == nil && !strings.HasPrefix(source, "path+") {
		return "", fmt.Errorf("cargo package %q has no source identity", pkg.ID)
	}
	if pkg.Source != nil {
		metadataSource := *pkg.Source
		// Modern git package IDs omit the resolved commit; metadata.Source
		// retains it. Keep the full source in the reported inventory.
		if strings.HasPrefix(metadataSource, "git+") {
			metadataSource, _, _ = strings.Cut(metadataSource, "#")
		}
		if metadataSource != source {
			return "", fmt.Errorf("cargo package %q has mismatched or unsupported source identity", pkg.ID)
		}
	}
	return pkg.Name + " " + pkg.Version + " " + source, nil
}

func reconcileRustLicenseList(data []byte, expected map[string]rustLicensePackage) ([]CargoCrateLicense, error) {
	// Streaming the object retains duplicate keys, which unmarshalling into a
	// map would silently collapse and wrongly treat as unique crate evidence.
	rows := map[string]struct {
		Licenses []string `json:"licenses"`
	}{}
	decoder := json.NewDecoder(bytes.NewReader(data))
	start, err := decoder.Token()
	if err != nil || start != json.Delim('{') {
		return nil, fmt.Errorf("cargo-deny crate list is not a JSON object")
	}
	for decoder.More() {
		token, err := decoder.Token()
		if err != nil {
			return nil, fmt.Errorf("decode crate key: %w", err)
		}
		key, ok := token.(string)
		if !ok {
			return nil, fmt.Errorf("invalid cargo-deny crate key")
		}
		if _, exists := rows[key]; exists {
			return nil, fmt.Errorf("duplicate cargo-deny crate identity %q", key)
		}
		var row struct {
			Licenses []string `json:"licenses"`
		}
		if err := decoder.Decode(&row); err != nil {
			return nil, fmt.Errorf("decode cargo-deny crate %q: %w", key, err)
		}
		if row.Licenses == nil {
			return nil, fmt.Errorf("cargo-deny crate %q omitted license evidence", key)
		}
		rows[key] = row
	}
	if _, err := decoder.Token(); err != nil {
		return nil, fmt.Errorf("truncated cargo-deny crate list: %w", err)
	}
	var trailing interface{}
	if err := decoder.Decode(&trailing); err != io.EOF {
		return nil, fmt.Errorf("cargo-deny crate list has trailing or malformed data")
	}
	if len(rows) == 0 || len(expected) == 0 {
		return nil, fmt.Errorf("rust license inventory or expected crate set is empty")
	}
	for key := range rows {
		if _, ok := expected[key]; !ok {
			return nil, fmt.Errorf("cargo-deny list returned unexpected crate identity %q", key)
		}
	}
	var keys []string
	for key := range expected {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	var omitted []string
	for _, key := range keys {
		if _, ok := rows[key]; !ok {
			omitted = append(omitted, fmt.Sprintf("%q (crate %q)", expected[key].ID, key))
		}
	}
	if len(omitted) > 0 {
		return nil, fmt.Errorf("incomplete Rust license inventory: cargo-deny list omitted gathered package IDs: %s; verify that cargo-deny list applies the configured license-stage settings: cargo-deny 0.19.0 does not apply licenses.include-dev=true to dev-only crates; use a release that honors this setting (verified with 0.20.2) and rerun without excluding gathered crates", strings.Join(omitted, ", "))
	}
	crates := make([]CargoCrateLicense, 0, len(keys))
	for _, key := range keys {
		pkg := expected[key]
		expression := ""
		if pkg.License != nil {
			expression = *pkg.License
		}
		if expression != "" {
			requirements := parseLicenseExpression(expression)
			observed := append([]string(nil), rows[key].Licenses...)
			sort.Strings(requirements)
			sort.Strings(observed)
			if !slices.Equal(requirements, observed) {
				return nil, fmt.Errorf("cargo-deny license evidence for %q does not match its authoritative SPDX expression %q", key, expression)
			}
		}
		source, _, _ := strings.Cut(pkg.ID, "#")
		if pkg.Source != nil {
			source = *pkg.Source
		}
		// JSON's flattened license IDs are retained as diagnostics only. They
		// cannot reconstruct AND/OR or replace the authoritative expression.
		crates = append(crates, CargoCrateLicense{Name: pkg.Name, Version: pkg.Version, Licenses: rows[key].Licenses, PackageID: pkg.ID, Expression: expression, Source: source})
	}
	return crates, nil
}

func parseRustLicenseCheck(data []byte, checks []CargoDenyCheckType) ([]cargoDenyEntry, error) {
	var entries []cargoDenyEntry
	scanner := bufio.NewScanner(bytes.NewReader(data))
	scanner.Buffer(make([]byte, 4096), 4*1024*1024)
	complete := false
	diagnosticErrors := 0
	for scanner.Scan() {
		line := bytes.TrimSpace(scanner.Bytes())
		if len(line) == 0 {
			continue
		}
		if complete {
			return nil, fmt.Errorf("cargo-deny emitted data after its completion summary")
		}
		var envelope struct {
			Type   string          `json:"type"`
			Fields json.RawMessage `json:"fields"`
		}
		if err := json.Unmarshal(line, &envelope); err != nil {
			return nil, fmt.Errorf("invalid cargo-deny check evidence: %w", err)
		}
		switch envelope.Type {
		case "summary":
			var summary map[string]struct {
				Errors   *int `json:"errors"`
				Warnings *int `json:"warnings"`
				Notes    *int `json:"notes"`
				Helps    *int `json:"helps"`
			}
			if err := json.Unmarshal(envelope.Fields, &summary); err != nil {
				return nil, fmt.Errorf("invalid cargo-deny completion summary: %w", err)
			}
			summaryErrors := 0
			for _, check := range checks {
				counts, ok := summary[string(check)]
				if !ok || counts.Errors == nil || counts.Warnings == nil || counts.Notes == nil || counts.Helps == nil || *counts.Errors < 0 || *counts.Warnings < 0 || *counts.Notes < 0 || *counts.Helps < 0 {
					return nil, fmt.Errorf("incomplete cargo-deny completion summary for %s", check)
				}
				summaryErrors += *counts.Errors
			}
			if diagnosticErrors != summaryErrors {
				return nil, fmt.Errorf("cargo-deny policy diagnostics do not match completion error counts")
			}
			complete = true
		case "diagnostic":
			var entry cargoDenyEntry
			if err := json.Unmarshal(line, &entry); err != nil || entry.Fields == nil || entry.Fields.Code == "" || entry.Fields.Severity == "" {
				return nil, fmt.Errorf("incomplete cargo-deny diagnostic")
			}
			switch entry.Fields.Severity {
			case "error", "warning", "note", "help":
			default:
				return nil, fmt.Errorf("unsupported cargo-deny diagnostic severity %q", entry.Fields.Severity)
			}
			entries = append(entries, entry)
			if entry.Fields.Severity == "error" {
				diagnosticErrors++
			}
		default:
			// JSON logging can report degraded graph collection without affecting
			// cargo-deny's exit status. Never silently discard that evidence.
			return nil, fmt.Errorf("cargo-deny check reported non-policy or unsupported evidence: %s", line)
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read cargo-deny check evidence: %w", err)
	}
	if !complete {
		return nil, fmt.Errorf("cargo-deny check did not produce a complete license/ban summary")
	}
	return entries, nil
}

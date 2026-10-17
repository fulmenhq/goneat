package dependencies

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// The Rust license graph reproduces what cargo-deny lists from the same
// metadata snapshot: krates builds the active graph (resolving features from
// the workspace roots and following only activated edges), then cargo-deny's
// license gatherer applies the include-dev/include-build stage by reachability
// over that graph. Raw resolve edges are not enough: cargo keeps an edge when
// any declaration exists, including optional dependencies that no feature
// enables and never-true cfg(any()) targets.

type rustGraphPackage struct {
	ID           string                  `json:"id"`
	Name         string                  `json:"name"`
	Version      string                  `json:"version"`
	Source       *string                 `json:"source"`
	License      *string                 `json:"license"`
	LicenseFile  *string                 `json:"license_file"`
	ManifestPath string                  `json:"manifest_path"`
	Dependencies *[]rustGraphDeclaration `json:"dependencies"`
	Features     *map[string][]string    `json:"features"`
}

type rustGraphDeclaration struct {
	Name                string   `json:"name"`
	Rename              *string  `json:"rename"`
	Req                 string   `json:"req"`
	Kind                *string  `json:"kind"`
	Target              *string  `json:"target"`
	Optional            bool     `json:"optional"`
	UsesDefaultFeatures bool     `json:"uses_default_features"`
	Features            []string `json:"features"`
	Source              *string  `json:"source"`
	Path                *string  `json:"path"`
}

type rustGraphMetadata struct {
	Packages         []rustGraphPackage `json:"packages"`
	WorkspaceMembers []string           `json:"workspace_members"`
	Resolve          *struct {
		Nodes []rustGraphNode `json:"nodes"`
	} `json:"resolve"`
}

type rustGraphNode struct {
	ID           string             `json:"id"`
	Dependencies []string           `json:"dependencies"`
	Deps         []rustGraphNodeDep `json:"deps"`
	Features     *[]string          `json:"features"`
}

type rustGraphNodeDep struct {
	Name     string `json:"name"`
	Pkg      string `json:"pkg"`
	DepKinds []struct {
		Kind   *string `json:"kind"`
		Target *string `json:"target"`
	} `json:"dep_kinds"`
}

type rustGraphEdge struct {
	to   string
	kind string // normal, build, dev
}

// rustLicenseGraph is the active crate graph before license-stage filtering.
type rustLicenseGraph struct {
	packages  map[string]rustLicensePackage
	workspace []string
	edges     map[string][]rustGraphEdge
}

type rustGraphVisit struct {
	id      string
	feature string // empty = non-optional fill
}

func buildRustLicenseGraph(data []byte) (*rustLicenseGraph, error) {
	var metadata rustGraphMetadata
	if err := json.Unmarshal(data, &metadata); err != nil {
		return nil, fmt.Errorf("decode Rust license metadata: %w", err)
	}
	if len(metadata.Packages) == 0 || len(metadata.WorkspaceMembers) == 0 || metadata.Resolve == nil || len(metadata.Resolve.Nodes) == 0 {
		return nil, fmt.Errorf("rust license metadata has an empty or unresolved workspace graph")
	}
	packages := map[string]rustGraphPackage{}
	for _, pkg := range metadata.Packages {
		if pkg.ID == "" || pkg.Name == "" || pkg.Version == "" {
			return nil, fmt.Errorf("rust license metadata contains a package without complete identity")
		}
		if _, exists := packages[pkg.ID]; exists {
			return nil, fmt.Errorf("rust license metadata repeats package ID %q", pkg.ID)
		}
		packages[pkg.ID] = pkg
	}
	nodes := map[string]rustGraphNode{}
	for _, node := range metadata.Resolve.Nodes {
		if _, exists := nodes[node.ID]; exists {
			return nil, fmt.Errorf("rust license metadata repeats resolve node %q", node.ID)
		}
		edgeIDs := map[string]bool{}
		for _, dep := range node.Deps {
			if dep.Pkg == "" || len(dep.DepKinds) == 0 {
				return nil, fmt.Errorf("incomplete Rust resolve edge for %q", node.ID)
			}
			for _, kind := range dep.DepKinds {
				if _, err := rustGraphKind(kind.Kind); err != nil {
					return nil, fmt.Errorf("%w for %q", err, node.ID)
				}
			}
			edgeIDs[dep.Pkg] = true
		}
		if len(edgeIDs) != len(node.Dependencies) {
			return nil, fmt.Errorf("rust resolve edge coverage is incomplete for %q", node.ID)
		}
		for _, id := range node.Dependencies {
			if !edgeIDs[id] {
				return nil, fmt.Errorf("rust resolve edge %q from %q omitted its kind evidence", id, node.ID)
			}
		}
		nodes[node.ID] = node
	}
	workspace := map[string]bool{}
	for _, id := range metadata.WorkspaceMembers {
		workspace[id] = true
	}
	// Feature and declaration evidence is required only for the packages the
	// walk actually reaches; check lazily so unrelated lock entries cannot fail.
	lookup := func(id string) (rustGraphPackage, rustGraphNode, error) {
		pkg, okPkg := packages[id]
		node, okNode := nodes[id]
		if !okPkg || !okNode {
			return pkg, node, fmt.Errorf("rust license metadata omits package or resolve node %q", id)
		}
		if pkg.Dependencies == nil || pkg.Features == nil || node.Features == nil {
			return pkg, node, fmt.Errorf("rust license metadata omits dependency or feature evidence for %q", id)
		}
		return pkg, node, nil
	}

	b := &rustGraphBuilder{
		lookup:      lookup,
		workspace:   workspace,
		filled:      map[string]bool{},
		actual:      map[string]map[string]bool{},
		edges:       map[string]map[rustGraphEdge]bool{},
		pendingWeak: map[string]map[string]map[string]bool{},
	}
	// Roots: workspace members with their metadata features. The snapshot was
	// produced with the configured feature selection, so root features are
	// exact; everything below the roots is re-derived from activated edges.
	for _, root := range metadata.WorkspaceMembers {
		_, node, err := lookup(root)
		if err != nil {
			return nil, err
		}
		b.stack = append(b.stack, rustGraphVisit{id: root})
		for _, feature := range *node.Features {
			b.stack = append(b.stack, rustGraphVisit{id: root, feature: feature})
		}
	}
	if err := b.run(); err != nil {
		return nil, err
	}

	graph := &rustLicenseGraph{packages: map[string]rustLicensePackage{}, workspace: append([]string(nil), metadata.WorkspaceMembers...), edges: map[string][]rustGraphEdge{}}
	reached := map[string]bool{}
	for _, id := range metadata.WorkspaceMembers {
		reached[id] = true
	}
	for from, set := range b.edges {
		reached[from] = true
		for edge := range set {
			reached[edge.to] = true
			graph.edges[from] = append(graph.edges[from], edge)
		}
		sort.Slice(graph.edges[from], func(i, j int) bool {
			a, c := graph.edges[from][i], graph.edges[from][j]
			return a.to < c.to || (a.to == c.to && a.kind < c.kind)
		})
	}
	for id := range reached {
		pkg := packages[id]
		graph.packages[id] = rustLicensePackage{ID: pkg.ID, Name: pkg.Name, Version: pkg.Version, Source: pkg.Source, License: pkg.License, LicenseFile: pkg.LicenseFile, ManifestPath: pkg.ManifestPath}
	}
	return graph, nil
}

type rustGraphBuilder struct {
	lookup      func(string) (rustGraphPackage, rustGraphNode, error)
	workspace   map[string]bool
	stack       []rustGraphVisit
	filled      map[string]bool
	actual      map[string]map[string]bool
	edges       map[string]map[rustGraphEdge]bool
	pendingWeak map[string]map[string]map[string]bool // pid -> dep -> features
}

// candidate is a dependency reached during one visit; strong means a feature
// explicitly named it (dep:x, x/f, or x?/f once x is present).
type rustGraphCandidate struct {
	dep      rustGraphNodeDep
	strong   bool
	features map[string]bool
}

func (b *rustGraphBuilder) run() error {
	for len(b.stack) > 0 {
		visit := b.stack[len(b.stack)-1]
		b.stack = b.stack[:len(b.stack)-1]
		if visit.feature == "" && b.filled[visit.id] {
			continue
		}
		if visit.feature != "" && b.actual[visit.id][visit.feature] {
			continue
		}
		pkg, node, err := b.lookup(visit.id)
		if err != nil {
			return err
		}
		candidates := map[string]*rustGraphCandidate{}
		var order []string
		add := func(dep rustGraphNodeDep, strong bool) *rustGraphCandidate {
			c, ok := candidates[dep.Pkg]
			if !ok {
				c = &rustGraphCandidate{dep: dep, features: map[string]bool{}}
				candidates[dep.Pkg] = c
				order = append(order, dep.Pkg)
			}
			c.strong = c.strong || strong
			return c
		}
		if visit.feature != "" {
			if b.actual[visit.id] == nil {
				b.actual[visit.id] = map[string]bool{}
			}
			b.actual[visit.id][visit.feature] = true
			if !b.filled[visit.id] {
				b.stack = append(b.stack, rustGraphVisit{id: visit.id})
			}
			// krates skips a feature absent from the manifest (resolved
			// features can name crates.io-index-only features); do the same.
			for _, value := range (*pkg.Features)[visit.feature] {
				name, sub, weak, plain := parseRustFeatureValue(value)
				if plain {
					b.stack = append(b.stack, rustGraphVisit{id: visit.id, feature: name})
					continue
				}
				dep, ok := rustFeatureDependency(pkg, node, name)
				if !ok {
					continue // no resolve edge: inactive on every target
				}
				if weak && !b.hasEdge(visit.id, dep.Pkg) {
					b.parkWeak(visit.id, dep.Pkg, sub)
					continue
				}
				c := add(dep, true)
				for f := range b.takeWeak(visit.id, dep.Pkg) {
					c.features[f] = true
				}
				if sub != "" {
					c.features[sub] = true
				}
			}
		} else {
			b.filled[visit.id] = true
			for _, dep := range node.Deps {
				add(dep, false)
			}
		}
		for _, depID := range order {
			if err := b.admit(visit.id, pkg, candidates[depID]); err != nil {
				return err
			}
		}
	}
	return nil
}

func (b *rustGraphBuilder) admit(from string, pkg rustGraphPackage, c *rustGraphCandidate) error {
	target, _, err := b.lookup(c.dep.Pkg)
	if err != nil {
		return err
	}
	type admitted struct {
		kind     string
		features []string
	}
	var edges []admitted
	for _, depKind := range c.dep.DepKinds {
		kind, _ := rustGraphKind(depKind.Kind)
		if kind == "dev" && !b.workspace[from] {
			continue // cargo never resolves dev-dependencies below the workspace
		}
		decl, err := rustGraphDeclarationFor(pkg, c.dep, target, kind, depKind.Target)
		if err != nil {
			return err
		}
		if decl.Optional && !c.strong {
			continue
		}
		if depKind.Target != nil {
			never, err := rustCfgNeverTrue(*depKind.Target)
			if err != nil {
				return fmt.Errorf("rust dependency %q -> %q: %w", from, c.dep.Pkg, err)
			}
			if never {
				continue
			}
		}
		features := append([]string(nil), decl.Features...)
		if _, ok := (*target.Features)["default"]; ok && decl.UsesDefaultFeatures {
			features = append(features, "default")
		}
		edges = append(edges, admitted{kind: kind, features: features})
	}
	if len(edges) == 0 {
		return nil
	}
	b.stack = append(b.stack, rustGraphVisit{id: c.dep.Pkg})
	for f := range b.takeWeak(from, c.dep.Pkg) {
		b.stack = append(b.stack, rustGraphVisit{id: c.dep.Pkg, feature: f})
	}
	for f := range c.features {
		b.stack = append(b.stack, rustGraphVisit{id: c.dep.Pkg, feature: f})
	}
	if b.edges[from] == nil {
		b.edges[from] = map[rustGraphEdge]bool{}
	}
	for _, edge := range edges {
		b.edges[from][rustGraphEdge{to: c.dep.Pkg, kind: edge.kind}] = true
		for _, f := range edge.features {
			b.stack = append(b.stack, rustGraphVisit{id: c.dep.Pkg, feature: f})
		}
	}
	return nil
}

func (b *rustGraphBuilder) hasEdge(from, to string) bool {
	for edge := range b.edges[from] {
		if edge.to == to {
			return true
		}
	}
	return false
}

func (b *rustGraphBuilder) parkWeak(from, to, feature string) {
	if b.pendingWeak[from] == nil {
		b.pendingWeak[from] = map[string]map[string]bool{}
	}
	if b.pendingWeak[from][to] == nil {
		b.pendingWeak[from][to] = map[string]bool{}
	}
	b.pendingWeak[from][to][feature] = true
}

func (b *rustGraphBuilder) takeWeak(from, to string) map[string]bool {
	pending := b.pendingWeak[from][to]
	delete(b.pendingWeak[from], to)
	return pending
}

func rustGraphKind(kind *string) (string, error) {
	if kind == nil {
		return "normal", nil
	}
	switch *kind {
	case "normal", "build", "dev":
		return *kind, nil
	default:
		return "", fmt.Errorf("unsupported Rust dependency kind %q", *kind)
	}
}

// parseRustFeatureValue splits a [features] entry: dep:x, x?/f (weak), x/f
// (strong), or a plain feature name.
func parseRustFeatureValue(value string) (name, sub string, weak, plain bool) {
	if strings.HasPrefix(value, "dep:") {
		return value[4:], "", false, false
	}
	if i := strings.Index(value, "?/"); i >= 0 {
		return value[:i], value[i+2:], true, false
	}
	if i := strings.Index(value, "/"); i >= 0 {
		return value[:i], value[i+1:], false, false
	}
	return value, "", false, true
}

func rustNormalizedName(name string) string {
	return strings.ReplaceAll(name, "-", "_")
}

// rustFeatureDependency resolves a feature's dependency name to a resolve edge.
// Resolve names are library target names; features use the declared name
// (rename or package name).
func rustFeatureDependency(pkg rustGraphPackage, node rustGraphNode, name string) (rustGraphNodeDep, bool) {
	renamed := map[string]bool{}
	for _, decl := range *pkg.Dependencies {
		if decl.Rename != nil && decl.Name == name {
			renamed[rustNormalizedName(*decl.Rename)] = true
		}
	}
	for _, dep := range node.Deps {
		if rustNormalizedName(dep.Name) == rustNormalizedName(name) {
			return dep, true
		}
	}
	for _, dep := range node.Deps {
		if !strings.Contains(dep.Pkg, "#") {
			continue
		}
		if rustPackageIDName(dep.Pkg) != name {
			continue
		}
		if len(renamed) == 0 || renamed[rustNormalizedName(dep.Name)] {
			return dep, true
		}
	}
	return rustGraphNodeDep{}, false
}

// rustPackageIDName extracts the package name from a modern Cargo package ID
// (`source#name@version`, or `source#version` when the name is the last path
// segment of the source).
func rustPackageIDName(id string) string {
	source, tail, _ := strings.Cut(id, "#")
	if name, _, ok := strings.Cut(tail, "@"); ok {
		return name
	}
	source = strings.SplitN(source, "?", 2)[0]
	return source[strings.LastIndex(source, "/")+1:]
}

// rustGraphDeclarationFor finds the manifest declaration that produced one
// resolved dependency kind: same kind, same target string, matching name,
// version requirement, and then source or path when still ambiguous.
func rustGraphDeclarationFor(pkg rustGraphPackage, dep rustGraphNodeDep, target rustGraphPackage, kind string, cfg *string) (rustGraphDeclaration, error) {
	var candidates []rustGraphDeclaration
	for _, decl := range *pkg.Dependencies {
		declKind, err := rustGraphKind(decl.Kind)
		if err != nil {
			return decl, fmt.Errorf("%w in %q", err, pkg.ID)
		}
		if declKind != kind || !rustTargetsEqual(cfg, decl.Target) {
			continue
		}
		name := decl.Name
		if decl.Rename != nil {
			name = *decl.Rename
		}
		if target.Name != name && rustNormalizedName(name) != rustNormalizedName(dep.Name) && rustNormalizedName(name) != rustNormalizedName(target.Name) {
			continue
		}
		ok, err := rustDeclarationVersionMatches(decl.Req, target.Version)
		if err != nil {
			return decl, fmt.Errorf("rust dependency %q -> %q: %w", pkg.ID, dep.Pkg, err)
		}
		if ok {
			candidates = append(candidates, decl)
		}
	}
	if len(candidates) > 1 {
		source, _, _ := strings.Cut(dep.Pkg, "#")
		var narrowed []rustGraphDeclaration
		for _, decl := range candidates {
			switch {
			case decl.Source != nil:
				declSource := *decl.Source
				if strings.HasPrefix(declSource, "git+") {
					declSource, _, _ = strings.Cut(declSource, "#")
				}
				if declSource == source {
					narrowed = append(narrowed, decl)
				}
			case decl.Path != nil:
				if source == "path+file://"+*decl.Path {
					narrowed = append(narrowed, decl)
				}
			}
		}
		candidates = narrowed
	}
	switch len(candidates) {
	case 1:
		return candidates[0], nil
	case 0:
		return rustGraphDeclaration{}, fmt.Errorf("rust resolve edge %q -> %q (%s) has no matching manifest declaration", pkg.ID, dep.Pkg, kind)
	default:
		return rustGraphDeclaration{}, fmt.Errorf("rust resolve edge %q -> %q (%s) matches several manifest declarations", pkg.ID, dep.Pkg, kind)
	}
}

// rustDeclarationVersionMatches is semver matching plus krates' declaration
// rule: a requirement with no comparators (an unversioned path or git
// dependency, reported as "*") also admits a pre-release package version.
func rustDeclarationVersionMatches(req, version string) (bool, error) {
	comparators, err := parseCargoReq(req)
	if err != nil {
		return false, err
	}
	v, err := parseCargoVersion(version)
	if err != nil {
		return false, err
	}
	if len(comparators) == 0 && v.pre != "" {
		return true, nil
	}
	return cargoMatchesReq(comparators, v), nil
}

func rustTargetsEqual(a, b *string) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return *a == *b
}

// rustCfgNeverTrue mirrors krates in all-target mode: drop a cfg(...) target
// only when it contains no predicates and evaluates false (cfg(any()) and
// friends). Any expression containing a predicate is retained, even if it is
// logically false, and plain target triples are always retained.
func rustCfgNeverTrue(target string) (bool, error) {
	if !strings.HasPrefix(target, "cfg(") {
		return false, nil // plain target triple
	}
	if !strings.HasSuffix(target, ")") {
		return false, fmt.Errorf("malformed cfg expression %q", target)
	}
	tokens, err := rustCfgTokens(target[4 : len(target)-1])
	if err != nil {
		return false, err
	}
	// Parse the whole expression first so malformed input always fails,
	// even when a predicate means the edge would be retained anyway.
	pos := 0
	value, predicates, err := rustCfgEval(tokens, &pos, target)
	if err != nil {
		return false, err
	}
	if pos != len(tokens) {
		return false, fmt.Errorf("malformed cfg expression %q", target)
	}
	return predicates == 0 && !value, nil
}

type rustCfgToken struct {
	text      string
	predicate bool
}

func rustCfgTokens(expr string) ([]rustCfgToken, error) {
	var tokens []rustCfgToken
	for i := 0; i < len(expr); {
		ch := expr[i]
		switch {
		case ch == ' ' || ch == '\t' || ch == '\n':
			i++
		case ch == '(' || ch == ')' || ch == ',':
			tokens = append(tokens, rustCfgToken{text: string(ch)})
			i++
		case ch == '_' || ch >= 'a' && ch <= 'z' || ch >= 'A' && ch <= 'Z':
			j := i
			for j < len(expr) && (expr[j] == '_' || expr[j] >= 'a' && expr[j] <= 'z' || expr[j] >= 'A' && expr[j] <= 'Z' || expr[j] >= '0' && expr[j] <= '9') {
				j++
			}
			ident := expr[i:j]
			k := j
			for k < len(expr) && expr[k] == ' ' {
				k++
			}
			if k < len(expr) && expr[k] == '=' {
				k++
				for k < len(expr) && expr[k] == ' ' {
					k++
				}
				if k >= len(expr) || expr[k] != '"' {
					return nil, fmt.Errorf("malformed cfg predicate in %q", expr)
				}
				end := strings.IndexByte(expr[k+1:], '"')
				if end < 0 {
					return nil, fmt.Errorf("malformed cfg predicate in %q", expr)
				}
				tokens = append(tokens, rustCfgToken{text: expr[i : k+end+2], predicate: true})
				i = k + end + 2
				continue
			}
			isOp := (ident == "all" || ident == "any" || ident == "not") && k < len(expr) && expr[k] == '('
			tokens = append(tokens, rustCfgToken{text: ident, predicate: !isOp})
			i = j
		default:
			return nil, fmt.Errorf("malformed cfg expression %q", expr)
		}
	}
	return tokens, nil
}

// rustCfgEval parses one cfg node and returns its value (meaningful only when
// it contains no predicates) and how many predicates it contains.
func rustCfgEval(tokens []rustCfgToken, pos *int, target string) (bool, int, error) {
	malformed := fmt.Errorf("malformed cfg expression %q", target)
	if *pos >= len(tokens) {
		return false, 0, malformed
	}
	if tokens[*pos].predicate {
		*pos++
		return true, 1, nil
	}
	op := tokens[*pos].text
	if op != "all" && op != "any" && op != "not" {
		return false, 0, malformed
	}
	*pos++
	if *pos >= len(tokens) || tokens[*pos].text != "(" {
		return false, 0, malformed
	}
	*pos++
	var values []bool
	predicates := 0
	for {
		if *pos >= len(tokens) {
			return false, 0, malformed
		}
		if tokens[*pos].text == ")" {
			*pos++
			break
		}
		if len(values) > 0 {
			if tokens[*pos].text != "," {
				return false, 0, malformed
			}
			*pos++
			if *pos < len(tokens) && tokens[*pos].text == ")" { // trailing comma
				*pos++
				break
			}
		}
		value, count, err := rustCfgEval(tokens, pos, target)
		if err != nil {
			return false, 0, err
		}
		values = append(values, value)
		predicates += count
	}
	switch op {
	case "all":
		for _, v := range values {
			if !v {
				return false, predicates, nil
			}
		}
		return true, predicates, nil
	case "any":
		for _, v := range values {
			if v {
				return true, predicates, nil
			}
		}
		return false, predicates, nil
	default:
		if len(values) != 1 {
			return false, 0, malformed
		}
		return !values[0], predicates, nil
	}
}

// stage returns the packages cargo-deny's license gatherer keeps for the given
// include-dev/include-build settings: reachability over the already-built
// graph with the excluded edge kind skipped, intersected when both are off.
// It never re-resolves features.
func (g *rustLicenseGraph) stage(includeDev, includeBuild bool) map[string]rustLicensePackage {
	reach := func(skip string) map[string]bool {
		seen := map[string]bool{}
		queue := append([]string(nil), g.workspace...)
		for _, id := range queue {
			seen[id] = true
		}
		for len(queue) > 0 {
			id := queue[0]
			queue = queue[1:]
			for _, edge := range g.edges[id] {
				if edge.kind == skip || seen[edge.to] {
					continue
				}
				seen[edge.to] = true
				queue = append(queue, edge.to)
			}
		}
		return seen
	}
	var kept map[string]bool
	switch {
	case includeDev && includeBuild:
		kept = map[string]bool{}
		for id := range g.packages {
			kept[id] = true
		}
	case includeDev:
		kept = reach("build")
	case includeBuild:
		kept = reach("dev")
	default:
		withoutDev, withoutBuild := reach("dev"), reach("build")
		kept = map[string]bool{}
		for id := range withoutDev {
			if withoutBuild[id] {
				kept[id] = true
			}
		}
	}
	out := map[string]rustLicensePackage{}
	for id := range kept {
		out[id] = g.packages[id]
	}
	return out
}

// keyed converts a package-ID set into crate-identity keys, rejecting
// collisions the reconciliation could not tell apart.
func rustLicenseKeyed(set map[string]rustLicensePackage) (map[string]rustLicensePackage, error) {
	ids := make([]string, 0, len(set))
	for id := range set {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	keyed := map[string]rustLicensePackage{}
	for _, id := range ids {
		pkg := set[id]
		key, err := rustLicenseCrateKey(pkg)
		if err != nil {
			return nil, err
		}
		if _, exists := keyed[key]; exists {
			return nil, fmt.Errorf("rust license crate identity collision for %q", key)
		}
		keyed[key] = pkg
	}
	return keyed, nil
}

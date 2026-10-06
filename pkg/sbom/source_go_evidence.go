package sbom

import (
	"fmt"
	"io/fs"
	"path"
	"path/filepath"
	"strings"

	"golang.org/x/mod/modfile"
)

var rootGoEvidenceNames = []string{"go.mod", "go.sum", "go.work", "go.work.sum"}

// protectSourceGoEvidence validates only the graph declared by root Go evidence.
// It never opens the original tree, fetches local replacements, or restores a
// workspace member. Everything read here belongs to the reconciled capture.
func protectSourceGoEvidence(fsys fs.FS, selection *sourceSelection, patterns []sourcePattern, opts SourceOptions) error {
	hasFile := func(name string) bool {
		entry, exists := selection.entries[name]
		return exists && !entry.directory
	}
	if !hasFile("go.mod") && !hasFile("go.work") {
		return nil
	}
	for _, name := range rootGoEvidenceNames {
		if !hasFile(name) || opts.NoIgnore || selection.forceIncludes(name) {
			continue
		}
		for _, pattern := range patterns {
			if pattern.policySource == "configured" && pattern.excludes(selection.entries[name]) {
				return fmt.Errorf("source SBOM required root Go evidence %q is excluded by configured pattern %q; use literal --force-include or --no-ignore", name, pattern.pattern)
			}
		}
		if !selection.selected[name] {
			selection.selected[name] = true
			selection.protectedEvidence = append(selection.protectedEvidence, name)
		}
	}

	var workspace *modfile.WorkFile
	var members []string
	if hasFile("go.mod") {
		members = append(members, ".")
	}
	if hasFile("go.work") {
		data, err := fs.ReadFile(fsys, "go.work")
		if err != nil {
			return fmt.Errorf("read captured Go evidence go.work: %w", err)
		}
		workspace, err = modfile.ParseWork("go.work", data, nil)
		if err != nil {
			return fmt.Errorf("source SBOM invalid Go evidence go.work: %w", err)
		}
		for _, use := range workspace.Use {
			member, err := sourceGoLocalPath(".", use.Path)
			if err != nil {
				return err
			}
			members = append(members, member)
		}
	}
	visited := make(map[string]bool)
	for len(members) > 0 {
		member := members[0]
		members = members[1:]
		if visited[member] {
			continue
		}
		visited[member] = true
		name := path.Join(member, "go.mod")
		if !hasFile(name) || !selection.selected[name] {
			return fmt.Errorf("source SBOM incomplete Go scope: required member evidence %q is absent or excluded; select it explicitly without expanding outside the captured subject", name)
		}
		// Existing sums are graph evidence too; optional absence is not an error.
		sum := path.Join(member, "go.sum")
		if hasFile(sum) && !selection.selected[sum] {
			return fmt.Errorf("source SBOM incomplete Go scope: member evidence %q is excluded; select it explicitly", sum)
		}
		data, err := fs.ReadFile(fsys, name)
		if err != nil {
			return fmt.Errorf("read captured Go evidence %s: %w", name, err)
		}
		module, err := modfile.Parse(name, data, nil)
		if err != nil {
			return fmt.Errorf("source SBOM invalid Go evidence %s: %w", name, err)
		}
		if module.Module == nil {
			return fmt.Errorf("source SBOM incomplete Go scope: %q has no module declaration", name)
		}
		for _, require := range module.Require {
			replacement := sourceGoReplacement(module.Replace, require.Mod.Path, require.Mod.Version)
			base := member
			if workspace != nil {
				if candidate := sourceGoReplacement(workspace.Replace, require.Mod.Path, require.Mod.Version); candidate != nil {
					replacement, base = candidate, "."
				}
			}
			if replacement == nil || replacement.New.Version != "" {
				continue
			}
			local, err := sourceGoLocalPath(base, replacement.New.Path)
			if err != nil {
				return err
			}
			members = append(members, local)
		}
	}
	return nil
}

func sourceGoReplacement(replacements []*modfile.Replace, requiredPath, requiredVersion string) *modfile.Replace {
	var wildcard *modfile.Replace
	for _, replacement := range replacements {
		if replacement.Old.Path != requiredPath {
			continue
		}
		if replacement.Old.Version == requiredVersion {
			return replacement
		}
		if replacement.Old.Version == "" {
			wildcard = replacement
		}
	}
	return wildcard
}

func sourceGoLocalPath(base, reference string) (string, error) {
	// Reject foreign-platform absolute/ambiguous paths as well as host absolute
	// paths. A relative ../ reference is allowed only if it stays in the subject.
	if filepath.IsAbs(reference) || strings.ContainsAny(reference, "\\:\x00\r\n") || strings.HasPrefix(reference, "/") {
		return "", fmt.Errorf("source SBOM incomplete Go scope: local reference %q is outside or unsupported in the captured subject", reference)
	}
	name := path.Join(base, reference)
	if name == ".." || strings.HasPrefix(name, "../") || !fs.ValidPath(name) {
		return "", fmt.Errorf("source SBOM incomplete Go scope: local reference %q from %q is outside the captured subject", reference, base)
	}
	return name, nil
}

package dependencies

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/google/go-licenses/v2/licenses"
)

// collectLicenses adapts upstream's build-time GOROOT heuristic to the selected
// go command. Standard is authoritative even when GOTOOLCHAIN resolves a module
// toolchain different from the one that compiled goneat. No globals are changed.
func collectLicenses(ctx context.Context) (map[string]*License, bool, error) {
	toolchain, err := selectedLicenseToolchain(ctx)
	if err != nil {
		return nil, true, err
	}
	cmd := exec.CommandContext(ctx, "go", "list", "-deps", "-json", "./...")
	var stderr strings.Builder
	cmd.Stderr = &stderr
	data, err := cmd.Output()
	if err != nil {
		return nil, true, fmt.Errorf("license package discovery failed: %w: %s", err, strings.TrimSpace(stderr.String()))
	}
	pkgs, err := decodeLicensePackages(data)
	if err != nil {
		return nil, true, err
	}
	ignored, expected, err := licensePackageCoverage(pkgs, toolchain.GOROOT)
	if err != nil {
		return nil, true, err
	}
	classifier, err := licenses.NewClassifier()
	if err != nil {
		return nil, true, err
	}
	strict := &strictLicenseClassifier{classifier: classifier}
	libraries, err := licenses.Libraries(ctx, strict, false, ignored, "./...")
	if err != nil {
		return nil, true, err
	}
	if err := strict.collectionError(); err != nil {
		return nil, true, err
	}
	after, err := selectedLicenseToolchain(ctx)
	if err != nil {
		return nil, true, err
	}
	if after != toolchain {
		return nil, true, fmt.Errorf("selected Go toolchain changed during license collection")
	}
	inventory, err := mapLicenseLibraries(libraries, expected)
	return inventory, err != nil, err
}

// Upstream logs candidate read/classification errors and continues. Capture
// them so a requested assessment cannot silently certify partial evidence.
type strictLicenseClassifier struct {
	classifier licenses.Classifier
	mu         sync.Mutex
	errors     []string
}

func (c *strictLicenseClassifier) Identify(path string) ([]licenses.License, error) {
	result, err := c.classifier.Identify(path)
	if err != nil {
		c.mu.Lock()
		c.errors = append(c.errors, fmt.Sprintf("%s: %v", path, err))
		c.mu.Unlock()
	}
	return result, err
}

func (c *strictLicenseClassifier) collectionError() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.errors) == 0 {
		return nil
	}
	sort.Strings(c.errors)
	return fmt.Errorf("license candidate classification failed: %s", strings.Join(c.errors, "; "))
}

type licenseToolchain struct {
	GOROOT    string
	GOVERSION string
}

func selectedLicenseToolchain(ctx context.Context) (licenseToolchain, error) {
	cmd := exec.CommandContext(ctx, "go", "env", "-json", "GOROOT", "GOVERSION")
	data, err := cmd.Output()
	if err != nil {
		return licenseToolchain{}, fmt.Errorf("resolve license toolchain: %w", err)
	}
	var selected licenseToolchain
	if err := json.Unmarshal(data, &selected); err != nil {
		return selected, fmt.Errorf("decode license toolchain: %w", err)
	}
	if !filepath.IsAbs(selected.GOROOT) || !strings.HasPrefix(selected.GOVERSION, "go1.") {
		return selected, fmt.Errorf("selected license toolchain has an empty or invalid GOROOT/GOVERSION")
	}
	return selected, nil
}

func decodeLicensePackages(data []byte) ([]goListPackage, error) {
	dec := json.NewDecoder(strings.NewReader(string(data)))
	var pkgs []goListPackage
	for {
		var pkg goListPackage
		err := dec.Decode(&pkg)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("decode license package inventory: %w", err)
		}
		pkgs = append(pkgs, pkg)
	}
	return pkgs, nil
}

func licensePackageCoverage(pkgs []goListPackage, goroot string) ([]string, map[string]goListModule, error) {
	ignored := make([]string, 0)
	expected := map[string]goListModule{}
	for _, pkg := range pkgs {
		if pkg.Standard {
			// Go list can reuse a module Root for a stdlib vendor alias when
			// both are in the graph. Goroot and the actual source Dir, unlike
			// that Root field, still identify the selected toolchain source.
			rel, err := filepath.Rel(filepath.Join(goroot, "src"), pkg.Dir)
			if pkg.ImportPath == "" || !pkg.Goroot || !filepath.IsAbs(pkg.Dir) || err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
				return nil, nil, fmt.Errorf("standard package %q source %q does not match selected GOROOT %q", pkg.ImportPath, pkg.Dir, goroot)
			}
			ignored = append(ignored, pkg.ImportPath)
			continue
		}
		if pkg.ImportPath == "" || pkg.Module == nil || pkg.Module.Path == "" {
			return nil, nil, fmt.Errorf("nonstandard package %q has no module identity", pkg.ImportPath)
		}
		// Match Libraries(includeTests=false): go list also reports roots
		// containing only _test.go files, which have no assessed source.
		// Never remove a package that has any selected Go or non-Go source.
		if !pkg.hasSourceFiles() {
			if len(pkg.TestGoFiles)+len(pkg.XTestGoFiles) > 0 {
				continue
			}
			return nil, nil, fmt.Errorf("nonstandard package %q has no classified source or test-only root", pkg.ImportPath)
		}
		expected[pkg.ImportPath] = *pkg.Module
	}
	if len(expected) == 0 {
		return nil, nil, fmt.Errorf("license package inventory has empty nonstandard coverage")
	}
	// Upstream ignores prefixes, not exact paths. Prove these inputs cannot
	// suppress a nonstandard package before using that API.
	for pkg := range expected {
		for _, prefix := range ignored {
			if prefix == "" || strings.HasPrefix(pkg, prefix) {
				return nil, nil, fmt.Errorf("stdlib ignore prefix %q overlaps nonstandard package %q", prefix, pkg)
			}
		}
	}
	sort.Strings(ignored)
	return ignored, expected, nil
}

func mapLicenseLibraries(libraries []*licenses.Library, expected map[string]goListModule) (map[string]*License, error) {
	covered := map[string]bool{}
	types := map[string]map[string]bool{}
	names := map[string]string{}
	for _, lib := range libraries {
		if lib == nil {
			return nil, fmt.Errorf("license collector returned a nil library")
		}
		for _, pkg := range lib.Packages {
			mod, ok := expected[pkg]
			if !ok {
				return nil, fmt.Errorf("license collector returned unexpected package %q", pkg)
			}
			resolved := mod
			if mod.Replace != nil {
				resolved = *mod.Replace
			}
			if lib.Version() != strings.TrimSuffix(resolved.Version, "+incompatible") {
				return nil, fmt.Errorf("license library version does not match package %q module %s@%s", pkg, mod.Path, mod.Version)
			}
			covered[pkg] = true
			key := mod.Path + "@" + mod.Version
			if types[key] == nil {
				types[key] = map[string]bool{}
			}
			if len(lib.Licenses) == 0 {
				types[key]["Unknown"] = true
			}
			for _, lic := range lib.Licenses {
				name := strings.TrimSpace(lic.Name)
				if name == "" {
					name = "Unknown"
				}
				types[key][name] = true
			}
			if lib.LicenseFile != "" {
				names[key] = filepath.Base(lib.LicenseFile)
			}
		}
	}
	var missing []string
	for pkg := range expected {
		if !covered[pkg] {
			missing = append(missing, pkg)
		}
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		return nil, fmt.Errorf("partial license inventory: missing packages %s", strings.Join(missing, ", "))
	}
	if len(types) == 0 {
		return nil, fmt.Errorf("license inventory is empty")
	}
	out := map[string]*License{}
	for key, set := range types {
		var values []string
		for name := range set {
			values = append(values, name)
		}
		sort.Strings(values)
		licenseType := strings.Join(values, " AND ")
		out[key] = &License{Name: names[key], Type: licenseType, URL: getLicenseURL(licenseType)}
	}
	return out, nil
}

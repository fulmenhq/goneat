package dependencies

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type goModuleGraphSBOM struct {
	BOMFormat    string                   `json:"bomFormat"`
	SpecVersion  string                   `json:"specVersion"`
	Version      int                      `json:"version"`
	Metadata     goModuleSBOMMetadata     `json:"metadata"`
	Components   []goModuleSBOMComponent  `json:"components"`
	Dependencies []goModuleSBOMDependency `json:"dependencies,omitempty"`
}

type goModuleSBOMMetadata struct {
	Timestamp string                         `json:"timestamp"`
	Component goModuleSBOMComponent          `json:"component"`
	Tools     map[string][]map[string]string `json:"tools,omitempty"`
}

type goModuleSBOMComponent struct {
	BOMRef     string                 `json:"bom-ref,omitempty"`
	Type       string                 `json:"type"`
	Name       string                 `json:"name"`
	Version    string                 `json:"version,omitempty"`
	PURL       string                 `json:"purl,omitempty"`
	Properties []goModuleSBOMProperty `json:"properties,omitempty"`
}

type goModuleSBOMProperty struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type goModuleSBOMDependency struct {
	Ref       string   `json:"ref"`
	DependsOn []string `json:"dependsOn"`
}

type goListModuleGraphEntry struct {
	Path    string `json:"Path"`
	Version string `json:"Version"`
	Main    bool   `json:"Main"`
	Dir     string `json:"Dir"`
	Replace *struct {
		Path    string `json:"Path"`
		Version string `json:"Version"`
		Dir     string `json:"Dir"`
	} `json:"Replace"`
}

func generateGoModuleGraphSBOM(ctx context.Context, target string, outputPath string) (int, error) {
	if _, err := os.Stat(filepath.Join(target, "go.mod")); err != nil {
		return 0, err
	}

	modules, err := listGoModules(ctx, target)
	if err != nil {
		return 0, err
	}
	if len(modules) == 0 {
		return 0, fmt.Errorf("go module graph is empty")
	}

	sort.Slice(modules, func(i, j int) bool {
		if modules[i].Main != modules[j].Main {
			return modules[i].Main
		}
		if modules[i].Path == modules[j].Path {
			return modules[i].Version < modules[j].Version
		}
		return modules[i].Path < modules[j].Path
	})

	mainRef := ""
	components := make([]goModuleSBOMComponent, 0, len(modules))
	depRefs := make([]string, 0, len(modules))
	for _, mod := range modules {
		ref := goModuleBOMRef(mod)
		componentType := "library"
		if mod.Main {
			componentType = "application"
			mainRef = ref
		} else {
			depRefs = append(depRefs, ref)
		}

		component := goModuleSBOMComponent{
			BOMRef:  ref,
			Type:    componentType,
			Name:    mod.Path,
			Version: mod.Version,
			PURL:    goModulePURL(mod),
			Properties: []goModuleSBOMProperty{
				{Name: "goneat:source_type", Value: "go-module-graph"},
				{Name: "goneat:source_path", Value: "go.mod"},
			},
		}
		if mod.Replace != nil {
			component.Properties = append(component.Properties, goModuleSBOMProperty{Name: "goneat:replace_path", Value: mod.Replace.Path})
		}
		components = append(components, component)
	}

	if mainRef == "" {
		mainRef = components[0].BOMRef
	}

	bom := goModuleGraphSBOM{
		BOMFormat:   "CycloneDX",
		SpecVersion: "1.5",
		Version:     1,
		Metadata: goModuleSBOMMetadata{
			Timestamp: time.Now().UTC().Format(time.RFC3339),
			Component: goModuleSBOMComponent{
				BOMRef:  mainRef,
				Type:    "application",
				Name:    modules[0].Path,
				Version: modules[0].Version,
				PURL:    goModulePURL(modules[0]),
			},
			Tools: map[string][]map[string]string{
				"components": {
					{"type": "application", "name": "goneat", "version": "unknown"},
				},
			},
		},
		Components: components,
		Dependencies: []goModuleSBOMDependency{
			{Ref: mainRef, DependsOn: depRefs},
		},
	}

	data, err := json.MarshalIndent(bom, "", "  ")
	if err != nil {
		return 0, fmt.Errorf("marshal go module graph sbom: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(outputPath), 0o750); err != nil {
		return 0, fmt.Errorf("create sbom directory: %w", err)
	}
	if err := os.WriteFile(outputPath, data, 0o600); err != nil {
		return 0, fmt.Errorf("write go module graph sbom: %w", err)
	}

	return len(components), nil
}

func listGoModules(ctx context.Context, target string) ([]goListModuleGraphEntry, error) {
	cmd := exec.CommandContext(ctx, "go", "list", "-m", "-json", "all") // #nosec G204 - go command and args are fixed
	cmd.Dir = target
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg != "" {
			return nil, fmt.Errorf("go list -m failed: %w: %s", err, msg)
		}
		return nil, fmt.Errorf("go list -m failed: %w", err)
	}

	dec := json.NewDecoder(bytes.NewReader(out))
	modules := []goListModuleGraphEntry{}
	for {
		var mod goListModuleGraphEntry
		if err := dec.Decode(&mod); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return nil, fmt.Errorf("decode go list module: %w", err)
		}
		if strings.TrimSpace(mod.Path) == "" {
			continue
		}
		modules = append(modules, mod)
	}
	return modules, nil
}

func goModuleBOMRef(mod goListModuleGraphEntry) string {
	if purl := goModulePURL(mod); purl != "" {
		return purl
	}
	return mod.Path
}

func goModulePURL(mod goListModuleGraphEntry) string {
	path := strings.TrimSpace(mod.Path)
	if path == "" {
		return ""
	}
	if mod.Version == "" {
		return "pkg:golang/" + path
	}
	return "pkg:golang/" + path + "@" + mod.Version
}

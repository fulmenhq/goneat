package cmd

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/bmatcuk/doublestar/v4"
	assetspkg "github.com/fulmenhq/goneat/internal/assets"
	"github.com/fulmenhq/goneat/pkg/logger"
	schemasvc "github.com/fulmenhq/goneat/pkg/schema"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	opreg "github.com/fulmenhq/goneat/internal/ops"
)

const (
	defaultDocsTarget     = "internal/assets/embedded_docs/docs"
	defaultSchemasTarget  = "internal/assets/embedded_schemas"
	defaultExamplesTarget = "internal/assets/embedded_examples"
	defaultAssetsTarget   = "internal/assets/embedded_assets"
)

// assetTypeConfig captures the defaults applied when a manifest selects a preset.
type assetTypeConfig struct {
	Name          string
	Description   string
	Patterns      []string
	Exclude       []string
	DefaultTarget string
}

var builtinAssetTypes = map[string]assetTypeConfig{
	"docs": {
		Name:          "docs",
		Description:   "Documentation files",
		Patterns:      []string{"**/*.md", "**/*.markdown", "**/*.txt"},
		DefaultTarget: defaultDocsTarget,
	},
	"schemas": {
		Name:          "schemas",
		Description:   "Schema definition files",
		Patterns:      []string{"**/*.json", "**/*.yaml", "**/*.yml", "**/*.schema"},
		DefaultTarget: defaultSchemasTarget,
	},
	"examples": {
		Name:          "examples",
		Description:   "Example assets and samples",
		Patterns:      []string{"**/examples/**/*", "**/*.example.*", "**/*.sample.*"},
		DefaultTarget: defaultExamplesTarget,
	},
	"assets": {
		Name:          "assets",
		Description:   "General asset embedding",
		Patterns:      []string{"**/*"},
		Exclude:       []string{"**/.git/**", "**/node_modules/**"},
		DefaultTarget: defaultAssetsTarget,
	},
}

var manifestSchemaPaths = map[string]string{
	"1.0.0": "embedded_schemas/schemas/content/v1.0.0/docs-embed-manifest.json",
	"1.1.0": "embedded_schemas/schemas/content/v1.1.0/embed-manifest.yaml",
}

type embedTopic struct {
	Title        string   `yaml:"title,omitempty"`
	Description  string   `yaml:"description,omitempty"`
	AssetType    string   `yaml:"asset_type,omitempty"`
	ContentTypes []string `yaml:"content_types,omitempty"`
	Include      []string `yaml:"include"`
	Exclude      []string `yaml:"exclude,omitempty"`
	Tags         []string `yaml:"tags,omitempty"`
	Target       string   `yaml:"target,omitempty"`
	Override     bool     `yaml:"override,omitempty"`
}

type embedManifest struct {
	Version         string                `yaml:"version"`
	AssetType       string                `yaml:"asset_type,omitempty"`
	ContentTypes    []string              `yaml:"content_types,omitempty"`
	ExcludePatterns []string              `yaml:"exclude_patterns,omitempty"`
	Target          string                `yaml:"target,omitempty"`
	Override        bool                  `yaml:"override,omitempty"`
	Topics          map[string]embedTopic `yaml:"topics"`
}

type contentItem struct {
	Slug        string   `json:"slug"`
	Path        string   `json:"path"`
	Rel         string   `json:"rel_path"`
	Topic       string   `json:"topic"`
	AssetType   string   `json:"asset_type,omitempty"`
	Target      string   `json:"target,omitempty"`
	Tags        []string `json:"tags,omitempty"`
	Size        int64    `json:"size"`
	Title       string   `json:"title,omitempty"`
	Description string   `json:"description,omitempty"`
	Manifest    string   `json:"manifest,omitempty"`
}

type contentMeta struct {
	root        string
	manifest    string
	target      string
	assetType   string
	version     string
	diagnostics []string
}

type contentPlan struct {
	Items []contentItem
	Meta  contentMeta
}

type contentResolveOptions struct {
	assetTypeOverride       string
	contentTypesOverride    []string
	excludePatternsOverride []string
}

var (
	contentRoot                    string
	contentManifest                string
	contentTarget                  string
	contentJSON                    bool
	contentFormat                  string
	contentPrintPaths              bool
	contentNoDelete                bool
	contentAllManifests            bool
	contentAssetTypeOverride       string
	contentContentTypesOverride    []string
	contentExcludePatternsOverride []string
	contentManifestsValidate       bool
	contentMigrateOutput           string
	contentMigrateForce            bool
	contentDryRun                  bool
	contentInitAssetType           string
	contentInitRoot                string
	contentInitTarget              string
	contentInitTopic               string
	contentInitOutput              string
	contentInitInclude             []string
	contentInitExclude             []string
	contentInitOverwrite           bool
)

var contentCmd = &cobra.Command{
	Use:   "content",
	Short: "Curate and embed documentation content",
}

var contentFindCmd = &cobra.Command{
	Use:   "find",
	Short: "Resolve curated assets from manifest(s)",
	RunE: func(cmd *cobra.Command, _ []string) error {
		plans, err := collectContentPlans(cmd)
		if err != nil {
			return err
		}
		if len(plans) == 0 {
			return errors.New("no embed manifests discovered")
		}
		// Aggregate items and collect diagnostics.
		var (
			allItems          []contentItem
			manifestSummaries []map[string]any
			targets           = make(map[string]struct{})
		)
		for _, plan := range plans {
			allItems = append(allItems, plan.Items...)
			targets[plan.Meta.target] = struct{}{}
			if len(plan.Meta.diagnostics) > 0 {
				for _, msg := range plan.Meta.diagnostics {
					logger.Warn(msg)
				}
			}
			manifestSummaries = append(manifestSummaries, map[string]any{
				"manifest":    relativeToRepo(plan.Meta.manifest),
				"root":        relativeToRepo(plan.Meta.root),
				"target":      plan.Meta.target,
				"asset_type":  plan.Meta.assetType,
				"count":       len(plan.Items),
				"diagnostics": append([]string(nil), plan.Meta.diagnostics...),
			})
		}
		// Sort items by relative path for deterministic output.
		sort.Slice(allItems, func(i, j int) bool { return allItems[i].Rel < allItems[j].Rel })

		if contentPrintPaths {
			multiTarget := len(targets) > 1
			for _, item := range allItems {
				if multiTarget {
					_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%s:%s\n", item.Target, item.Rel)
				} else {
					_, _ = fmt.Fprintln(cmd.OutOrStdout(), item.Rel)
				}
			}
			return nil
		}

		if contentJSON || contentFormat == "json" {
			report := struct {
				Version   string           `json:"version"`
				Root      string           `json:"root"`
				Manifest  string           `json:"manifest,omitempty"`
				Count     int              `json:"count"`
				Items     []contentItem    `json:"items"`
				Manifests []map[string]any `json:"manifests,omitempty"`
			}{
				Version:   "2.0.0",
				Count:     len(allItems),
				Items:     allItems,
				Manifests: manifestSummaries,
			}
			if len(plans) == 1 {
				report.Root = relativeToRepo(plans[0].Meta.root)
				report.Manifest = relativeToRepo(plans[0].Meta.manifest)
			}
			enc := json.NewEncoder(cmd.OutOrStdout())
			enc.SetIndent("", "  ")
			return enc.Encode(report)
		}

		printPrettyItems(cmd.OutOrStdout(), allItems)
		return nil
	},
}

var contentEmbedCmd = &cobra.Command{
	Use:   "embed",
	Short: "Sync curated assets into embedded mirrors",
	RunE: func(cmd *cobra.Command, _ []string) error {
		plans, err := collectContentPlans(cmd)
		if err != nil {
			return err
		}
		if len(plans) == 0 {
			return errors.New("no embed manifests discovered")
		}

		targetFlagSet := flagChanged(cmd, "target")
		aggregated := aggregatePlansByTarget(plans, contentTarget, targetFlagSet)
		targetDiagnostics := gatherDiagnosticsByTarget(plans, contentTarget, targetFlagSet)

		type embedReport struct {
			Target      string   `json:"target"`
			Count       int      `json:"count"`
			DryRun      bool     `json:"dry_run"`
			Diagnostics []string `json:"diagnostics,omitempty"`
		}

		var (
			results []embedReport
			total   int
		)

		for _, group := range aggregated {
			diags := append([]string(nil), targetDiagnostics[group.target]...)
			result := embedReport{Target: group.target, Count: len(group.items), DryRun: contentDryRun, Diagnostics: diags}
			if contentDryRun {
				results = append(results, result)
				if !(contentJSON || contentFormat == "json") {
					_, _ = fmt.Fprintf(cmd.OutOrStdout(), "🔎 DRY-RUN: would embed %d asset(s) to %s\n", len(group.items), group.target)
					if len(diags) > 0 {
						for _, d := range diags {
							_, _ = fmt.Fprintf(cmd.OutOrStdout(), "   - %s\n", d)
						}
					}
				}
				continue
			}

			if err := embedTarget(group.target, group.items, contentNoDelete); err != nil {
				return err
			}
			total += len(group.items)
			results = append(results, result)
			if !(contentJSON || contentFormat == "json") {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "✅ Embedded %d asset(s) to %s\n", len(group.items), group.target)
				if len(diags) > 0 {
					for _, d := range diags {
						_, _ = fmt.Fprintf(cmd.OutOrStdout(), "   - %s\n", d)
					}
				}
			}
		}

		if contentJSON || contentFormat == "json" {
			payload := struct {
				Version string        `json:"version"`
				Targets []embedReport `json:"targets"`
			}{
				Version: "1.0.0",
				Targets: results,
			}
			enc := json.NewEncoder(cmd.OutOrStdout())
			enc.SetIndent("", "  ")
			return enc.Encode(payload)
		}

		if !contentDryRun && total == 0 {
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), "ℹ️ No assets selected for embedding")
		}
		return nil
	},
}

var contentVerifyCmd = &cobra.Command{
	Use:   "verify",
	Short: "Verify embedded mirrors match manifest selection",
	RunE: func(cmd *cobra.Command, _ []string) error {
		plans, err := collectContentPlans(cmd)
		if err != nil {
			return err
		}
		if len(plans) == 0 {
			return errors.New("no embed manifests discovered")
		}
		targetFlagSet := flagChanged(cmd, "target")
		aggregated := aggregatePlansByTarget(plans, contentTarget, targetFlagSet)
		targetDiagnostics := gatherDiagnosticsByTarget(plans, contentTarget, targetFlagSet)

		type targetReport struct {
			Target      string   `json:"target"`
			Missing     []string `json:"missing"`
			Changed     []string `json:"changed"`
			Extra       []string `json:"extra"`
			Diagnostics []string `json:"diagnostics,omitempty"`
		}
		var reports []targetReport
		allOK := true
		for _, group := range aggregated {
			missing, changed, extra, err := verifyTarget(group.target, group.items)
			if err != nil {
				return err
			}
			if len(missing)+len(changed)+len(extra) == 0 {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "✅ Embedded assets verified: %s\n", group.target)
			} else {
				allOK = false
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "❌ Drift detected for %s\n", group.target)
				if len(missing) > 0 {
					_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Missing: %s\n", strings.Join(missing, ", "))
				}
				if len(changed) > 0 {
					_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Changed: %s\n", strings.Join(changed, ", "))
				}
				if len(extra) > 0 {
					_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Extra: %s\n", strings.Join(extra, ", "))
				}
			}
			diags := append([]string(nil), targetDiagnostics[group.target]...)
			reports = append(reports, targetReport{Target: group.target, Missing: missing, Changed: changed, Extra: extra, Diagnostics: diags})
			if len(diags) > 0 && !(contentJSON || contentFormat == "json") {
				for _, d := range diags {
					_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Diagnostic: %s\n", d)
				}
			}
		}

		if contentJSON || contentFormat == "json" {
			out := struct {
				Version string         `json:"version"`
				Targets []targetReport `json:"targets"`
			}{
				Version: "2.0.0",
				Targets: reports,
			}
			enc := json.NewEncoder(cmd.OutOrStdout())
			enc.SetIndent("", "  ")
			if err := enc.Encode(out); err != nil {
				return err
			}
		}

		if !allOK {
			return errors.New("embedded content drift detected")
		}
		return nil
	},
}

var contentManifestsCmd = &cobra.Command{
	Use:   "manifests",
	Short: "List discovered embed manifests",
	RunE: func(cmd *cobra.Command, _ []string) error {
		repoRoot := findRepoRoot()
		opts := contentResolveOptions{
			assetTypeOverride:       contentAssetTypeOverride,
			contentTypesOverride:    contentContentTypesOverride,
			excludePatternsOverride: contentExcludePatternsOverride,
		}

		manifests, err := discoverManifestPaths(contentRoot, contentManifest, contentAllManifests)
		if err != nil {
			return err
		}
		if len(manifests) == 0 {
			return errors.New("no embed manifests discovered")
		}

		rootFlagSet := flagChanged(cmd, "root")
		type summary struct {
			Path        string   `json:"path"`
			Root        string   `json:"root"`
			Version     string   `json:"version"`
			AssetType   string   `json:"asset_type"`
			Target      string   `json:"target"`
			Topics      []string `json:"topics"`
			Items       int      `json:"items"`
			Diagnostics []string `json:"diagnostics,omitempty"`
			Valid       bool     `json:"valid"`
		}

		summaries := make([]summary, 0, len(manifests))
		for _, manifestPath := range manifests {
			manifestAbs, raw, err := loadManifestBytes(manifestPath, repoRoot)
			if err != nil {
				return err
			}
			var versionProbe struct {
				Version string `yaml:"version"`
			}
			_ = yaml.Unmarshal(raw, &versionProbe)
			manifestVersion := versionProbe.Version
			if manifestVersion == "" {
				manifestVersion = "1.0.0"
			}
			if contentManifestsValidate {
				if err := validateManifestBytes(raw, manifestVersion); err != nil {
					return fmt.Errorf("manifest %s failed validation: %w", relativeToRepo(manifestAbs), err)
				}
			}
			effectiveRoot := contentRoot
			if !rootFlagSet {
				effectiveRoot = filepath.Dir(manifestPath)
			}
			items, meta, err := resolveContent(effectiveRoot, manifestPath, opts)
			if err != nil {
				return err
			}
			topics := extractTopics(items)
			summaries = append(summaries, summary{
				Path:        relativeToRepo(manifestAbs),
				Root:        relativeToRepo(meta.root),
				Version:     meta.version,
				AssetType:   meta.assetType,
				Target:      relativeToRepo(meta.target),
				Topics:      topics,
				Items:       len(items),
				Diagnostics: append([]string(nil), meta.diagnostics...),
				Valid:       true,
			})
		}

		if contentJSON || contentFormat == "json" {
			payload := struct {
				Version   string    `json:"version"`
				Count     int       `json:"count"`
				Manifests []summary `json:"manifests"`
			}{
				Version:   "1.0.0",
				Count:     len(summaries),
				Manifests: summaries,
			}
			enc := json.NewEncoder(cmd.OutOrStdout())
			enc.SetIndent("", "  ")
			return enc.Encode(payload)
		}

		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Discovered %d manifest(s):\n", len(summaries))
		for i, s := range summaries {
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%d. %s\n", i+1, s.Path)
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "   version: %s\n", s.Version)
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "   asset_type: %s\n", s.AssetType)
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "   root: %s\n", s.Root)
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "   target: %s\n", s.Target)
			if len(s.Topics) > 0 {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "   topics: %s\n", strings.Join(s.Topics, ", "))
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "   items: %d\n", s.Items)
			if len(s.Diagnostics) > 0 {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "   diagnostics:\n")
				for _, d := range s.Diagnostics {
					_, _ = fmt.Fprintf(cmd.OutOrStdout(), "     - %s\n", d)
				}
			}
		}
		return nil
	},
}

var contentInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Create a new embed manifest",
	RunE: func(cmd *cobra.Command, _ []string) error {
		repoRoot := findRepoRoot()

		assetType := strings.ToLower(strings.TrimSpace(contentInitAssetType))
		if assetType == "" {
			var err error
			assetType, err = promptAssetType(cmd)
			if err != nil {
				return err
			}
			assetType = strings.ToLower(strings.TrimSpace(assetType))
		}
		if assetType == "" {
			assetType = "docs"
		}
		cfg := resolveAssetType(assetType)

		root := contentInitRoot
		if root == "" {
			if contentRoot != "" {
				root = contentRoot
			} else if cfg.Name != "" {
				root = cfg.Name
			} else {
				root = "docs"
			}
		}

		topic := contentInitTopic
		if topic == "" {
			topic = assetType
			if topic == "" {
				topic = "default"
			}
		}

		includes := append([]string(nil), contentInitInclude...)
		if len(includes) == 0 {
			var err error
			includes, err = promptPatternList(cmd, "Include patterns", cfg.Patterns)
			if err != nil {
				return err
			}
		}
		if len(includes) == 0 {
			includes = append(includes, cfg.Patterns...)
		}

		excludes := append([]string(nil), contentInitExclude...)
		if len(excludes) == 0 {
			excludes = append(excludes, cfg.Exclude...)
		}

		if err := ensureIncludesNonEmpty(includes); err != nil {
			return err
		}

		target := contentInitTarget
		if target == "" {
			target = cfg.DefaultTarget
		}

		manifest := embedManifest{
			Version:   "1.1.0",
			AssetType: cfg.Name,
			Target:    target,
			Topics: map[string]embedTopic{
				topic: {
					AssetType: cfg.Name,
					Include:   normalizePatterns(includes),
					Exclude:   normalizePatterns(excludes),
				},
			},
		}

		output := contentInitOutput
		if output == "" {
			output = filepath.Join(root, "embed-manifest.yaml")
		}
		if !filepath.IsAbs(output) && repoRoot != "" {
			output = filepath.Join(repoRoot, output)
		}
		if !contentInitOverwrite {
			if _, err := os.Stat(output); err == nil {
				return fmt.Errorf("output file already exists: %s (use --overwrite)", relativeToRepo(output))
			}
		}
		if err := os.MkdirAll(filepath.Dir(output), 0o755); err != nil {
			return err
		}

		manifestBytes, err := yaml.Marshal(manifest)
		if err != nil {
			return err
		}
		if err := validateManifestBytes(manifestBytes, manifest.Version); err != nil {
			return fmt.Errorf("generated manifest failed validation: %w", err)
		}
		if err := os.WriteFile(output, manifestBytes, 0o644); err != nil {
			return err
		}

		report := struct {
			Path      string   `json:"path"`
			Root      string   `json:"root"`
			Target    string   `json:"target"`
			AssetType string   `json:"asset_type"`
			Topics    []string `json:"topics"`
		}{
			Path:      relativeToRepo(output),
			Root:      root,
			Target:    target,
			AssetType: cfg.Name,
			Topics:    []string{topic},
		}

		if contentJSON || contentFormat == "json" {
			enc := json.NewEncoder(cmd.OutOrStdout())
			enc.SetIndent("", "  ")
			return enc.Encode(report)
		}

		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "✅ Wrote manifest to %s\n", report.Path)
		return nil
	},
}

var contentMigrateManifestCmd = &cobra.Command{
	Use:   "migrate-manifest",
	Short: "Migrate an embed manifest to version 1.1.0",
	RunE: func(cmd *cobra.Command, _ []string) error {
		if contentManifest == "" {
			return errors.New("manifest path is required (use --manifest)")
		}
		repoRoot := findRepoRoot()
		abs, data, err := loadManifestBytes(contentManifest, repoRoot)
		if err != nil {
			return err
		}

		var manifest embedManifest
		if err := yaml.Unmarshal(data, &manifest); err != nil {
			return fmt.Errorf("invalid manifest yaml (%s): %w", contentManifest, err)
		}

		origVersion := manifest.Version
		if origVersion == "" {
			origVersion = "1.0.0"
		}
		switch origVersion {
		case "1.0.0":
			// proceed
		case "1.1.0":
			if !contentMigrateForce {
				return errors.New("manifest already at version 1.1.0 (use --force to rewrite)")
			}
		default:
			return fmt.Errorf("unsupported manifest version %s", origVersion)
		}

		if manifest.AssetType == "" {
			manifest.AssetType = "docs"
		}
		for name, topic := range manifest.Topics {
			if topic.AssetType == "" {
				topic.AssetType = manifest.AssetType
			}
			manifest.Topics[name] = topic
		}
		manifest.Version = "1.1.0"

		marshaled, err := yaml.Marshal(manifest)
		if err != nil {
			return err
		}
		if err := validateManifestBytes(marshaled, manifest.Version); err != nil {
			return fmt.Errorf("migrated manifest failed validation: %w", err)
		}

		outputPath := contentMigrateOutput
		if outputPath == "" {
			outputPath = abs
		} else if !filepath.IsAbs(outputPath) && repoRoot != "" {
			outputPath = filepath.Join(repoRoot, outputPath)
		}
		if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(outputPath, marshaled, 0o644); err != nil {
			return err
		}

		if contentJSON || contentFormat == "json" {
			payload := struct {
				Path       string `json:"path"`
				Version    string `json:"version"`
				Overwrote  bool   `json:"overwrote"`
				SourcePath string `json:"source_path"`
			}{
				Path:       relativeToRepo(outputPath),
				Version:    manifest.Version,
				Overwrote:  outputPath == abs,
				SourcePath: relativeToRepo(abs),
			}
			enc := json.NewEncoder(cmd.OutOrStdout())
			enc.SetIndent("", "  ")
			return enc.Encode(payload)
		}

		message := fmt.Sprintf("✅ Migrated %s to version 1.1.0", relativeToRepo(abs))
		if outputPath != abs {
			message = fmt.Sprintf("%s (written to %s)", message, relativeToRepo(outputPath))
		}
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), message)
		return nil
	},
}

var contentConflictsCmd = &cobra.Command{
	Use:   "conflicts",
	Short: "Report manifest conflicts and overrides",
	RunE: func(cmd *cobra.Command, _ []string) error {
		plans, err := collectContentPlans(cmd)
		if err != nil {
			return err
		}
		if len(plans) == 0 {
			return errors.New("no embed manifests discovered")
		}

		type conflictReport struct {
			Manifest  string `json:"manifest"`
			Target    string `json:"target"`
			AssetType string `json:"asset_type"`
			Root      string `json:"root"`
			Severity  string `json:"severity"`
			Message   string `json:"message"`
		}

		targetFlagSet := flagChanged(cmd, "target")
		var reports []conflictReport
		for _, plan := range plans {
			for _, diag := range plan.Meta.diagnostics {
				severity, text, ok := classifyDiagnostic(diag)
				if !ok {
					continue
				}
				reports = append(reports, conflictReport{
					Manifest:  relativeToRepo(plan.Meta.manifest),
					Target:    relativeToRepo(manifestTargetKey(plan.Meta, contentTarget, targetFlagSet)),
					AssetType: plan.Meta.assetType,
					Root:      relativeToRepo(plan.Meta.root),
					Severity:  severity,
					Message:   text,
				})
			}
		}

		if contentJSON || contentFormat == "json" {
			payload := struct {
				Version   string           `json:"version"`
				Count     int              `json:"count"`
				Conflicts []conflictReport `json:"conflicts"`
			}{
				Version:   "1.0.0",
				Count:     len(reports),
				Conflicts: reports,
			}
			enc := json.NewEncoder(cmd.OutOrStdout())
			enc.SetIndent("", "  ")
			return enc.Encode(payload)
		}

		if len(reports) == 0 {
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), "✅ No conflicts detected")
			return nil
		}

		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Detected %d issue(s):\n", len(reports))
		for i, r := range reports {
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%d. [%s] %s\n", i+1, strings.ToUpper(r.Severity), r.Message)
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "   manifest: %s\n", r.Manifest)
			if r.Target != "" {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "   target: %s\n", r.Target)
			}
			if r.AssetType != "" {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "   asset_type: %s\n", r.AssetType)
			}
		}
		return nil
	},
}

func init() {
	caps := opreg.GetDefaultCapabilities(opreg.GroupNeat, opreg.CategoryValidation)
	if err := opreg.RegisterCommandWithTaxonomy("content", opreg.GroupNeat, opreg.CategoryValidation, caps, contentCmd, "Curate and embed documentation/content assets"); err != nil {
		_ = err
	}

	contentCmd.PersistentFlags().StringVar(&contentRoot, "root", "docs", "Root directory for manifest evaluation")
	contentCmd.PersistentFlags().StringVar(&contentManifest, "manifest", "docs/embed-manifest.yaml", "Manifest file path")
	contentCmd.PersistentFlags().BoolVar(&contentAllManifests, "all-manifests", false, "Discover and process all embed manifests under the root")
	contentCmd.PersistentFlags().StringVar(&contentAssetTypeOverride, "asset-type", "", "Override manifest asset type preset")
	contentCmd.PersistentFlags().StringSliceVar(&contentContentTypesOverride, "content-types", nil, "Override manifest content type patterns (repeatable)")
	contentCmd.PersistentFlags().StringSliceVar(&contentExcludePatternsOverride, "exclude-patterns", nil, "Additional exclude patterns (repeatable)")
	contentCmd.PersistentFlags().BoolVar(&contentJSON, "json", false, "JSON output (alias for --format json)")
	contentCmd.PersistentFlags().StringVar(&contentFormat, "format", "pretty", "Output format: pretty|json")

	contentFindCmd.Flags().BoolVar(&contentPrintPaths, "print-paths", false, "Print only resolved relative paths")
	contentCmd.AddCommand(contentFindCmd)

	contentInitCmd.Flags().StringVar(&contentInitAssetType, "asset-type", "", "Asset type preset (docs|schemas|examples|assets)")
	contentInitCmd.Flags().StringVar(&contentInitRoot, "root", "", "Root directory for the manifest (defaults based on asset type)")
	contentInitCmd.Flags().StringVar(&contentInitTarget, "target", "", "Target directory override for embedding")
	contentInitCmd.Flags().StringVar(&contentInitTopic, "topic", "", "Primary topic name (defaults to asset type)")
	contentInitCmd.Flags().StringVar(&contentInitOutput, "output", "", "Output path for the manifest")
	contentInitCmd.Flags().StringSliceVar(&contentInitInclude, "include", nil, "Include glob pattern (repeatable)")
	contentInitCmd.Flags().StringSliceVar(&contentInitExclude, "exclude", nil, "Exclude glob pattern (repeatable)")
	contentInitCmd.Flags().BoolVar(&contentInitOverwrite, "overwrite", false, "Overwrite existing manifest if present")
	contentCmd.AddCommand(contentInitCmd)

	contentEmbedCmd.Flags().StringVar(&contentTarget, "target", "", "Target directory override (defaults to manifest or asset type preset)")
	contentEmbedCmd.Flags().BoolVar(&contentNoDelete, "no-delete", false, "Do not delete files missing from manifest selection")
	contentEmbedCmd.Flags().BoolVar(&contentDryRun, "dry-run", false, "Preview embedding changes without writing files")
	contentCmd.AddCommand(contentEmbedCmd)

	contentVerifyCmd.Flags().StringVar(&contentTarget, "target", "", "Target directory override used for verification")
	contentCmd.AddCommand(contentVerifyCmd)

	contentManifestsCmd.Flags().BoolVar(&contentManifestsValidate, "validate", false, "Validate manifests before listing output")
	contentCmd.AddCommand(contentManifestsCmd)

	contentMigrateManifestCmd.Flags().StringVar(&contentMigrateOutput, "output", "", "Write migrated manifest to the provided path (defaults to input file)")
	contentMigrateManifestCmd.Flags().BoolVar(&contentMigrateForce, "force", false, "Rewrite even if manifest already at version 1.1.0")
	contentCmd.AddCommand(contentMigrateManifestCmd)

	contentCmd.AddCommand(contentConflictsCmd)

	rootCmd.AddCommand(contentCmd)
}

func collectContentPlans(cmd *cobra.Command) ([]contentPlan, error) {
	repoRoot := findRepoRoot()
	rootFlagSet := flagChanged(cmd, "root")
	opts := contentResolveOptions{
		assetTypeOverride:       contentAssetTypeOverride,
		contentTypesOverride:    contentContentTypesOverride,
		excludePatternsOverride: contentExcludePatternsOverride,
	}

	manifests, err := discoverManifestPaths(contentRoot, contentManifest, contentAllManifests)
	if err != nil {
		return nil, err
	}
	if len(manifests) == 0 {
		// Fall back to explicit manifest if discovery found nothing.
		if contentManifest == "" {
			return nil, errors.New("no embed manifest found")
		}
		manifests = []string{contentManifest}
	}

	plans := make([]contentPlan, 0, len(manifests))
	for _, manifestPath := range manifests {
		effectiveRoot := contentRoot
		if !rootFlagSet {
			effectiveRoot = filepath.Dir(manifestPath)
		}
		items, meta, err := resolveContent(effectiveRoot, manifestPath, opts)
		if err != nil {
			return nil, err
		}
		if repoRoot != "" {
			meta.root = ensureAbsolute(repoRoot, meta.root)
			meta.manifest = ensureAbsolute(repoRoot, meta.manifest)
		}
		plans = append(plans, contentPlan{Items: items, Meta: meta})
	}

	return plans, nil
}

func discoverManifestPaths(root string, manifestFlag string, includeAll bool) ([]string, error) {
	repoRoot := findRepoRoot()
	if repoRoot == "" {
		var err error
		repoRoot, err = filepath.Abs(".")
		if err != nil {
			return nil, err
		}
	}

	seen := make(map[string]struct{})
	ordered := make([]string, 0)

	addIfExists := func(candidate string) {
		if candidate == "" {
			return
		}
		norm, err := normalizeManifestPath(repoRoot, candidate)
		if err != nil {
			return
		}
		if _, ok := seen[norm]; ok {
			return
		}
		seen[norm] = struct{}{}
		ordered = append(ordered, norm)
	}

	if manifestFlag != "" {
		addIfExists(manifestFlag)
		if !includeAll {
			return ordered, nil
		}
	}

	rootCandidate := root
	if rootCandidate == "" {
		rootCandidate = "."
	}

	defaultCandidates := []string{
		filepath.Join(rootCandidate, "embed-manifest.yaml"),
		"embed-manifest.yaml",
		filepath.Join(".goneat", "embed-manifest.yaml"),
		filepath.Join("docs", "embed-manifest.yaml"),
		filepath.Join("schemas", "embed-manifest.yaml"),
		filepath.Join("examples", "embed-manifest.yaml"),
	}
	for _, candidate := range defaultCandidates {
		addIfExists(candidate)
	}

	if includeAll {
		walkRoots := []string{rootCandidate}
		if repoRoot != "" {
			walkRoots = append(walkRoots, repoRoot)
		}
		for _, walkRoot := range walkRoots {
			_ = filepath.WalkDir(walkRoot, func(path string, d fs.DirEntry, err error) error {
				if err != nil {
					return nil
				}
				if d.IsDir() {
					base := filepath.Base(path)
					if base == ".git" || base == "node_modules" || strings.HasPrefix(base, ".") {
						return filepath.SkipDir
					}
					return nil
				}
				if strings.EqualFold(filepath.Base(path), "embed-manifest.yaml") {
					addIfExists(path)
				}
				return nil
			})
		}
	}

	return ordered, nil
}

func resolveContent(root string, manifestPath string, opts contentResolveOptions) ([]contentItem, contentMeta, error) {
	repoRoot := findRepoRoot()

	manifestAbs, manifestData, err := loadManifestBytes(manifestPath, repoRoot)
	if err != nil {
		return nil, contentMeta{}, err
	}

	var versionProbe struct {
		Version string `yaml:"version"`
	}
	_ = yaml.Unmarshal(manifestData, &versionProbe)
	probeVersion := versionProbe.Version
	if probeVersion == "" {
		probeVersion = "1.0.0"
	}
	if err := validateManifestBytes(manifestData, probeVersion); err != nil {
		return nil, contentMeta{}, err
	}

	var manifest embedManifest
	if err := yaml.Unmarshal(manifestData, &manifest); err != nil {
		return nil, contentMeta{}, fmt.Errorf("invalid manifest yaml (%s): %w", manifestPath, err)
	}
	if manifest.Version == "" {
		manifest.Version = "1.0.0"
	}
	if len(manifest.Topics) == 0 {
		return nil, contentMeta{}, fmt.Errorf("manifest %s missing topics", manifestPath)
	}

	rootBase := root
	if rootBase == "" {
		rootBase = filepath.Dir(manifestAbs)
	}
	rootAbs := ensureAbsolute(repoRoot, rootBase)

	assetTypeName := manifest.AssetType
	if opts.assetTypeOverride != "" {
		assetTypeName = opts.assetTypeOverride
	}
	if assetTypeName == "" {
		if manifest.Version == "1.0.0" {
			assetTypeName = "docs"
		} else {
			assetTypeName = "assets"
		}
	}
	manifestType := resolveAssetType(strings.ToLower(assetTypeName))

	manifestPatterns := opts.contentTypesOverride
	if len(manifestPatterns) == 0 {
		manifestPatterns = manifest.ContentTypes
	}
	if len(manifestPatterns) == 0 {
		manifestPatterns = manifestType.Patterns
	}
	if len(manifestPatterns) == 0 {
		manifestPatterns = []string{"**/*"}
	}
	manifestPatterns = normalizePatterns(manifestPatterns)

	globalExclude := append([]string{}, manifestType.Exclude...)
	globalExclude = append(globalExclude, manifest.ExcludePatterns...)
	globalExclude = append(globalExclude, opts.excludePatternsOverride...)
	globalExclude = normalizePatterns(globalExclude)

	manifestTarget := manifest.Target
	if manifestTarget == "" {
		manifestTarget = manifestType.DefaultTarget
	}
	if manifestTarget == "" {
		manifestTarget = defaultAssetsTarget
	}

	type topicConfig struct {
		name        string
		include     []string
		exclude     []string
		tags        []string
		title       string
		description string
		assetType   assetTypeConfig
		patterns    []string
		target      string
		override    bool
	}

	var topicConfigs []topicConfig
	for name, topic := range manifest.Topics {
		if len(topic.Include) == 0 {
			return nil, contentMeta{}, fmt.Errorf("topic %s missing include patterns", name)
		}
		topicTypeName := topic.AssetType
		if topicTypeName == "" {
			topicTypeName = manifestType.Name
		}
		topicType := resolveAssetType(strings.ToLower(topicTypeName))

		topicPatterns := manifestPatterns
		if len(topic.ContentTypes) > 0 {
			topicPatterns = normalizePatterns(topic.ContentTypes)
		}

		topicTarget := topic.Target
		if topicTarget == "" {
			topicTarget = manifestTarget
		}
		if topicTarget == "" {
			topicTarget = defaultAssetsTarget
		}

		topicExclude := append([]string{}, globalExclude...)
		topicExclude = append(topicExclude, topic.Exclude...)
		topicExclude = normalizePatterns(topicExclude)

		topicConfigs = append(topicConfigs, topicConfig{
			name:        name,
			include:     append([]string(nil), topic.Include...),
			exclude:     topicExclude,
			tags:        append([]string(nil), topic.Tags...),
			title:       topic.Title,
			description: topic.Description,
			assetType:   topicType,
			patterns:    topicPatterns,
			target:      topicTarget,
			override:    manifest.Override || topic.Override,
		})
	}

	sort.Slice(topicConfigs, func(i, j int) bool { return topicConfigs[i].name < topicConfigs[j].name })

	itemsByPath := make(map[string]contentItem)
	ownerByPath := make(map[string]struct {
		topic    string
		override bool
	})
	var diagnostics []string

	for _, topicCfg := range topicConfigs {
		for _, includePattern := range topicCfg.include {
			matches, err := expandPattern(rootAbs, includePattern)
			if err != nil {
				diagnostics = append(diagnostics, fmt.Sprintf("warn: include %s/%s: %v", topicCfg.name, includePattern, err))
				continue
			}
			for _, match := range matches {
				info, err := os.Stat(match)
				if err != nil || info.IsDir() {
					continue
				}

				if repoRoot != "" && !ensureWithinRepo(match, repoRoot) {
					diagnostics = append(diagnostics, fmt.Sprintf("warn: skipping %s (outside repository)", match))
					continue
				}

				rel, err := filepath.Rel(rootAbs, match)
				if err != nil {
					diagnostics = append(diagnostics, fmt.Sprintf("warn: unable to compute relative path for %s: %v", match, err))
					continue
				}
				rel = filepath.ToSlash(rel)
				cleanRel := filepath.Clean(rel)
				if strings.HasPrefix(cleanRel, "../") || strings.Contains(cleanRel, "/../") {
					diagnostics = append(diagnostics, fmt.Sprintf("warn: skipping %s (pattern escapes root)", match))
					continue
				}

				if !matchesAny(topicCfg.patterns, rel) {
					continue
				}
				if matchesAny(topicCfg.exclude, rel) {
					continue
				}

				abs := filepath.ToSlash(match)
				key := filepath.Clean(abs)
				if existing, ok := ownerByPath[key]; ok {
					if topicCfg.override {
						diagnostics = append(diagnostics, fmt.Sprintf("override: %s claimed by %s overriding %s", rel, topicCfg.name, existing.topic))
					} else if existing.override {
						diagnostics = append(diagnostics, fmt.Sprintf("skip: %s already claimed by %s", rel, existing.topic))
						continue
					} else {
						diagnostics = append(diagnostics, fmt.Sprintf("conflict: %s claimed by both %s and %s", rel, existing.topic, topicCfg.name))
						continue
					}
				}

				slug := rel
				if topicCfg.assetType.Name == "docs" {
					slug = strings.TrimSuffix(rel, filepath.Ext(rel))
				}

				item := contentItem{
					Slug:        slug,
					Path:        abs,
					Rel:         rel,
					Topic:       topicCfg.name,
					AssetType:   topicCfg.assetType.Name,
					Target:      topicCfg.target,
					Tags:        append([]string(nil), topicCfg.tags...),
					Size:        sizeOf(info),
					Title:       topicCfg.title,
					Description: topicCfg.description,
					Manifest:    filepath.ToSlash(relativeToRepo(manifestAbs)),
				}

				itemsByPath[key] = item
				ownerByPath[key] = struct {
					topic    string
					override bool
				}{topic: topicCfg.name, override: topicCfg.override}
			}
		}
	}

	items := make([]contentItem, 0, len(itemsByPath))
	for _, item := range itemsByPath {
		items = append(items, item)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Rel < items[j].Rel })

	meta := contentMeta{
		root:        rootAbs,
		manifest:    manifestAbs,
		target:      manifestTarget,
		assetType:   manifestType.Name,
		version:     manifest.Version,
		diagnostics: diagnostics,
	}

	return items, meta, nil
}

func embedTarget(target string, items []contentItem, noDelete bool) error {
	repoRoot := findRepoRoot()
	targetAbs := ensureAbsolute(repoRoot, target)
	if err := os.MkdirAll(targetAbs, 0o750); err != nil {
		return err
	}

	want := make(map[string]contentItem, len(items))
	for _, item := range items {
		rel := filepath.Clean(filepath.FromSlash(item.Rel))
		if rel == "." || strings.HasPrefix(rel, "../") || strings.Contains(rel, ".."+string(os.PathSeparator)) {
			logger.Warn(fmt.Sprintf("skipping unsafe relative path %s", item.Rel))
			continue
		}
		want[rel] = item
	}

	if !noDelete {
		_ = filepath.WalkDir(targetAbs, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			if d.IsDir() {
				return nil
			}
			rel, _ := filepath.Rel(targetAbs, path)
			rel = filepath.Clean(rel)
			if _, ok := want[rel]; !ok {
				if remErr := os.Remove(path); remErr != nil {
					return remErr
				}
			}
			return nil
		})
	}

	for rel, item := range want {
		dst := filepath.Join(targetAbs, rel)
		if err := os.MkdirAll(filepath.Dir(dst), 0o750); err != nil {
			return err
		}
		if !ensureWithinRepo(item.Path, repoRoot) {
			logger.Warn(fmt.Sprintf("skipping %s (outside repository)", item.Path))
			continue
		}
		if err := copyFileMode(item.Path, dst, 0o640); err != nil {
			return err
		}
	}

	return nil
}

func verifyTarget(target string, items []contentItem) (missing []string, changed []string, extra []string, err error) {
	repoRoot := findRepoRoot()
	targetAbs := ensureAbsolute(repoRoot, target)

	want := make(map[string]contentItem, len(items))
	order := make([]string, 0, len(items))
	for _, item := range items {
		rel := filepath.Clean(filepath.FromSlash(item.Rel))
		want[rel] = item
		order = append(order, rel)
	}

	for _, rel := range order {
		item := want[rel]
		dst := filepath.Join(targetAbs, rel)
		info, err := os.Stat(dst)
		if errors.Is(err, os.ErrNotExist) {
			missing = append(missing, rel)
			continue
		}
		if err != nil {
			return nil, nil, nil, err
		}
		// #nosec G304 -- paths validated to stay within repo
		srcBytes, sErr := os.ReadFile(item.Path)
		dstBytes, dErr := os.ReadFile(dst)
		if sErr != nil || dErr != nil || !bytes.Equal(srcBytes, dstBytes) || info.Mode()&0o777 != 0o640 {
			changed = append(changed, rel)
		}
	}

	_ = filepath.WalkDir(targetAbs, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			return nil
		}
		rel, _ := filepath.Rel(targetAbs, path)
		rel = filepath.Clean(rel)
		if _, ok := want[rel]; !ok {
			extra = append(extra, filepath.ToSlash(rel))
		}
		return nil
	})

	return
}

func normalizeManifestPath(repoRoot, path string) (string, error) {
	if path == "" {
		return "", fmt.Errorf("empty manifest path")
	}
	candidate := filepath.Clean(path)
	if !filepath.IsAbs(candidate) {
		candidate = filepath.Join(repoRoot, candidate)
	}
	abs, err := filepath.Abs(candidate)
	if err != nil {
		return "", err
	}
	if _, err := os.Stat(abs); err != nil {
		return "", err
	}
	if repoRoot != "" && !ensureWithinRepo(abs, repoRoot) {
		return "", fmt.Errorf("manifest %s outside repository", abs)
	}
	return abs, nil
}

func loadManifestBytes(manifestPath string, repoRoot string) (string, []byte, error) {
	abs, err := normalizeManifestPath(repoRoot, manifestPath)
	if err != nil {
		return "", nil, err
	}
	data, err := os.ReadFile(abs)
	if err != nil {
		return "", nil, err
	}
	return abs, data, nil
}

func expandPattern(baseAbs string, pattern string) ([]string, error) {
	pat := filepath.FromSlash(pattern)
	if filepath.IsAbs(pat) {
		return doublestar.FilepathGlob(filepath.Clean(pat))
	}
	joined := filepath.Join(baseAbs, pat)
	return doublestar.FilepathGlob(joined)
}

func normalizePatterns(patterns []string) []string {
	out := make([]string, 0, len(patterns))
	for _, p := range patterns {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		out = append(out, filepath.ToSlash(p))
	}
	return out
}

func matchesAny(patterns []string, rel string) bool {
	rel = filepath.ToSlash(rel)
	for _, pattern := range patterns {
		if ok, err := doublestar.Match(pattern, rel); err == nil && ok {
			return true
		}
	}
	return false
}

func resolveAssetType(name string) assetTypeConfig {
	if cfg, ok := builtinAssetTypes[name]; ok {
		return cfg
	}
	return assetTypeConfig{
		Name:          name,
		Patterns:      []string{"**/*"},
		DefaultTarget: defaultAssetsTarget,
	}
}

func ensureWithinRepo(path, repoRoot string) bool {
	if repoRoot == "" {
		return true
	}
	cleaned, err := filepath.Abs(path)
	if err != nil {
		return false
	}
	repoAbs, err := filepath.Abs(repoRoot)
	if err != nil {
		return false
	}
	if cleaned == repoAbs {
		return true
	}
	return strings.HasPrefix(cleaned, repoAbs+string(os.PathSeparator))
}

func relativeToRepo(path string) string {
	repoRoot := findRepoRoot()
	if repoRoot == "" {
		return filepath.ToSlash(path)
	}
	rel, err := filepath.Rel(repoRoot, path)
	if err != nil {
		return filepath.ToSlash(path)
	}
	return filepath.ToSlash(rel)
}

func validateManifestBytes(data []byte, version string) error {
	schemaPath, ok := manifestSchemaPaths[version]
	if !ok {
		// fall back to latest schema for forward compatibility
		schemaPath = manifestSchemaPaths["1.1.0"]
	}
	if schemaPath == "" {
		return nil
	}
	schemaBytes, ok := assetspkg.GetSchema(schemaPath)
	if !ok {
		return nil
	}
	res, err := schemasvc.ValidateDataFromBytes(schemaBytes, data)
	if err != nil {
		return fmt.Errorf("manifest validation failed: %w", err)
	}
	if !res.Valid {
		msgs := make([]string, 0, len(res.Errors))
		for _, e := range res.Errors {
			msgs = append(msgs, e.Message)
		}
		return fmt.Errorf("manifest failed schema validation: %s", strings.Join(msgs, "; "))
	}
	return nil
}

func aggregateHeaderWidths(items []contentItem) (slug, topic, asset, target, pathWidth int) {
	slug, topic, asset, target, pathWidth = 4, 5, 5, 6, 4
	for _, it := range items {
		if l := utf8.RuneCountInString(it.Slug); l > slug {
			slug = l
		}
		if l := utf8.RuneCountInString(it.Topic); l > topic {
			topic = l
		}
		if l := utf8.RuneCountInString(it.AssetType); l > asset {
			asset = l
		}
		if l := utf8.RuneCountInString(it.Target); l > target {
			target = l
		}
		if l := utf8.RuneCountInString(it.Path); l > pathWidth {
			pathWidth = l
		}
	}
	return
}

func extractTopics(items []contentItem) []string {
	if len(items) == 0 {
		return nil
	}
	set := make(map[string]struct{})
	for _, it := range items {
		if it.Topic == "" {
			continue
		}
		set[it.Topic] = struct{}{}
	}
	if len(set) == 0 {
		return nil
	}
	res := make([]string, 0, len(set))
	for topic := range set {
		res = append(res, topic)
	}
	sort.Strings(res)
	return res
}

func printPrettyItems(w io.Writer, items []contentItem) {
	if len(items) == 0 {
		_, _ = fmt.Fprintln(w, "(no items)")
		return
	}
	slugW, topicW, assetW, targetW, pathW := aggregateHeaderWidths(items)
	_, _ = fmt.Fprintf(w, "%-*s  %-*s  %-*s  %-*s  %-*s\n", slugW, "SLUG", topicW, "TOPIC", assetW, "ASSET", targetW, "TARGET", pathW, "PATH")
	for _, it := range items {
		_, _ = fmt.Fprintf(w, "%-*s  %-*s  %-*s  %-*s  %-*s\n", slugW, it.Slug, topicW, it.Topic, assetW, it.AssetType, targetW, it.Target, pathW, it.Path)
	}
}

func copyFileMode(src, dst string, mode os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() { _ = in.Close() }()
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, in); err != nil {
		return err
	}
	tmp := dst + ".tmp"
	if err := os.WriteFile(tmp, buf.Bytes(), mode); err != nil {
		return err
	}
	if err := os.Rename(tmp, dst); err != nil {
		return err
	}
	return nil
}

func sizeOf(fi os.FileInfo) int64 {
	if fi == nil {
		return 0
	}
	return fi.Size()
}

func findRepoRoot() string {
	dir, err := os.Getwd()
	if err != nil {
		return ""
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}

func flagChanged(cmd *cobra.Command, name string) bool {
	if cmd == nil {
		return false
	}
	if f := cmd.Flags(); f != nil {
		if flag := f.Lookup(name); flag != nil && flag.Changed {
			return true
		}
	}
	if f := cmd.PersistentFlags(); f != nil {
		if flag := f.Lookup(name); flag != nil && flag.Changed {
			return true
		}
	}
	if f := cmd.InheritedFlags(); f != nil {
		if flag := f.Lookup(name); flag != nil && flag.Changed {
			return true
		}
	}
	return false
}

type targetGroup struct {
	target string
	items  []contentItem
}

func manifestTargetKey(meta contentMeta, targetOverride string, targetOverrideSet bool) string {
	target := meta.target
	if targetOverrideSet && targetOverride != "" {
		target = targetOverride
	}
	if target == "" {
		target = defaultAssetsTarget
	}
	return target
}

func gatherDiagnosticsByTarget(plans []contentPlan, targetOverride string, targetOverrideSet bool) map[string][]string {
	if len(plans) == 0 {
		return nil
	}
	result := make(map[string][]string)
	for _, plan := range plans {
		target := manifestTargetKey(plan.Meta, targetOverride, targetOverrideSet)
		if len(plan.Meta.diagnostics) == 0 {
			continue
		}
		result[target] = append(result[target], plan.Meta.diagnostics...)
	}
	return result
}

func aggregatePlansByTarget(plans []contentPlan, targetOverride string, targetOverrideSet bool) map[string]targetGroup {
	groups := make(map[string]targetGroup)
	for _, plan := range plans {
		target := manifestTargetKey(plan.Meta, targetOverride, targetOverrideSet)
		tg := groups[target]
		tg.target = target
		tg.items = append(tg.items, plan.Items...)
		groups[target] = tg
	}
	for key, tg := range groups {
		sort.Slice(tg.items, func(i, j int) bool { return tg.items[i].Rel < tg.items[j].Rel })
		groups[key] = tg
	}
	return groups
}

func ensureAbsolute(repoRoot, path string) string {
	if filepath.IsAbs(path) {
		return filepath.Clean(path)
	}
	if repoRoot == "" {
		abs, _ := filepath.Abs(path)
		return filepath.Clean(abs)
	}
	return filepath.Clean(filepath.Join(repoRoot, path))
}

func classifyDiagnostic(msg string) (severity, text string, ok bool) {
	parts := strings.SplitN(msg, ":", 2)
	if len(parts) != 2 {
		return "", "", false
	}
	severity = strings.TrimSpace(strings.ToLower(parts[0]))
	text = strings.TrimSpace(parts[1])
	if severity == "" || text == "" {
		return "", "", false
	}
	return severity, text, true
}

func promptAssetType(cmd *cobra.Command) (string, error) {
	reader := bufio.NewReader(cmd.InOrStdin())
	names := make([]string, 0, len(builtinAssetTypes))
	for name := range builtinAssetTypes {
		names = append(names, name)
	}
	sort.Strings(names)
	defaultVal := "docs"
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Asset type [%s]: ", strings.Join(names, ", "))
	line, err := reader.ReadString('\n')
	if err != nil && err != io.EOF {
		return "", err
	}
	line = strings.TrimSpace(line)
	if line == "" {
		line = defaultVal
	}
	return line, nil
}

func promptPatternList(cmd *cobra.Command, question string, defaults []string) ([]string, error) {
	reader := bufio.NewReader(cmd.InOrStdin())
	defaultText := strings.Join(defaults, ", ")
	if defaultText == "" {
		defaultText = "(none)"
	}
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%s [%s]: ", question, defaultText)
	line, err := reader.ReadString('\n')
	if err != nil && err != io.EOF {
		return nil, err
	}
	line = strings.TrimSpace(line)
	if line == "" {
		return append([]string(nil), defaults...), nil
	}
	return splitPatterns(line), nil
}

func splitPatterns(input string) []string {
	parts := strings.Split(input, ",")
	var out []string
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed == "" {
			continue
		}
		out = append(out, trimmed)
	}
	return out
}

func ensureIncludesNonEmpty(includes []string) error {
	for _, pattern := range includes {
		if strings.TrimSpace(pattern) != "" {
			return nil
		}
	}
	return errors.New("at least one include pattern is required")
}

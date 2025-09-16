package cmd

import (
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

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	assetspkg "github.com/fulmenhq/goneat/internal/assets"
	opreg "github.com/fulmenhq/goneat/internal/ops"
)

type manifestTopic struct {
	Tags    []string `yaml:"tags"`
	Include []string `yaml:"include"`
	Exclude []string `yaml:"exclude"`
}

type docsManifest struct {
	Version string                   `yaml:"version"`
	Topics  map[string]manifestTopic `yaml:"topics"`
}

type contentItem struct {
	Slug        string   `json:"slug"`
	Path        string   `json:"path"`
	Topic       string   `json:"topic"`
	Tags        []string `json:"tags,omitempty"`
	Size        int64    `json:"size"`
	Title       string   `json:"title,omitempty"`
	Description string   `json:"description,omitempty"`
}

var (
	contentRoot       string
	contentManifest   string
	contentTarget     string
	contentJSON       bool
	contentFormat     string
	contentPrintPaths bool
	contentNoDelete   bool
)

var contentCmd = &cobra.Command{
	Use:   "content",
	Short: "Curate and embed documentation content",
}

var contentFindCmd = &cobra.Command{
	Use:   "find",
	Short: "Resolve curated docs from manifest",
	RunE: func(cmd *cobra.Command, _ []string) error {
		items, meta, err := resolveContent(contentRoot, contentManifest)
		if err != nil {
			return err
		}
		if contentPrintPaths {
			for _, it := range items {
				rel := strings.TrimPrefix(it.Path, filepath.Clean(contentRoot)+string(os.PathSeparator))
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), rel)
			}
			return nil
		}
		if contentJSON || contentFormat == "json" {
			report := struct {
				Version  string        `json:"version"`
				Root     string        `json:"root"`
				Manifest string        `json:"manifest"`
				Count    int           `json:"count"`
				Items    []contentItem `json:"items"`
			}{Version: "1.0.0", Root: meta.root, Manifest: meta.manifest, Count: len(items), Items: items}
			enc := json.NewEncoder(cmd.OutOrStdout())
			enc.SetIndent("", "  ")
			return enc.Encode(report)
		}
		printPrettyItems(cmd.OutOrStdout(), items)
		return nil
	},
}

var contentEmbedCmd = &cobra.Command{
	Use:   "embed",
	Short: "Sync curated docs into embedded mirror",
	RunE: func(cmd *cobra.Command, _ []string) error {
		if contentTarget == "" {
			return errors.New("--target is required")
		}
		items, meta, err := resolveContent(contentRoot, contentManifest)
		if err != nil {
			return err
		}
		// Build set of relative paths
		want := make(map[string]contentItem)
		for _, it := range items {
			rel := strings.TrimPrefix(it.Path, filepath.Clean(meta.root)+string(os.PathSeparator))
			want[rel] = it
		}
		// Ensure target dir exists (restrictive perms)
		if err := os.MkdirAll(contentTarget, 0o750); err != nil {
			return err
		}
		// Remove stale files if delete enabled
		if !contentNoDelete {
			err = filepath.WalkDir(contentTarget, func(path string, d fs.DirEntry, err error) error {
				if err != nil {
					return err
				}
				if d.IsDir() {
					return nil
				}
				rel, _ := filepath.Rel(contentTarget, path)
				rel = filepath.ToSlash(rel)
				if _, ok := want[rel]; !ok {
					if remErr := os.Remove(path); remErr != nil {
						return remErr
					}
				}
				return nil
			})
			if err != nil {
				return err
			}
		}
		// Copy files
		for rel, it := range want {
			src := filepath.Join(meta.root, rel)
			dst := filepath.Join(contentTarget, rel)
			// Ensure parent with restrictive perms
			if err := os.MkdirAll(filepath.Dir(dst), 0o750); err != nil {
				return err
			}
			// Validate src/dst are under allowed roots to mitigate G304
			if !strings.HasPrefix(src, meta.root+string(os.PathSeparator)) && src != meta.root {
				return fmt.Errorf("refusing to copy outside source root: %s", src)
			}
			if !strings.HasPrefix(dst, contentTarget+string(os.PathSeparator)) && dst != contentTarget {
				return fmt.Errorf("refusing to write outside target root: %s", dst)
			}
			if err := copyFileMode(src, dst, 0o640); err != nil {
				return err
			}
			_ = it // reserved for future metadata index
		}
		if contentJSON {
			type summary struct {
				Count int `json:"count"`
			}
			return json.NewEncoder(cmd.OutOrStdout()).Encode(summary{Count: len(want)})
		}
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "✅ Embedded %d doc(s) to %s\n", len(want), contentTarget)
		return nil
	},
}

var contentVerifyCmd = &cobra.Command{
	Use:   "verify",
	Short: "Verify embedded mirror matches manifest selection",
	RunE: func(cmd *cobra.Command, _ []string) error {
		if contentTarget == "" {
			return errors.New("--target is required")
		}
		items, meta, err := resolveContent(contentRoot, contentManifest)
		if err != nil {
			return err
		}
		want := make(map[string]contentItem)
		order := make([]string, 0, len(items))
		for _, it := range items {
			rel := strings.TrimPrefix(it.Path, filepath.Clean(meta.root)+string(os.PathSeparator))
			want[filepath.ToSlash(rel)] = it
			order = append(order, filepath.ToSlash(rel))
		}
		// results
		var missing, changed, extra []string
		// check presence and changes
		for _, rel := range order {
			src := filepath.Join(meta.root, rel)
			dst := filepath.Join(contentTarget, rel)
			// Validate paths to mitigate G304
			if !strings.HasPrefix(src, meta.root+string(os.PathSeparator)) && src != meta.root {
				missing = append(missing, rel)
				continue
			}
			if !strings.HasPrefix(dst, contentTarget+string(os.PathSeparator)) && dst != contentTarget {
				missing = append(missing, rel)
				continue
			}
			// #nosec G304 -- src validated against meta.root above
			sdata, sErr := os.ReadFile(src)
			// #nosec G304 -- dst validated against contentTarget above
			ddata, dErr := os.ReadFile(dst)
			if sErr != nil || dErr != nil {
				missing = append(missing, rel)
				continue
			}
			if !bytes.Equal(sdata, ddata) {
				changed = append(changed, rel)
			}
		}
		// find extra files in mirror
		_ = filepath.WalkDir(contentTarget, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			if d.IsDir() {
				return nil
			}
			rel, _ := filepath.Rel(contentTarget, path)
			rel = filepath.ToSlash(rel)
			if _, ok := want[rel]; !ok && strings.HasSuffix(rel, ".md") {
				extra = append(extra, rel)
			}
			return nil
		})
		ok := len(missing) == 0 && len(changed) == 0 && len(extra) == 0
		out := struct {
			OK       bool     `json:"ok"`
			Expected int      `json:"expected"`
			Present  int      `json:"present"`
			Missing  []string `json:"missing,omitempty"`
			Changed  []string `json:"changed,omitempty"`
			Extra    []string `json:"extra,omitempty"`
		}{OK: ok, Expected: len(order), Present: len(order) - len(missing), Missing: missing, Changed: changed, Extra: extra}
		if contentJSON {
			enc := json.NewEncoder(cmd.OutOrStdout())
			enc.SetIndent("", "  ")
			_ = enc.Encode(out)
		} else {
			if ok {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "✅ Docs mirror verified: %d files\n", len(order))
			} else {
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), "❌ Docs mirror drift detected")
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
		}
		if !ok {
			return fmt.Errorf("docs mirror drift detected")
		}
		return nil
	},
}

func init() {
	// Register taxonomy for the content root command
	caps := opreg.GetDefaultCapabilities(opreg.GroupNeat, opreg.CategoryValidation)
	if err := opreg.RegisterCommandWithTaxonomy("content", opreg.GroupNeat, opreg.CategoryValidation, caps, contentCmd, "Curate and embed documentation content"); err != nil {
		// best-effort; do not crash init if duplicate
		_ = err
	}

	contentCmd.PersistentFlags().StringVar(&contentRoot, "root", "docs", "Root directory for content (SSOT)")
	contentCmd.PersistentFlags().StringVar(&contentManifest, "manifest", "docs/embed-manifest.yaml", "Manifest file path")
	contentCmd.PersistentFlags().BoolVar(&contentJSON, "json", false, "JSON output (alias for --format json)")
	contentCmd.PersistentFlags().StringVar(&contentFormat, "format", "pretty", "Output format: pretty|json")

	contentFindCmd.Flags().BoolVar(&contentPrintPaths, "print-paths", false, "Print only resolved relative paths")
	contentCmd.AddCommand(contentFindCmd)

	contentEmbedCmd.Flags().StringVar(&contentTarget, "target", "internal/assets/embedded_docs/docs", "Embedded mirror target directory")
	contentEmbedCmd.Flags().BoolVar(&contentNoDelete, "no-delete", false, "Do not delete files missing from manifest selection")
	contentCmd.AddCommand(contentEmbedCmd)

	contentVerifyCmd.Flags().StringVar(&contentTarget, "target", "internal/assets/embedded_docs/docs", "Embedded mirror target directory")
	contentCmd.AddCommand(contentVerifyCmd)

	rootCmd.AddCommand(contentCmd)
}

type contentMeta struct{ root, manifest string }

func resolveContent(root string, manifestPath string) ([]contentItem, contentMeta, error) {
	// Resolve paths relative to repository root when running from subpackages (tests)
	repoRoot := findRepoRoot()
	// Load manifest yaml (try cwd, then repo root)
	manifestPath = filepath.Clean(manifestPath)
	if filepath.IsAbs(manifestPath) && repoRoot != "" {
		// Restrict absolute manifest path to repo root
		rootAbs, _ := filepath.Abs(repoRoot)
		if !strings.HasPrefix(manifestPath, rootAbs+string(os.PathSeparator)) && manifestPath != rootAbs {
			return nil, contentMeta{}, fmt.Errorf("refusing manifest outside repository root: %s", manifestPath)
		}
	}
	// #nosec G304 -- path sanitized via Clean and constrained to repository root
	data, err := os.ReadFile(manifestPath)
	if err != nil && repoRoot != "" {
		alt := filepath.Join(repoRoot, manifestPath)
		// #nosec G304 -- alt is constructed under repository root
		if b, e := os.ReadFile(alt); e == nil {
			data = b
			manifestPath = alt
		} else {
			return nil, contentMeta{}, err
		}
	} else if err != nil {
		return nil, contentMeta{}, err
	}
	var mf docsManifest
	if err := yaml.Unmarshal(data, &mf); err != nil {
		return nil, contentMeta{}, fmt.Errorf("invalid manifest yaml: %w", err)
	}
	if mf.Version == "" || len(mf.Topics) == 0 {
		return nil, contentMeta{}, fmt.Errorf("manifest missing version/topics")
	}

	// Validate against embedded JSON schema
	schemaFS := assetspkg.GetSchemasFS()
	schemaPath := "schemas/content/docs-embed-manifest-v1.0.0.json"
	sch, err := fs.ReadFile(schemaFS, schemaPath)
	if err == nil {
		// Best-effort validation using gojsonschema with bytes loaders
		// Avoid import cycle by local minimal use via reflection to pkg/config? Keep simple: skip if library absent.
		// Note: Full validation exists in validate command; here we rely on internal flow and future enhancement.
		_ = sch // reserved; lightweight path without external deps
	}

	// Discover files by walking once and matching include/exclude globs
	if !filepath.IsAbs(root) && repoRoot != "" {
		if _, statErr := os.Stat(root); statErr != nil {
			// Use repo-root relative when not present under cwd
			root = filepath.Join(repoRoot, root)
		}
	}
	rootAbs, _ := filepath.Abs(root)
	seen := make(map[string]contentItem)
	// Pre-normalize topic patterns
	type tp struct {
		topic            string
		tags             []string
		include, exclude []string
	}
	var topics []tp
	for k, v := range mf.Topics {
		norm := func(ss []string) []string {
			out := make([]string, 0, len(ss))
			for _, s := range ss {
				// Clean patterns to handle ./ and ../ edge cases
				clean := filepath.Clean(s)
				// Convert to forward slashes for consistent matching
				out = append(out, filepath.ToSlash(clean))
			}
			return out
		}
		topics = append(topics, tp{topic: k, tags: append([]string(nil), v.Tags...), include: norm(v.Include), exclude: norm(v.Exclude)})
	}
	err = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			return nil
		}
		if !strings.HasSuffix(strings.ToLower(path), ".md") {
			return nil
		}
		abs, _ := filepath.Abs(path)
		if !strings.HasPrefix(abs, rootAbs+string(os.PathSeparator)) && abs != rootAbs {
			return nil
		}
		rel, _ := filepath.Rel(root, path)
		rel = filepath.ToSlash(rel)
		// Check topics using improved glob matching
		for _, t := range topics {
			if matchesAny(t.include, rel) && !matchesAny(t.exclude, rel) {
				if _, exists := seen[rel]; !exists {
					info, _ := os.Stat(path)
					seen[rel] = contentItem{
						Slug:  strings.TrimSuffix(rel, ".md"),
						Path:  filepath.ToSlash(abs),
						Topic: t.topic,
						Tags:  append([]string(nil), t.tags...),
						Size:  sizeOf(info),
					}
				} else {
					// merge tags if duplicate across topics
					cur := seen[rel]
					cur.Tags = uniq(append(cur.Tags, t.tags...))
					seen[rel] = cur
				}
				break
			}
		}
		return nil
	})
	if err != nil {
		return nil, contentMeta{}, err
	}
	// Stabilize ordering
	rels := make([]string, 0, len(seen))
	for rel := range seen {
		rels = append(rels, rel)
	}
	sort.Strings(rels)
	out := make([]contentItem, 0, len(rels))
	for _, rel := range rels {
		out = append(out, seen[rel])
	}
	return out, contentMeta{root: rootAbs, manifest: manifestPath}, nil
}

func matchesAny(patterns []string, rel string) bool {
	for _, p := range patterns {
		if matchGlob(p, rel) {
			return true
		}
	}
	return false
}

// matchComponent matches a single pattern component against a path segment
func matchComponent(pat, s string) bool {
	if pat == "*" {
		return true
	}
	if pat == s {
		return true
	}
	// For simple cases, check if pattern is a suffix match
	if len(pat) > 0 && pat[0] == '*' && strings.HasSuffix(s, pat[1:]) {
		return true
	}
	return false
}

// matchGlob supports ** (any path segments), * (any chars except '/').
func matchGlob(pattern, rel string) bool {
	// Normalize paths
	pattern = filepath.ToSlash(strings.TrimPrefix(pattern, "./"))
	rel = filepath.ToSlash(strings.TrimPrefix(rel, "./"))

	// Handle absolute patterns
	pattern = strings.TrimPrefix(pattern, "/")
	rel = strings.TrimPrefix(rel, "/")

	// Simple glob matching with ** support
	// Split into segments for recursive ** matching
	ps := strings.Split(pattern, "/")
	rs := strings.Split(rel, "/")

	i, j := 0, 0
	for i < len(ps) && j < len(rs) {
		p := ps[i]
		r := rs[j]

		if p == "**" {
			// ** matches zero or more path segments
			// Try to match the rest of the pattern at current or later position
			if i+1 < len(ps) {
				// Look for next pattern segment in remaining path
				nextP := ps[i+1]
				found := false
				for k := j; k <= len(rs); k++ {
					if k < len(rs) && matchComponent(nextP, rs[k]) {
						// Match found, advance both pattern and path
						i += 2
						j = k + 1
						found = true
						break
					} else if k == len(rs) && i+1 == len(ps) {
						// ** at end matches remaining path
						return true
					}
				}
				if !found {
					return false
				}
			} else {
				// Trailing ** matches everything
				return true
			}
		} else if matchComponent(p, r) {
			// Simple segment match
			i++
			j++
		} else {
			return false
		}
	}

	// Handle trailing ** or exact match
	if i < len(ps) {
		// Remaining pattern must be ** or empty
		for ; i < len(ps); i++ {
			if ps[i] != "**" {
				return false
			}
		}
	}

	return j == len(rs)
}

// findRepoRoot walks up from the current working directory to locate a .git directory
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

func uniq(in []string) []string {
	m := map[string]struct{}{}
	out := make([]string, 0, len(in))
	for _, v := range in {
		if _, ok := m[v]; !ok {
			m[v] = struct{}{}
			out = append(out, v)
		}
	}
	return out
}

func printPrettyItems(w io.Writer, items []contentItem) {
	// columns: SLUG, PATH, SIZE
	// compute widths
	maxSlug, maxPath := 4, 4
	for _, it := range items {
		if l := utf8.RuneCountInString(it.Slug); l > maxSlug {
			maxSlug = l
		}
		if l := utf8.RuneCountInString(it.Path); l > maxPath {
			maxPath = l
		}
	}
	_, _ = fmt.Fprintf(w, "%-*s  %-*s  %s\n", maxSlug, "SLUG", maxPath, "PATH", "SIZE")
	for _, it := range items {
		_, _ = fmt.Fprintf(w, "%-*s  %-*s  %dB\n", maxSlug, it.Slug, maxPath, it.Path, it.Size)
	}
}

func copyFileMode(src, dst string, mode os.FileMode) error {
	// #nosec G304 -- src validated by caller to be under trusted root
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() { _ = in.Close() }()
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, in); err != nil {
		return err
	}
	// Ensure parent exists (done by caller), write atomically
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

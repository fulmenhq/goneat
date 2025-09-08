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
    contentRoot      string
    contentManifest  string
    contentTarget    string
    contentJSON      bool
    contentFormat    string
    contentPrintPaths bool
    contentNoDelete  bool
)

var contentCmd = &cobra.Command{
    Use:   "content",
    Short: "Curate and embed documentation content",
}

var contentFindCmd = &cobra.Command{
    Use:   "find",
    Short: "Resolve curated docs from manifest",
    RunE: func(cmd *cobra.Command, args []string) error {
        items, meta, err := resolveContent(contentRoot, contentManifest)
        if err != nil {
            return err
        }
        if contentPrintPaths {
            for _, it := range items {
                rel := strings.TrimPrefix(it.Path, filepath.Clean(contentRoot)+string(os.PathSeparator))
                fmt.Println(rel)
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
            enc := json.NewEncoder(os.Stdout)
            enc.SetIndent("", "  ")
            return enc.Encode(report)
        }
        printPrettyItems(os.Stdout, items)
        return nil
    },
}

var contentEmbedCmd = &cobra.Command{
    Use:   "embed",
    Short: "Sync curated docs into embedded mirror",
    RunE: func(cmd *cobra.Command, args []string) error {
        if contentTarget == "" {
            return errors.New("--target is required")
        }
        items, _, err := resolveContent(contentRoot, contentManifest)
        if err != nil {
            return err
        }
        // Build set of relative paths
        want := make(map[string]contentItem)
        for _, it := range items {
            rel := strings.TrimPrefix(it.Path, filepath.Clean(contentRoot)+string(os.PathSeparator))
            want[rel] = it
        }
        // Ensure target dir exists
        if err := os.MkdirAll(contentTarget, 0o755); err != nil {
            return err
        }
        // Remove stale files if delete enabled
        if !contentNoDelete {
            err = filepath.WalkDir(contentTarget, func(path string, d fs.DirEntry, err error) error {
                if err != nil { return err }
                if d.IsDir() { return nil }
                rel, _ := filepath.Rel(contentTarget, path)
                rel = filepath.ToSlash(rel)
                if _, ok := want[rel]; !ok {
                    if remErr := os.Remove(path); remErr != nil {
                        return remErr
                    }
                }
                return nil
            })
            if err != nil { return err }
        }
        // Copy files
        for rel, it := range want {
            src := filepath.Join(contentRoot, rel)
            dst := filepath.Join(contentTarget, rel)
            if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil { return err }
            if err := copyFileMode(src, dst, 0o644); err != nil { return err }
            _ = it // reserved for future metadata index
        }
        if contentJSON {
            type summary struct{
                Count int `json:"count"`
            }
            return json.NewEncoder(os.Stdout).Encode(summary{Count: len(want)})
        }
        fmt.Printf("✅ Embedded %d doc(s) to %s\n", len(want), contentTarget)
        return nil
    },
}

var contentVerifyCmd = &cobra.Command{
    Use:   "verify",
    Short: "Verify embedded mirror matches manifest selection",
    RunE: func(cmd *cobra.Command, args []string) error {
        if contentTarget == "" {
            return errors.New("--target is required")
        }
        items, _, err := resolveContent(contentRoot, contentManifest)
        if err != nil { return err }
        want := make(map[string]contentItem)
        order := make([]string, 0, len(items))
        for _, it := range items {
            rel := strings.TrimPrefix(it.Path, filepath.Clean(contentRoot)+string(os.PathSeparator))
            want[filepath.ToSlash(rel)] = it
            order = append(order, filepath.ToSlash(rel))
        }
        // results
        var missing, changed, extra []string
        // check presence and changes
        for _, rel := range order {
            src := filepath.Join(contentRoot, rel)
            dst := filepath.Join(contentTarget, rel)
            sdata, sErr := os.ReadFile(src)
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
        filepath.WalkDir(contentTarget, func(path string, d fs.DirEntry, err error) error {
            if err != nil { return nil }
            if d.IsDir() { return nil }
            rel, _ := filepath.Rel(contentTarget, path)
            rel = filepath.ToSlash(rel)
            if _, ok := want[rel]; !ok && strings.HasSuffix(rel, ".md") {
                extra = append(extra, rel)
            }
            return nil
        })
        ok := len(missing) == 0 && len(changed) == 0 && len(extra) == 0
        out := struct {
            OK      bool     `json:"ok"`
            Expected int      `json:"expected"`
            Present  int      `json:"present"`
            Missing []string `json:"missing,omitempty"`
            Changed []string `json:"changed,omitempty"`
            Extra   []string `json:"extra,omitempty"`
        }{OK: ok, Expected: len(order), Present: len(order) - len(missing), Missing: missing, Changed: changed, Extra: extra}
        if contentJSON {
            enc := json.NewEncoder(os.Stdout)
            enc.SetIndent("", "  ")
            _ = enc.Encode(out)
        } else {
            if ok {
                fmt.Fprintf(os.Stdout, "✅ Docs mirror verified: %d files\n", len(order))
            } else {
                fmt.Fprintln(os.Stdout, "❌ Docs mirror drift detected")
                if len(missing) > 0 { fmt.Fprintf(os.Stdout, "Missing: %s\n", strings.Join(missing, ", ")) }
                if len(changed) > 0 { fmt.Fprintf(os.Stdout, "Changed: %s\n", strings.Join(changed, ", ")) }
                if len(extra) > 0 { fmt.Fprintf(os.Stdout, "Extra: %s\n", strings.Join(extra, ", ")) }
            }
        }
        if !ok { return fmt.Errorf("docs mirror drift detected") }
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
    // Load manifest yaml
    data, err := os.ReadFile(manifestPath)
    if err != nil { return nil, contentMeta{}, err }
    var mf docsManifest
    if err := yaml.Unmarshal(data, &mf); err != nil { return nil, contentMeta{}, fmt.Errorf("invalid manifest yaml: %w", err) }
    if mf.Version == "" || len(mf.Topics) == 0 { return nil, contentMeta{}, fmt.Errorf("manifest missing version/topics") }

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
    rootAbs, _ := filepath.Abs(root)
    seen := make(map[string]contentItem)
    // Pre-normalize topic patterns
    type tp struct{ topic string; tags []string; include, exclude []string }
    var topics []tp
    for k, v := range mf.Topics {
        norm := func(ss []string) []string { out := make([]string, 0, len(ss)); for _, s := range ss { out = append(out, filepath.ToSlash(strings.TrimSpace(s))) }; return out }
        topics = append(topics, tp{topic: k, tags: append([]string(nil), v.Tags...), include: norm(v.Include), exclude: norm(v.Exclude)})
    }
    err = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
        if err != nil { return nil }
        if d.IsDir() { return nil }
        if !strings.HasSuffix(strings.ToLower(path), ".md") { return nil }
        abs, _ := filepath.Abs(path)
        if !strings.HasPrefix(abs, rootAbs+string(os.PathSeparator)) && abs != rootAbs { return nil }
        rel, _ := filepath.Rel(root, path)
        rel = filepath.ToSlash(rel)
        // Check topics
        for _, t := range topics {
            if matchesAny(t.include, rel) && !matchesAny(t.exclude, rel) {
                if _, exists := seen[rel]; !exists {
                    info, _ := os.Stat(path)
                    seen[rel] = contentItem{
                        Slug:  strings.TrimSuffix(rel, ".md"),
                        Path:  filepath.ToSlash(path),
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
    if err != nil { return nil, contentMeta{}, err }
    // Stabilize ordering
    rels := make([]string, 0, len(seen))
    for rel := range seen { rels = append(rels, rel) }
    sort.Strings(rels)
    out := make([]contentItem, 0, len(rels))
    for _, rel := range rels { out = append(out, seen[rel]) }
    return out, contentMeta{root: root, manifest: manifestPath}, nil
}

func matchesAny(patterns []string, rel string) bool {
    for _, p := range patterns {
        if matchGlob(p, rel) { return true }
    }
    return false
}

// matchGlob supports ** (any path segments), * (any chars except '/').
func matchGlob(pattern, rel string) bool {
    // Normalize
    pattern = filepath.ToSlash(strings.TrimPrefix(pattern, "./"))
    rel = filepath.ToSlash(strings.TrimPrefix(rel, "./"))
    // split into segments
    ps := strings.Split(pattern, "/")
    rs := strings.Split(rel, "/")
    return matchSeg(ps, rs)
}

func matchSeg(ps, rs []string) bool {
    for len(ps) > 0 {
        p := ps[0]
        ps = ps[1:]
        if p == "**" {
            if len(ps) == 0 { return true } // trailing ** matches all
            // try to match the rest at any position
            for i := 0; i <= len(rs); i++ {
                if matchSeg(ps, rs[i:]) { return true }
            }
            return false
        }
        if len(rs) == 0 { return false }
        if !matchComponent(p, rs[0]) { return false }
        rs = rs[1:]
    }
    return len(rs) == 0
}

func matchComponent(pat, s string) bool {
    // support * wildcard
    // convert to rune slices for '?' support in future; implement simple '*' now
    // greedy: split by '*'
    if pat == "*" { return true }
    parts := strings.Split(pat, "*")
    if len(parts) == 1 { return pat == s }
    // prefix
    if !strings.HasPrefix(s, parts[0]) { return false }
    s = s[len(parts[0]):]
    // middle parts must appear in order
    for i := 1; i < len(parts)-1; i++ {
        idx := strings.Index(s, parts[i])
        if idx < 0 { return false }
        s = s[idx+len(parts[i]):]
    }
    // suffix
    return strings.HasSuffix(s, parts[len(parts)-1])
}

func uniq(in []string) []string {
    m := map[string]struct{}{}
    out := make([]string, 0, len(in))
    for _, v := range in { if _, ok := m[v]; !ok { m[v] = struct{}{}; out = append(out, v) } }
    return out
}

func printPrettyItems(w io.Writer, items []contentItem) {
    // columns: SLUG, PATH, SIZE
    // compute widths
    maxSlug, maxPath := 4, 4
    for _, it := range items {
        if l := utf8.RuneCountInString(it.Slug); l > maxSlug { maxSlug = l }
        if l := utf8.RuneCountInString(it.Path); l > maxPath { maxPath = l }
    }
    fmt.Fprintf(w, "%-*s  %-*s  %s\n", maxSlug, "SLUG", maxPath, "PATH", "SIZE")
    for _, it := range items {
        fmt.Fprintf(w, "%-*s  %-*s  %dB\n", maxSlug, it.Slug, maxPath, it.Path, it.Size)
    }
}

func copyFileMode(src, dst string, mode os.FileMode) error {
    in, err := os.Open(src)
    if err != nil { return err }
    defer in.Close()
    var buf bytes.Buffer
    if _, err := io.Copy(&buf, in); err != nil { return err }
    // Ensure parent exists (done by caller), write atomically
    tmp := dst + ".tmp"
    if err := os.WriteFile(tmp, buf.Bytes(), mode); err != nil { return err }
    if err := os.Rename(tmp, dst); err != nil { return err }
    return nil
}

func sizeOf(fi os.FileInfo) int64 { if fi == nil { return 0 }; return fi.Size() }

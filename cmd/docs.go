package cmd

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	assetspkg "github.com/fulmenhq/goneat/internal/assets"
	opreg "github.com/fulmenhq/goneat/internal/ops"
)

var (
	docsOutputFormat string
	docsTypeFilter   string
	docsJSON         bool
	docsOpen         bool
)

type docListItem struct {
	Slug        string   `json:"slug"`
	Path        string   `json:"path"`
	Title       string   `json:"title,omitempty"`
	Description string   `json:"description,omitempty"`
	Tags        []string `json:"tags,omitempty"`
	Size        int64    `json:"size"`
}

var docsCmd = &cobra.Command{
	Use:   "docs",
	Short: "Read-only access to embedded docs",
}

var docsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List embedded docs",
	Long:  "List embedded docs in a tree (pretty) or as JSON. Slugs are the path-like entries shown in the tree; use them with 'goneat docs show <slug>'.",
	RunE: func(cmd *cobra.Command, _ []string) error {
		dfs := assetspkg.GetDocsFS()
		var items []docListItem
		err := fs.WalkDir(dfs, "docs", func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				return nil
			}
			if !strings.HasSuffix(p, ".md") {
				return nil
			}
			rel := strings.TrimPrefix(p, "docs/")
			slug := strings.TrimSuffix(rel, ".md")
			if docsTypeFilter != "" && !matchesTypeFilter(slug, docsTypeFilter) {
				return nil
			}
			info, _ := fs.Stat(dfs, p)
			items = append(items, docListItem{Slug: slug, Path: "docs/" + rel, Size: sizeOfFS(info)})
			return nil
		})
		if err != nil {
			return err
		}
		sort.Slice(items, func(i, j int) bool { return items[i].Slug < items[j].Slug })
		if docsOutputFormat == "json" || docsJSON {
			enc := json.NewEncoder(cmd.OutOrStdout())
			enc.SetIndent("", "  ")
			return enc.Encode(items)
		}
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), "Hint: use 'goneat docs show <slug>', e.g., 'user-guide/commands/format'.")
		printDocsTree(cmd.OutOrStdout(), items)
		return nil
	},
}

var docsShowCmd = &cobra.Command{
	Use:   "show <slug>",
	Short: "Show a document by slug",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		slug := args[0]
		dfs := assetspkg.GetDocsFS()
		p := filepath.ToSlash("docs/" + slug + ".md")
		f, err := dfs.Open(p)
		if err != nil {
			return fmt.Errorf("doc not found: %s", slug)
		}
		defer func() { _ = f.Close() }()
		if docsOutputFormat == "json" {
			// minimal envelope; frontmatter enrichment to be added later
			b := new(strings.Builder)
			sc := bufio.NewScanner(f)
			for sc.Scan() {
				b.WriteString(sc.Text())
				b.WriteByte('\n')
			}
			out := map[string]any{
				"slug":    slug,
				"path":    p,
				"content": b.String(),
			}
			enc := json.NewEncoder(cmd.OutOrStdout())
			enc.SetIndent("", "  ")
			return enc.Encode(out)
		} else if docsOutputFormat == "html" || docsOpen {
			// Lightweight HTML wrapper with escaped content (no external deps)
			b := new(strings.Builder)
			sc := bufio.NewScanner(f)
			for sc.Scan() {
				b.WriteString(htmlEscape(sc.Text()))
				b.WriteByte('\n')
			}
			html := fmt.Sprintf("<!doctype html><html><head><meta charset=\"utf-8\"><title>%s</title></head><body><pre class=\"markdown\">%s</pre></body></html>", slug, b.String())
			if docsOpen {
				tmp, err := os.CreateTemp("", "goneat-doc-*.html")
				if err != nil {
					return err
				}
				defer func() { _ = tmp.Close() }()
				if _, err := tmp.WriteString(html); err != nil {
					return err
				}
				return openBrowser(tmp.Name())
			}
			_, err := fmt.Fprint(cmd.OutOrStdout(), html)
			if err != nil {
				return err
			}
			return sc.Err()
		}
		// passthrough markdown
		sc := bufio.NewScanner(f)
		for sc.Scan() {
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), sc.Text())
		}
		return sc.Err()
	},
}

func init() {
	caps := opreg.GetDefaultCapabilities(opreg.GroupSupport, opreg.CategoryInformation)
	if err := opreg.RegisterCommandWithTaxonomy("docs", opreg.GroupSupport, opreg.CategoryInformation, caps, docsCmd, "Read-only documentation access"); err != nil {
		_ = err
	}
	docsListCmd.Flags().StringVar(&docsOutputFormat, "format", "json", "Output format: json|pretty")
	docsListCmd.Flags().BoolVar(&docsJSON, "json", false, "JSON output (alias for --format json)")
	docsListCmd.Flags().StringVar(&docsTypeFilter, "type", "", "Filter by type: commands|tutorials|workflows|configuration")
	docsShowCmd.Flags().StringVar(&docsOutputFormat, "format", "json", "Output format: json|markdown|html")
	docsShowCmd.Flags().BoolVar(&docsOpen, "open", false, "Open in default browser (implies --format html)")
	docsHelpCmd.Flags().StringVar(&docsOutputFormat, "format", "markdown", "Output format: json|markdown|html")
	docsCmd.AddCommand(docsListCmd, docsShowCmd, docsHelpCmd)
	rootCmd.AddCommand(docsCmd)
}

func sizeOfFS(fi fs.FileInfo) int64 {
	if fi == nil {
		return 0
	}
	return fi.Size()
}

// docs help <command>: convenience alias for command reference
var docsHelpCmd = &cobra.Command{
	Use:   "help <command>",
	Short: "Show help page for a specific command",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		// map to user-guide/commands/<command>
		target := "user-guide/commands/" + strings.TrimSpace(args[0])
		// Reuse docs show
		return docsShowCmd.RunE(cmd, []string{target})
	},
}

func matchesTypeFilter(slug, typ string) bool {
	switch strings.ToLower(typ) {
	case "commands":
		return strings.HasPrefix(slug, "user-guide/commands/")
	case "tutorials":
		return strings.HasPrefix(slug, "user-guide/tutorials/")
	case "workflows":
		return strings.HasPrefix(slug, "user-guide/workflows/")
	case "configuration":
		return strings.HasPrefix(slug, "configuration/")
	default:
		return true
	}
}

func htmlEscape(s string) string {
	// minimal escape
	r := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", "\"", "&quot;")
	return r.Replace(s)
}

func printDocsTree(w io.Writer, items []docListItem) {
	// Build nested map
	type node struct {
		name string
		size int64
		file bool
		kids map[string]*node
	}
	root := &node{name: "", kids: map[string]*node{}}
	for _, it := range items {
		parts := strings.Split(it.Slug, "/")
		cur := root
		for i, p := range parts {
			if cur.kids == nil {
				cur.kids = map[string]*node{}
			}
			if _, ok := cur.kids[p]; !ok {
				cur.kids[p] = &node{name: p, kids: map[string]*node{}}
			}
			cur = cur.kids[p]
			if i == len(parts)-1 {
				cur.file = true
				cur.size = it.Size
			}
		}
	}
	var walk func(n *node, prefix string)
	// Deterministic order
	walk = func(n *node, prefix string) {
		// Sort children names
		names := make([]string, 0, len(n.kids))
		for k := range n.kids {
			names = append(names, k)
		}
		sort.Strings(names)
		for i, name := range names {
			child := n.kids[name]
			last := i == len(names)-1
			connector := "├── "
			nextPrefix := prefix + "│   "
			if last {
				connector = "└── "
				nextPrefix = prefix + "    "
			}
			if child.file {
				_, _ = fmt.Fprintf(w, "%s%s%s (%dB)\n", prefix, connector, name, child.size)
			} else {
				_, _ = fmt.Fprintf(w, "%s%s%s/\n", prefix, connector, name)
			}
			walk(child, nextPrefix)
		}
	}
	walk(root, "")
}

func openBrowser(path string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", path) // #nosec G204 G702 - path is internal temp HTML from rendered docs
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", path) // #nosec G204 G702 - path is internal temp HTML from rendered docs
	default:
		cmd = exec.Command("xdg-open", path) // #nosec G204 G702 - path is internal temp HTML from rendered docs
	}
	return cmd.Start()
}

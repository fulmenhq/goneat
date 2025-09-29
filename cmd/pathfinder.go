package cmd

import (
	"encoding/json"
	"fmt"
	"path"
	"path/filepath"
	"strings"

	"github.com/fulmenhq/goneat/internal/ops"
	"github.com/fulmenhq/goneat/pkg/logger"
	"github.com/fulmenhq/goneat/pkg/pathfinder"
	"github.com/spf13/cobra"
)

var pathfinderCmd = &cobra.Command{
	Use:   "pathfinder",
	Short: "Path discovery utilities for goneat",
	RunE: func(cmd *cobra.Command, _ []string) error {
		return cmd.Help()
	},
}

var pathfinderFindCmd = &cobra.Command{
	Use:   "find",
	Short: "Discover files using pathfinder facade",
	RunE:  runPathfinderFind,
}

func init() {
	rootCmd.AddCommand(pathfinderCmd)
	pathfinderCmd.AddCommand(pathfinderFindCmd)

	capabilities := ops.GetDefaultCapabilities(ops.GroupSupport, ops.CategoryInformation)
	capabilities.SupportsJSON = true
	if err := ops.RegisterCommandWithTaxonomy("pathfinder", ops.GroupSupport, ops.CategoryInformation, capabilities, pathfinderCmd, "Discover files using goneat's pathfinder"); err != nil {
		panic(fmt.Sprintf("Failed to register pathfinder command: %v", err))
	}

	pathfinderFindCmd.Flags().String("path", ".", "Root path or loader source to search")
	pathfinderFindCmd.Flags().StringSlice("include", nil, "Glob patterns to include (doublestar supported)")
	pathfinderFindCmd.Flags().StringSlice("exclude", nil, "Glob patterns to exclude")
	pathfinderFindCmd.Flags().StringSlice("skip-dir", nil, "Directory substrings to skip during traversal")
	pathfinderFindCmd.Flags().Int("max-depth", -1, "Maximum directory depth (-1 for unlimited)")
	pathfinderFindCmd.Flags().Bool("follow-symlinks", false, "Follow symlinks during traversal")
	pathfinderFindCmd.Flags().Int("workers", 0, "Worker count for traversal (0 = auto)")
	pathfinderFindCmd.Flags().Bool("stream", false, "Stream results as they are discovered")
	pathfinderFindCmd.Flags().String("output", "json", "Output format: json|text")
	pathfinderFindCmd.Flags().Bool("show-source", false, "Include source path in text output")
	pathfinderFindCmd.Flags().String("strip-prefix", "", "Strip prefix from relative path when producing logical path")
	pathfinderFindCmd.Flags().String("logical-prefix", "", "Prepend prefix to logical path output")
	pathfinderFindCmd.Flags().Bool("flatten", false, "Use base filename as logical path (overrides strip-prefix)")
	pathfinderFindCmd.Flags().String("loader", "local", "Loader type to use (local, s3, gcs, etc.)")
	pathfinderFindCmd.Flags().Bool("schemas", false, "Enable schema signature discovery mode")
	pathfinderFindCmd.Flags().StringSlice("schema-id", nil, "Filter schema matches by signature id or alias")
	pathfinderFindCmd.Flags().StringSlice("schema-category", nil, "Filter schema matches by category (e.g., json-schema, openapi)")
	pathfinderFindCmd.Flags().Bool("schema-metadata", false, "Include signature metadata (match details, docs links)")
}

func runPathfinderFind(cmd *cobra.Command, _ []string) error {
	rootPath, _ := cmd.Flags().GetString("path")
	include, _ := cmd.Flags().GetStringSlice("include")
	exclude, _ := cmd.Flags().GetStringSlice("exclude")
	skipDirs, _ := cmd.Flags().GetStringSlice("skip-dir")
	maxDepth, _ := cmd.Flags().GetInt("max-depth")
	followSymlinks, _ := cmd.Flags().GetBool("follow-symlinks")
	workers, _ := cmd.Flags().GetInt("workers")
	stream, _ := cmd.Flags().GetBool("stream")
	outputFormat, _ := cmd.Flags().GetString("output")
	showSource, _ := cmd.Flags().GetBool("show-source")
	stripPrefix, _ := cmd.Flags().GetString("strip-prefix")
	logicalPrefix, _ := cmd.Flags().GetString("logical-prefix")
	flatten, _ := cmd.Flags().GetBool("flatten")
	loaderType, _ := cmd.Flags().GetString("loader")
	schemaMode, _ := cmd.Flags().GetBool("schemas")
	schemaIDs, _ := cmd.Flags().GetStringSlice("schema-id")
	schemaCategories, _ := cmd.Flags().GetStringSlice("schema-category")
	schemaMetadata, _ := cmd.Flags().GetBool("schema-metadata")

	output := strings.ToLower(outputFormat)
	if schemaMode && output == "json" && !schemaMetadata {
		schemaMetadata = true
	}
	if output != "json" && output != "text" {
		return fmt.Errorf("unsupported output format: %s", outputFormat)
	}

	// Normalize root path for local loader convenience
	if loaderType == "local" {
		cleaned := filepath.Clean(rootPath)
		if cleaned == "" {
			cleaned = "."
		}
		rootPath = cleaned
	}

	facade := pathfinder.NewFinderFacade(pathfinder.NewPathFinder(), pathfinder.FinderConfig{
		MaxWorkers: workers,
		LoaderType: loaderType,
	})

	query := pathfinder.FindQuery{
		Root:                  rootPath,
		Include:               include,
		Exclude:               exclude,
		SkipDirs:              skipDirs,
		MaxDepth:              maxDepth,
		FollowSymlinks:        followSymlinks,
		Workers:               workers,
		Context:               cmd.Context(),
		SchemaMode:            schemaMode,
		SchemaIDs:             schemaIDs,
		SchemaCategories:      schemaCategories,
		IncludeSchemaMetadata: schemaMetadata,
	}

	query.Transform = buildTransform(stripPrefix, logicalPrefix, flatten)

	if stream && output == "text" {
		return streamTextResults(cmd, facade, query, showSource)
	}

	results, err := facade.Find(query)
	if err != nil {
		return err
	}

	if stream {
		logger.Warn("Streaming currently writes buffered results for non-text outputs")
	}

	switch output {
	case "json":
		return writeResultsJSON(cmd, results)
	case "text":
		writeResultsText(cmd, results, showSource)
		return nil
	default:
		return fmt.Errorf("unsupported output format: %s", output)
	}
}

func streamTextResults(cmd *cobra.Command, facade *pathfinder.FinderFacade, query pathfinder.FindQuery, showSource bool) error {
	resultCh, errCh := facade.FindStream(query)
	for res := range resultCh {
		writeSingleTextResult(cmd, res, showSource)
	}
	if err := <-errCh; err != nil {
		return err
	}
	return nil
}

func writeResultsJSON(cmd *cobra.Command, results []pathfinder.PathResult) error {
	enc := json.NewEncoder(cmd.OutOrStdout())
	enc.SetIndent("", "  ")
	return enc.Encode(results)
}

func writeResultsText(cmd *cobra.Command, results []pathfinder.PathResult, showSource bool) {
	for _, res := range results {
		writeSingleTextResult(cmd, res, showSource)
	}
}

func writeSingleTextResult(cmd *cobra.Command, res pathfinder.PathResult, showSource bool) {
	label := res.LogicalPath
	if res.Metadata != nil {
		if schemaMeta, ok := res.Metadata["schema"].(map[string]any); ok {
			if id, ok := schemaMeta["id"].(string); ok && id != "" {
				label = fmt.Sprintf("%s [schema:%s]", label, id)
			} else if category, ok := schemaMeta["category"].(string); ok && category != "" {
				label = fmt.Sprintf("%s [schema:%s]", label, category)
			}
		}
	}
	if showSource {
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%s -> %s\n", label, res.SourcePath)
		return
	}
	_, _ = fmt.Fprintln(cmd.OutOrStdout(), label)
}

func buildTransform(stripPrefix, logicalPrefix string, flatten bool) pathfinder.PathTransform {
	if stripPrefix == "" && logicalPrefix == "" && !flatten {
		return nil
	}

	normalizedStrip := strings.Trim(strings.TrimPrefix(filepath.ToSlash(stripPrefix), "./"), "/")
	normalizedPrefix := strings.Trim(strings.TrimPrefix(filepath.ToSlash(logicalPrefix), "./"), "/")

	return func(result pathfinder.PathResult) pathfinder.PathResult {
		logical := result.RelativePath
		if normalizedStrip != "" {
			logical = strings.TrimPrefix(logical, normalizedStrip)
			logical = strings.TrimPrefix(logical, "/")
		}
		if flatten {
			logical = path.Base(logical)
		}
		if normalizedPrefix != "" {
			if logical == "" {
				logical = normalizedPrefix
			} else {
				logical = path.Join(normalizedPrefix, logical)
			}
		}
		logical = strings.TrimPrefix(path.Clean(logical), "./")
		if logical == "" {
			logical = result.RelativePath
		}
		result.LogicalPath = logical
		return result
	}
}

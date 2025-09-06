package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/3leaps/goneat/internal/assess"
	"github.com/spf13/cobra"
)

// prettyCmd is a stub renderer for piping JSON to console or HTML in future
var prettyCmd = &cobra.Command{
	Use:   "pretty",
	Short: "Render assessment JSON as pretty console or HTML (stub)",
	RunE: func(cmd *cobra.Command, args []string) error {
		to, _ := cmd.Flags().GetString("to")
		from, _ := cmd.Flags().GetString("from")
		input, _ := cmd.Flags().GetString("input")

		if from != "json" {
			return fmt.Errorf("only --from json is supported in stub")
		}
		var data []byte
		var err error
		if input != "" {
			// Validate input path to prevent path traversal
			input = filepath.Clean(input)
			if strings.Contains(input, "..") {
				return fmt.Errorf("invalid input path: contains path traversal")
			}
			data, err = os.ReadFile(input)
			if err != nil {
				return err
			}
		} else {
			data, err = io.ReadAll(os.Stdin)
			if err != nil {
				return err
			}
		}

		report, err := parseReportJSON(data)
		if err != nil {
			return err
		}

		switch to {
        case "console":
            f := assess.NewFormatter(assess.FormatConcise)
            out, err := f.FormatReport(report)
            if err != nil {
                return err
            }
            _, _ = fmt.Fprintln(cmd.OutOrStdout(), out)
            return nil
        case "html":
            f := assess.NewFormatter(assess.FormatHTML)
            out, err := f.FormatReport(report)
            if err != nil {
                return err
            }
            _, _ = fmt.Fprintln(cmd.OutOrStdout(), out)
            return nil
		default:
			return fmt.Errorf("unsupported --to: %s (use console or html)", to)
		}
	},
}

func init() {
	prettyCmd.Flags().String("from", "json", "Input format (json)")
	prettyCmd.Flags().String("to", "console", "Output (console, html)")
	prettyCmd.Flags().String("input", "", "Input file (default: stdin)")
	rootCmd.AddCommand(prettyCmd)
}

func parseReportJSON(data []byte) (*assess.AssessmentReport, error) {
	// Be tolerant of log preamble/postamble: extract first top-level JSON object
	s := string(data)
	start := -1
	braceCount := 0
	for i, r := range s {
		if r == '{' {
			start = i
			break
		}
	}
	if start == -1 {
		return nil, fmt.Errorf("no JSON object found in input")
	}
	end := start
	for i := start; i < len(s); i++ {
		switch s[i] {
		case '{':
			braceCount++
		case '}':
			braceCount--
			if braceCount == 0 {
				end = i + 1
				goto foundEnd
			}
		}
	}
foundEnd:
	if end <= start {
		return nil, fmt.Errorf("incomplete JSON object in input")
	}
	jsonSlice := s[start:end]

	var r assess.AssessmentReport
	if err := json.Unmarshal([]byte(jsonSlice), &r); err != nil {
		return nil, fmt.Errorf("failed to parse assessment JSON: %w", err)
	}
	return &r, nil
}

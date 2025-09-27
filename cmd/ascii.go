package cmd

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/fulmenhq/goneat/internal/ops"
	"github.com/fulmenhq/goneat/pkg/ascii"
	"github.com/spf13/cobra"
)

var (
	asciiWidth int
)

var asciiCmd = &cobra.Command{
	Use:   "ascii",
	Short: "Utilities for formatting ASCII output",
}

var asciiBoxCmd = &cobra.Command{
	Use:   "box",
	Short: "Render lines of text inside an ASCII box",
	Example: strings.TrimSpace(`  # Box the provided arguments, each treated as a line
  goneat ascii box "Status" "All systems nominal"

  # Box input from stdin
  goneat ascii box < message.txt

  # Truncate lines to 60 columns before boxing
  goneat ascii box --width 60 < report.txt

  # Generate box without terminal-specific width overrides (for debugging)
  goneat ascii box --raw < message.txt
`),
	RunE: func(cmd *cobra.Command, args []string) error {
		raw, _ := cmd.Flags().GetBool("raw")

		var lines []string
		if len(args) > 0 {
			lines = append(lines, args...)
		} else {
			scanner := bufio.NewScanner(os.Stdin)
			for scanner.Scan() {
				lines = append(lines, scanner.Text())
			}
			if err := scanner.Err(); err != nil {
				return fmt.Errorf("read input: %w", err)
			}
		}

		if asciiWidth > 0 {
			trimmed := make([]string, len(lines))
			for i, line := range lines {
				trimmed[i] = ascii.TruncateForBox(line, asciiWidth)
			}
			if raw {
				fmt.Print(ascii.BoxRaw(trimmed))
			} else {
				fmt.Print(ascii.Box(trimmed))
			}
		} else {
			if raw {
				fmt.Print(ascii.BoxRaw(lines))
			} else {
				fmt.Print(ascii.Box(lines))
			}
		}
		return nil
	},
}

var asciiStringInfoCmd = &cobra.Command{
	Use:   "stringinfo",
	Short: "Analyze string display width and byte length",
	Example: strings.TrimSpace(`  # Analyze a single string
  goneat ascii stringinfo "Hello 🌟 World"

	# Analyze input from stdin (splits on linefeeds)
  echo -e "Line 1\nLine 2 🌟" | goneat ascii stringinfo

  # Include terminal environment for analysis
  goneat ascii stringinfo --env <tests/fixtures/ascii/emojis-collection.txt
`),
	RunE: func(cmd *cobra.Command, args []string) error {
		includeEnv, _ := cmd.Flags().GetBool("env")

		var lines []string
		if len(args) > 0 {
			// For command line args, treat as single line
			input := strings.Join(args, " ")
			lines = []string{input}
		} else {
			// For stdin, read line by line (handles different line endings)
			scanner := bufio.NewScanner(os.Stdin)
			for scanner.Scan() {
				lines = append(lines, scanner.Text())
			}
			if err := scanner.Err(); err != nil {
				return fmt.Errorf("read input: %w", err)
			}
		}

		// Print terminal environment if requested
		if includeEnv && len(lines) > 1 {
			fmt.Printf("# Terminal Environment\n")
			fmt.Printf("# TERM=%s\n", os.Getenv("TERM"))
			fmt.Printf("# TERM_PROGRAM=%s\n", os.Getenv("TERM_PROGRAM"))
			fmt.Printf("# \n")
		}

		// Analyze each line
		for i, line := range lines {
			byteLen, displayWidth := ascii.StringWidthInfo(line)
			if len(lines) > 1 {
				// Use zero-padded line numbers and fixed-width columns
				fmt.Printf("Line %04d: %-15s %-18s %s\n",
					i+1,
					fmt.Sprintf("byte_len=%d", byteLen),
					fmt.Sprintf("display_width=%d", displayWidth),
					fmt.Sprintf("content=%q", line))
			} else {
				fmt.Printf("%-15s %-18s %s\n",
					fmt.Sprintf("byte_len=%d", byteLen),
					fmt.Sprintf("display_width=%d", displayWidth),
					fmt.Sprintf("content=%q", line))
			}
		}

		return nil
	},
}

var asciiDiagCmd = &cobra.Command{
	Use:   "diag",
	Short: "Diagnose terminal Unicode width handling",
	Long: `Diagnose how the current terminal handles Unicode character widths.
This helps debug ASCII box alignment issues across different terminals.`,
	Example: strings.TrimSpace(`  # Run terminal diagnostics
  goneat ascii diag

  # Test specific emojis
  goneat ascii diag "🚀" "ℹ️" "📋"
`),
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("=== Terminal Environment ===")
		fmt.Printf("TERM: %s\n", os.Getenv("TERM"))
		fmt.Printf("LANG: %s\n", os.Getenv("LANG"))
		fmt.Printf("LC_ALL: %s\n", os.Getenv("LC_ALL"))
		fmt.Printf("TERM_PROGRAM: %s\n", os.Getenv("TERM_PROGRAM"))

		// Test emojis - either from args or default set
		var testEmojis []string
		if len(args) > 0 {
			testEmojis = args
		} else {
			testEmojis = []string{
				"🚀", "ℹ️", "⚠️", "❌", "✅", "🔄", "⏳", "⏱️",
				"📋", "📂", "💻", "🔗", "🎯", "🛠️", "🔧", "📊",
				"🔍", "📝", "💡", "🎉",
			}
		}

		fmt.Println("\n=== Character Width Analysis ===")
		fmt.Println("Char | Width | Bytes | Analysis")
		fmt.Println("-----|-------|-------|----------")

		for _, emoji := range testEmojis {
			width := ascii.StringWidth(emoji)
			byteLen := len(emoji)

			// Analyze if it might be problematic
			analysis := "OK"
			if width == 1 && byteLen > 3 {
				analysis = "May render as 2 in some terminals"
			} else if width == 2 && byteLen == 3 {
				analysis = "May render as 1 in some terminals"
			}

			fmt.Printf("%-4s | %5d | %5d | %s\n", emoji, width, byteLen, analysis)
		}

		// Show a sample box to visually check alignment
		fmt.Println("\n=== Visual Alignment Test ===")
		sampleLines := []string{
			"Test Box Alignment",
			"ℹ️  Information (w=" + fmt.Sprintf("%d", ascii.StringWidth("ℹ️")) + ")",
			"🚀  Rocket (w=" + fmt.Sprintf("%d", ascii.StringWidth("🚀")) + ")",
			"📋  Clipboard (w=" + fmt.Sprintf("%d", ascii.StringWidth("📋")) + ")",
		}

		box := ascii.Box(sampleLines)
		fmt.Print(box)

		fmt.Println("\n=== Recommendations ===")
		fmt.Println("If the box above appears misaligned:")
		fmt.Println("1. Your terminal may render emojis differently than calculated")
		fmt.Println("2. Try setting: export RUNEWIDTH_EASTASIAN=1")
		fmt.Println("3. Consider using a different terminal or font")
		fmt.Println("4. Report the issue with your terminal details")

		// Terminal detection info
		fmt.Println("\n=== Terminal Detection ===")
		termProgram := os.Getenv("TERM_PROGRAM")
		switch termProgram {
		case "iTerm.app":
			fmt.Println("Detected: iTerm2")
			fmt.Println("Known issues: Some emoji+variation selector sequences may misalign")
		case "Apple_Terminal":
			fmt.Println("Detected: macOS Terminal")
			fmt.Println("Status: Generally works well with default settings")
		case "ghostty":
			fmt.Println("Detected: Ghostty")
			fmt.Println("Status: Custom width handling active for known problematic emojis")
		default:
			if termProgram != "" {
				fmt.Printf("Detected: %s (unknown behavior profile)\n", termProgram)
			} else {
				fmt.Println("Terminal program not detected (TERM_PROGRAM not set)")
			}
		}

		return nil
	},
}

var asciiCalibrateCmd = &cobra.Command{
	Use:   "calibrate [test-file]",
	Short: "Interactively calibrate terminal character widths",
	Long: `Interactively calibrate character widths for the current terminal.
	
This command displays a test box and guides you through identifying characters
that render wider or narrower than calculated. Adjustments are saved to your
GONEAT_HOME configuration.`,
	Example: strings.TrimSpace(`  # Calibrate using emoji collection
  goneat ascii calibrate tests/fixtures/ascii/emojis-collection.txt

  # Calibrate using logging emojis  
  goneat ascii calibrate tests/fixtures/ascii/logging-emojis.txt

  # Calibrate using guardian approval box
  goneat ascii calibrate tests/fixtures/ascii/guardian-approval.txt

  # Override terminal detection for testing
  goneat ascii calibrate --term-program=ghostty tests/fixtures/ascii/emojis-collection.txt
  goneat ascii calibrate --term=xterm-ghostty tests/fixtures/ascii/logging-emojis.txt
`),
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		testFile := args[0]

		// Check if test file exists
		if _, err := os.Stat(testFile); os.IsNotExist(err) {
			return fmt.Errorf("test file not found: %s", testFile)
		}

		// Get override flags
		term, _ := cmd.Flags().GetString("term")
		termProgram, _ := cmd.Flags().GetString("term-program")

		session := ascii.NewCalibrationSession(testFile).WithTerminalOverride(term, termProgram)
		return session.RunInteractiveCalibration()
	},
}

var asciiMarkCmd = &cobra.Command{
	Use:   "mark",
	Short: "Mark characters as wide or narrow",
	Long: `Mark specific characters as wider or narrower than calculated.
	
This is a quick way to adjust character widths without the interactive session.
Changes are applied to the current terminal configuration.`,
	Example: strings.TrimSpace(`  # Mark emojis as too wide
  goneat ascii mark --wide "🚀" "ℹ️" "📋"

  # Mark symbols as too narrow  
  goneat ascii mark --narrow "→" "←" "↑"

  # Combine both
  goneat ascii mark --wide "🎯" --narrow "·"

  # Override terminal for specific configurations
  goneat ascii mark --term-program=iTerm.app --wide "✨" "🔥"
  goneat ascii mark --term=screen-256color --narrow "→" "←"
`),
	RunE: func(cmd *cobra.Command, args []string) error {
		wideChars, _ := cmd.Flags().GetStringSlice("wide")
		narrowChars, _ := cmd.Flags().GetStringSlice("narrow")

		if len(wideChars) == 0 && len(narrowChars) == 0 {
			return fmt.Errorf("must specify --wide or --narrow characters")
		}

		// Trim spaces from character inputs (common when pasting in terminal)
		for i, char := range wideChars {
			wideChars[i] = strings.TrimSpace(char)
		}
		for i, char := range narrowChars {
			narrowChars[i] = strings.TrimSpace(char)
		}

		// Get override flags
		term, _ := cmd.Flags().GetString("term")
		termProgram, _ := cmd.Flags().GetString("term-program")

		if len(wideChars) > 0 {
			if err := ascii.MarkCharactersWideWithOverride(wideChars, term, termProgram); err != nil {
				return fmt.Errorf("failed to mark wide characters: %w", err)
			}
			fmt.Printf("Marked %d characters as wide\n", len(wideChars))
		}

		if len(narrowChars) > 0 {
			if err := ascii.MarkCharactersNarrowWithOverride(narrowChars, term, termProgram); err != nil {
				return fmt.Errorf("failed to mark narrow characters: %w", err)
			}
			fmt.Printf("Marked %d characters as narrow\n", len(narrowChars))
		}

		return nil
	},
}

var asciiResetCmd = &cobra.Command{
	Use:   "reset",
	Short: "Reset user terminal configuration to defaults",
	Long: `Reset user terminal configuration to repository defaults.
	
This removes your user-specific terminal overrides and resets to the
embedded defaults from the goneat repository. Useful for testing or
starting fresh with calibration.`,
	Example: strings.TrimSpace(`  # Reset to defaults
  goneat ascii reset

  # Reset and immediately calibrate
  goneat ascii reset && goneat ascii calibrate tests/fixtures/ascii/emojis-collection.txt
`),
	RunE: func(cmd *cobra.Command, args []string) error {
		return ascii.ResetUserConfig()
	},
}

var asciiDebugCmd = &cobra.Command{
	Use:   "debug",
	Short: "Debug terminal catalog configuration",
	Long:  `Debug information about terminal catalog loading and configuration.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ascii.DebugTerminalCatalog()
		return nil
	},
}

var asciiExpandCmd = &cobra.Command{
	Use:   "expand [input-file]",
	Short: "Expand multi-emoji lines into single-emoji calibration format",
	Long: `Convert multi-emoji test files into single-emoji-per-line format for easier calibration.
	
This command takes a file with multiple emojis per line (like emojis-collection.txt)
and converts it to one emoji per line with descriptive labels for visual calibration.`,
	Example: strings.TrimSpace(`  # Expand emoji collection for calibration
  goneat ascii expand tests/fixtures/ascii/emojis-collection.txt

  # Save to file for testing
  goneat ascii expand tests/fixtures/ascii/emojis-collection.txt > emoji-test.txt
`),
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		var input io.Reader = os.Stdin

		if len(args) > 0 {
			file, err := os.Open(args[0])
			if err != nil {
				return fmt.Errorf("failed to open file: %w", err)
			}
			defer file.Close()
			input = file
		}

		scanner := bufio.NewScanner(input)
		lineNum := 0

		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			lineNum++

			if line == "" {
				continue
			}

			// If first line or contains mostly text, use as header
			if lineNum == 1 || strings.Count(line, " ") > len([]rune(line))/2 {
				fmt.Println(line + " (Expanded for Calibration)")
				continue
			}

			// Extract grapheme clusters from the line (handles emoji+variation selector)
			clusters := extractGraphemeClusters(line)
			for _, cluster := range clusters {
				if isUnicodeCluster(cluster) {
					width := ascii.StringWidth(cluster)
					byteLen := len(cluster)

					// Create descriptive line with proper cluster info
					clusterInfo := getClusterInfo(cluster)
					fmt.Printf("%s  %s (width=%d, bytes=%d)\n", cluster, clusterInfo, width, byteLen)
				}
			}
		}

		if err := scanner.Err(); err != nil {
			return fmt.Errorf("error reading input: %w", err)
		}

		return nil
	},
}

var asciiAnalyzeCmd = &cobra.Command{
	Use:   "analyze",
	Short: "Analyze box rendering and generate width adjustment suggestions",
	Long: `Analyze box rendering by comparing expected vs actual alignment and generate
width adjustment suggestions automatically.

This command takes a test file, renders it as a box, and analyzes the output
to detect characters that might be rendering wider or narrower than calculated.`,
	Example: strings.TrimSpace(`  # Analyze and suggest adjustments (pipe box output)
  TERM_PROGRAM=iTerm.app goneat ascii box <tests/fixtures/ascii/emojis-collection.txt | goneat ascii analyze

  # Analyze stringinfo output for width discrepancies  
  TERM_PROGRAM=iTerm.app goneat ascii stringinfo <tests/fixtures/ascii/emojis-collection.txt | goneat ascii analyze --stringinfo

	# Generate mark commands for specific terminal
  goneat ascii analyze --terminal iTerm.app --generate-marks < box-output.txt

  # Auto-apply adjustments directly to config
  TERM_PROGRAM=ghostty goneat ascii stringinfo --env <tests/fixtures/ascii/emojis-collection.txt | goneat ascii analyze --stringinfo --apply
`),
	RunE: func(cmd *cobra.Command, args []string) error {
		stringinfoMode, _ := cmd.Flags().GetBool("stringinfo")
		generateMarks, _ := cmd.Flags().GetBool("generate-marks")
		apply, _ := cmd.Flags().GetBool("apply")
		terminal, _ := cmd.Flags().GetString("terminal")

		if stringinfoMode {
			return ascii.AnalyzeStringInfoOutput(os.Stdin, terminal, generateMarks, apply)
		}

		return ascii.AnalyzeBoxOutput(os.Stdin, terminal, generateMarks, apply)
	},
}

// extractGraphemeClusters extracts grapheme clusters from a string
// This handles emoji+variation selector sequences properly
func extractGraphemeClusters(s string) []string {
	var clusters []string
	runes := []rune(s)

	for i := 0; i < len(runes); i++ {
		cluster := string(runes[i])

		// Check if next rune is a variation selector
		if i+1 < len(runes) && runes[i+1] == 0xFE0F {
			cluster += string(runes[i+1])
			i++ // Skip the variation selector in next iteration
		}

		clusters = append(clusters, cluster)
	}

	return clusters
}

// isUnicodeCluster checks if a cluster is a Unicode character (not ASCII space)
func isUnicodeCluster(cluster string) bool {
	if cluster == " " {
		return false
	}
	runes := []rune(cluster)
	return len(runes) > 0 && runes[0] > 127
}

// getClusterInfo generates description for a grapheme cluster with normalized width
func getClusterInfo(cluster string) string {
	runes := []rune(cluster)

	var description string
	if len(runes) == 1 {
		// Single character
		description = fmt.Sprintf("Character U+%04X", runes[0])
	} else if len(runes) == 2 && runes[1] == 0xFE0F {
		// Emoji + variation selector - use shorter notation for VS
		description = fmt.Sprintf("Character U+%04X+VS", runes[0])
	} else {
		// Complex cluster
		var parts []string
		for _, r := range runes {
			if r == 0xFE0F {
				parts = append(parts, "VS") // Abbreviate variation selector
			} else {
				parts = append(parts, fmt.Sprintf("U+%04X", r))
			}
		}
		description = fmt.Sprintf("Character %s", strings.Join(parts, "+"))
	}

	// Normalize description width to 30 characters for consistent box alignment
	const maxDescWidth = 30
	if len(description) > maxDescWidth {
		description = description[:maxDescWidth-3] + "..."
	}
	description = fmt.Sprintf("%-*s", maxDescWidth, description)

	return description
}

func init() {
	rootCmd.AddCommand(asciiCmd)

	capabilities := ops.GetDefaultCapabilities(ops.GroupNeat, ops.CategoryFormatting)
	if err := ops.RegisterCommandWithTaxonomy("ascii", ops.GroupNeat, ops.CategoryFormatting, capabilities, asciiCmd, "ASCII formatting helpers"); err != nil {
		panic(fmt.Sprintf("Failed to register ascii command: %v", err))
	}

	asciiCmd.AddCommand(asciiBoxCmd)
	asciiBoxCmd.Flags().IntVarP(&asciiWidth, "width", "w", 0, "truncate lines to the specified display width before boxing")
	asciiBoxCmd.Flags().Bool("raw", false, "generate box without terminal-specific width overrides")

	asciiCmd.AddCommand(asciiStringInfoCmd)
	asciiCmd.AddCommand(asciiDiagCmd)
	asciiCmd.AddCommand(asciiCalibrateCmd)
	asciiCmd.AddCommand(asciiMarkCmd)
	asciiCmd.AddCommand(asciiResetCmd)
	asciiCmd.AddCommand(asciiDebugCmd)
	asciiCmd.AddCommand(asciiExpandCmd)
	asciiCmd.AddCommand(asciiAnalyzeCmd)

	// Add flags for calibrate command
	asciiCalibrateCmd.Flags().String("term", "", "Override TERM environment variable")
	asciiCalibrateCmd.Flags().String("term-program", "", "Override TERM_PROGRAM environment variable")

	// Add flags for mark command
	asciiMarkCmd.Flags().StringSlice("wide", nil, "Characters to mark as too wide")
	asciiMarkCmd.Flags().StringSlice("narrow", nil, "Characters to mark as too narrow")
	asciiMarkCmd.Flags().String("term", "", "Override TERM environment variable")
	asciiMarkCmd.Flags().String("term-program", "", "Override TERM_PROGRAM environment variable")

	// Add flags for stringinfo command
	asciiStringInfoCmd.Flags().Bool("env", false, "Include terminal environment information in output")

	// Add flags for analyze command
	asciiAnalyzeCmd.Flags().Bool("stringinfo", false, "Analyze stringinfo output instead of box output")
	asciiAnalyzeCmd.Flags().Bool("generate-marks", false, "Generate goneat ascii mark commands")
	asciiAnalyzeCmd.Flags().Bool("apply", false, "Apply adjustments directly to user config")
	asciiAnalyzeCmd.Flags().String("terminal", "", "Target terminal for generated mark commands")
}

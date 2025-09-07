/*
Copyright © 2025 3 Leaps <info@3leaps.com>
*/
package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/fulmenhq/goneat/internal/ops"
	"github.com/fulmenhq/goneat/pkg/config"
	"github.com/fulmenhq/goneat/pkg/logger"
	"github.com/spf13/cobra"
)

// ANSI color codes
const (
	colorReset  = "\033[0m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorBlue   = "\033[34m"
	colorCyan   = "\033[36m"
	colorBold   = "\033[1m"
)

// colorize returns colored text if colors are enabled
func colorize(text, color string, useColor bool) string {
	if !useColor {
		return text
	}
	return color + text + colorReset
}

// getColorPreference checks if colors should be used
func getColorPreference(cmd *cobra.Command) bool {
	noColor, _ := cmd.Flags().GetBool("no-color")
	return !noColor
}

// EnvData represents the structured data for environment information.
type EnvData struct {
	System    SystemInfo        `json:"system"`
	Variables map[string]string `json:"variables"`
	Extended  *ExtendedInfo     `json:"extended,omitempty"`
}

// ExtendedInfo holds extended system information.
type ExtendedInfo struct {
	Processor   string                 `json:"processor"`
	OSVersion   string                 `json:"osVersion"`
	Memory      string                 `json:"memory"`
	DiskUsage   string                 `json:"diskUsage"`
	DirStats    string                 `json:"dirStats"`
	GoEcosystem string                 `json:"goEcosystem"`
	IgnoreFiles map[string]interface{} `json:"ignoreFiles"`
}

// SystemInfo holds system-related information.
type SystemInfo struct {
	OS           string    `json:"os"`
	Architecture string    `json:"architecture"`
	GoVersion    string    `json:"goVersion"`
	NumCPU       int       `json:"numCPU"`
	Hostname     string    `json:"hostname"`
	WorkingDir   string    `json:"workingDir"`
	Timestamp    time.Time `json:"timestamp"`
	Version      string    `json:"version"`
}

// envinfoCmd represents the envinfo command
var envinfoCmd = &cobra.Command{
	Use:   "envinfo",
	Short: "Display environment and system information",
	Long: `Display detailed information about the system and environment variables.

This command provides insights into the operating system, architecture, Go version,
and environment variables that affect the behavior of goneat.`,
	RunE: runEnvinfo,
}

func init() {
	rootCmd.AddCommand(envinfoCmd)

	// Register command in ops registry with taxonomy
	capabilities := ops.GetDefaultCapabilities(ops.GroupSupport, ops.CategoryEnvironment)
	if err := ops.RegisterCommandWithTaxonomy("envinfo", ops.GroupSupport, ops.CategoryEnvironment, capabilities, envinfoCmd, "Show system information"); err != nil {
		panic(fmt.Sprintf("Failed to register envinfo command: %v", err))
	}

	envinfoCmd.Flags().Bool("json", false, "Output in JSON format")
	envinfoCmd.Flags().Bool("extended", false, "Show extended system information including disk usage and tree stats")
}

func runEnvinfo(cmd *cobra.Command, args []string) error {
	jsonFormat, _ := cmd.Flags().GetBool("json")
	extended, _ := cmd.Flags().GetBool("extended")
	useColor := getColorPreference(cmd)

	envData := collectEnvironmentData(extended)

	out := cmd.OutOrStdout()

	if jsonFormat {
		jsonData, err := json.MarshalIndent(envData, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to format JSON output: %v", err)
		}
		if _, err := fmt.Fprintln(out, string(jsonData)); err != nil {
			return fmt.Errorf("failed to write JSON output: %v", err)
		}
		return nil
	}

	// System Information Section
	header := colorize("🖥️  System Information", colorBold+colorBlue, useColor)
	if _, err := fmt.Fprintln(out, header); err != nil {
		return fmt.Errorf("failed to write system header: %v", err)
	}
	separator := colorize("==================================================", colorCyan, useColor)
	if _, err := fmt.Fprintln(out, separator); err != nil {
		return fmt.Errorf("failed to write separator: %v", err)
	}

	// Color keys in cyan, values use terminal default (adapts to light/dark mode)
	keyColor := colorCyan
	valueColor := "" // Terminal default text color
	resetColor := colorReset
	if !useColor {
		keyColor = ""
		resetColor = ""
	}

	_, _ = fmt.Fprintf(out, "%s%-16s%s | %s%s%s\n", keyColor, "OS", resetColor, valueColor, envData.System.OS, resetColor)
	_, _ = fmt.Fprintf(out, "%s%-16s%s | %s%s%s\n", keyColor, "Architecture", resetColor, valueColor, envData.System.Architecture, resetColor)
	_, _ = fmt.Fprintf(out, "%s%-16s%s | %s%s%s\n", keyColor, "Go Version", resetColor, valueColor, envData.System.GoVersion, resetColor)
	_, _ = fmt.Fprintf(out, "%s%-16s%s | %s%d%s\n", keyColor, "CPU Cores", resetColor, valueColor, envData.System.NumCPU, resetColor)
	_, _ = fmt.Fprintf(out, "%s%-16s%s | %s%s%s\n", keyColor, "Hostname", resetColor, valueColor, envData.System.Hostname, resetColor)
	_, _ = fmt.Fprintf(out, "%s%-16s%s | %s%s%s\n", keyColor, "Working Dir", resetColor, valueColor, envData.System.WorkingDir, resetColor)
	_, _ = fmt.Fprintf(out, "%s%-16s%s | %s%s%s\n", keyColor, "Timestamp", resetColor, valueColor, envData.System.Timestamp.Format(time.RFC3339), resetColor)
	_, _ = fmt.Fprintf(out, "%s%-16s%s | %s%s%s\n", keyColor, "Goneat Version", resetColor, valueColor, envData.System.Version, resetColor)

	if extended {
		// Extended Information Section
		if _, err := fmt.Fprintln(out, ""); err != nil {
			return fmt.Errorf("failed to write newline: %v", err)
		}
		extendedHeader := colorize("🔧 Extended Information", colorBold+colorGreen, useColor)
		if _, err := fmt.Fprintln(out, extendedHeader); err != nil {
			return fmt.Errorf("failed to write extended header: %v", err)
		}
		if _, err := fmt.Fprintln(out, separator); err != nil {
			return fmt.Errorf("failed to write extended separator: %v", err)
		}

		if envData.Extended != nil {
			_, _ = fmt.Fprintf(out, "%s%-16s%s | %s%s%s\n", keyColor, "Processor", resetColor, valueColor, envData.Extended.Processor, resetColor)
			_, _ = fmt.Fprintf(out, "%s%-16s%s | %s%s%s\n", keyColor, "OS Version", resetColor, valueColor, envData.Extended.OSVersion, resetColor)
			_, _ = fmt.Fprintf(out, "%s%-16s%s | %s%s%s\n", keyColor, "Memory", resetColor, valueColor, envData.Extended.Memory, resetColor)
			_, _ = fmt.Fprintf(out, "%s%-16s%s | %s%s%s\n", keyColor, "Disk Usage", resetColor, valueColor, envData.Extended.DiskUsage, resetColor)
			_, _ = fmt.Fprintf(out, "%s%-16s%s | %s%s%s\n", keyColor, "Directory Stats", resetColor, valueColor, envData.Extended.DirStats, resetColor)

			if _, err := fmt.Fprintln(out, ""); err != nil {
				return fmt.Errorf("failed to write newline: %v", err)
			}
			ecosystemHeader := colorize("🐹 Go Ecosystem", colorBold+colorYellow, useColor)
			if _, err := fmt.Fprintln(out, ecosystemHeader); err != nil {
				return fmt.Errorf("failed to write go ecosystem header: %v", err)
			}
			if _, err := fmt.Fprintln(out, separator); err != nil {
				return fmt.Errorf("failed to write go ecosystem separator: %v", err)
			}
			// Print Go ecosystem info with proper formatting
			lines := strings.Split(envData.Extended.GoEcosystem, "\n")
			for _, line := range lines {
				if strings.TrimSpace(line) != "" {
					if useColor && valueColor != "" {
						if _, err := fmt.Fprintf(out, "%s%s%s\n", valueColor, line, resetColor); err != nil {
							logger.Warn(fmt.Sprintf("Failed to write to output: %v", err))
						}
					} else {
						if _, err := fmt.Fprintln(out, line); err != nil {
							logger.Warn(fmt.Sprintf("Failed to write to output: %v", err))
						}
					}
				}
			}

			// Ignore Files Section
			if _, err := fmt.Fprintln(out, ""); err != nil {
				return fmt.Errorf("failed to write newline: %v", err)
			}
			ignoreHeader := colorize("🚫 Ignore Configuration", colorBold+colorYellow, useColor)
			if _, err := fmt.Fprintln(out, ignoreHeader); err != nil {
				return fmt.Errorf("failed to write ignore header: %v", err)
			}
			if _, err := fmt.Fprintln(out, separator); err != nil {
				return fmt.Errorf("failed to write ignore separator: %v", err)
			}

			// Display ignore file information
			if gitignoreStatus, ok := envData.Extended.IgnoreFiles[".gitignore"].(map[string]interface{}); ok {
				exists := gitignoreStatus["exists"].(bool)
				path := gitignoreStatus["path"].(string)
				status := "❌ Not found"
				if exists {
					patterns := gitignoreStatus["patterns"].(int)
					status = fmt.Sprintf("✅ Found (%d patterns)", patterns)
				}
				_, _ = fmt.Fprintf(out, "%s%-20s%s | %s%s%s\n", keyColor, ".gitignore", resetColor, valueColor, status, resetColor)
				_, _ = fmt.Fprintf(out, "%s%-20s%s | %s%s%s\n", keyColor, "  Path", resetColor, valueColor, path, resetColor)
			}

			if goneatIgnoreStatus, ok := envData.Extended.IgnoreFiles[".goneatignore"].(map[string]interface{}); ok {
				exists := goneatIgnoreStatus["exists"].(bool)
				path := goneatIgnoreStatus["path"].(string)
				status := "❌ Not found"
				if exists {
					patterns := goneatIgnoreStatus["patterns"].(int)
					status = fmt.Sprintf("✅ Found (%d patterns)", patterns)
				}
				_, _ = fmt.Fprintf(out, "%s%-20s%s | %s%s%s\n", keyColor, ".goneatignore", resetColor, valueColor, status, resetColor)
				_, _ = fmt.Fprintf(out, "%s%-20s%s | %s%s%s\n", keyColor, "  Path", resetColor, valueColor, path, resetColor)
			}

			if userIgnoreStatus, ok := envData.Extended.IgnoreFiles["user-.goneatignore"].(map[string]interface{}); ok {
				exists := userIgnoreStatus["exists"].(bool)
				path := userIgnoreStatus["path"].(string)
				status := "❌ Not found"
				if exists {
					patterns := userIgnoreStatus["patterns"].(int)
					status = fmt.Sprintf("✅ Found (%d patterns)", patterns)
				}
				_, _ = fmt.Fprintf(out, "%s%-20s%s | %s%s%s\n", keyColor, "~/.goneat/.goneatignore", resetColor, valueColor, status, resetColor)
				_, _ = fmt.Fprintf(out, "%s%-20s%s | %s%s%s\n", keyColor, "  Path", resetColor, valueColor, path, resetColor)
			}
		}
	}

	return nil
}

// collectEnvironmentData gathers system information.
func collectEnvironmentData(extended bool) EnvData {
	hostname, _ := os.Hostname()
	wd, _ := os.Getwd()

	systemInfo := SystemInfo{
		OS:           runtime.GOOS,
		Architecture: runtime.GOARCH,
		GoVersion:    runtime.Version(),
		NumCPU:       runtime.NumCPU(),
		Hostname:     hostname,
		WorkingDir:   wd,
		Timestamp:    time.Now(),
		Version:      "v0.1.0", // TODO: make this dynamic
	}

	envData := EnvData{
		System:    systemInfo,
		Variables: make(map[string]string), // Placeholder
	}

	if extended {
		envData.Extended = &ExtendedInfo{
			Processor:   getProcessorInfo(),
			OSVersion:   getOSVersion(),
			Memory:      getMemoryInfo(),
			DiskUsage:   getDiskUsage(),
			DirStats:    getDirStats(),
			GoEcosystem: getGoEcosystem(),
			IgnoreFiles: getIgnoreFileInfo(),
		}
	}

	return envData
}

// getProcessorInfo returns processor information
func getProcessorInfo() string {
	// Placeholder - in real implementation, use sysinfo or similar
	return "Unknown Processor"
}

// getOSVersion returns OS version and build
func getOSVersion() string {
	// Placeholder - use runtime or exec uname
	return runtime.GOOS + " (build unknown)"
}

// getMemoryInfo returns memory information
func getMemoryInfo() string {
	// Placeholder - use sysinfo
	return "Unknown"
}

// getDiskUsage returns disk usage in df -h style
func getDiskUsage() string {
	// Placeholder - use exec df
	return "Filesystem 1K-blocks Used Available Use% Mounted on\n/dev/disk1s1s1 1000000 500000 500000 50% /"
}

// getDirStats returns directory stats in du -h style for current dir
func getDirStats() string {
	// Placeholder - use exec du
	return "4.0K .\n8.0K ./cmd\n12.0K ./pkg"
}

// getGoEcosystem returns information about installed Go tools and packages
func getGoEcosystem() string {
	gopath := os.Getenv("GOPATH")
	if gopath == "" {
		gopath = os.Getenv("HOME") + "/go"
	}

	binDir := gopath + "/bin"
	files, err := os.ReadDir(binDir)
	if err != nil {
		return "Unable to read GOPATH/bin: " + err.Error()
	}

	var tools []string
	for _, file := range files {
		if !file.IsDir() {
			tools = append(tools, file.Name())
		}
	}

	if len(tools) == 0 {
		return "No Go tools found in " + binDir
	}

	result := "Installed Go tools in " + binDir + ":\n"
	for _, tool := range tools {
		result += "  - " + tool + "\n"
	}
	return result
}

// getIgnoreFileInfo returns information about ignore files
func getIgnoreFileInfo() map[string]interface{} {
	info := make(map[string]interface{})

	// Check .gitignore in current directory
	wd, _ := os.Getwd()
	gitignorePath := filepath.Join(wd, ".gitignore")
	gitignoreInfo := map[string]interface{}{
		"path":     gitignorePath,
		"exists":   false,
		"patterns": 0,
	}
    // Read with sanitized path; file is at repository root
    if content, err := os.ReadFile(filepath.Clean(gitignorePath)); err == nil { // #nosec G304 -- fixed path under working directory
		gitignoreInfo["exists"] = true
		gitignoreInfo["patterns"] = countPatterns(string(content))
	}
	info[".gitignore"] = gitignoreInfo

	// Check .goneatignore in current directory
	goneatignorePath := filepath.Join(wd, ".goneatignore")
	goneatignoreInfo := map[string]interface{}{
		"path":     goneatignorePath,
		"exists":   false,
		"patterns": 0,
	}
    if content, err := os.ReadFile(filepath.Clean(goneatignorePath)); err == nil { // #nosec G304 -- fixed path under working directory
		goneatignoreInfo["exists"] = true
		goneatignoreInfo["patterns"] = countPatterns(string(content))
	}
	info[".goneatignore"] = goneatignoreInfo

	// Check user global .goneatignore
	var userIgnorePath string
	if goneatHome, err := config.GetGoneatHome(); err == nil {
		userIgnorePath = filepath.Join(goneatHome, ".goneatignore")
	} else if homeDir, err := os.UserHomeDir(); err == nil {
		userIgnorePath = filepath.Join(homeDir, ".goneat", ".goneatignore")
	}

	userIgnoreInfo := map[string]interface{}{
		"path":     userIgnorePath,
		"exists":   false,
		"patterns": 0,
	}
    if userIgnorePath != "" {
        // User override is constrained to goneat home or $HOME/.goneat
        if content, err := os.ReadFile(filepath.Clean(userIgnorePath)); err == nil { // #nosec G304 -- cleaned path within user profile
			userIgnoreInfo["exists"] = true
			userIgnoreInfo["patterns"] = countPatterns(string(content))
		}
	}
	info["user-.goneatignore"] = userIgnoreInfo

	return info
}

// countPatterns counts non-empty, non-comment lines in ignore file content
func countPatterns(content string) int {
	count := 0
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" && !strings.HasPrefix(line, "#") {
			count++
		}
	}
	return count
}

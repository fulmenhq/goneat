package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/viper"
)

// Config holds all configuration for goneat
type Config struct {
	Format   FormatConfig   `mapstructure:"format"`
	Security SecurityConfig `mapstructure:"security"`
	Schema   SchemaConfig   `mapstructure:"schema"`
}

// FormatConfig holds formatting configuration
type FormatConfig struct {
	Go       GoFormatConfig       `mapstructure:"go"`
	YAML     YAMLFormatConfig     `mapstructure:"yaml"`
	JSON     JSONFormatConfig     `mapstructure:"json"`
	Markdown MarkdownFormatConfig `mapstructure:"markdown"`
}

// GoFormatConfig holds Go formatting options
type GoFormatConfig struct {
	// Currently gofmt has minimal config, but we can add options here
	Simplify bool `mapstructure:"simplify"` // -s flag: simplify code
}

// YAMLFormatConfig holds YAML formatting options
type YAMLFormatConfig struct {
	Indent          int    `mapstructure:"indent"`
	LineLength      int    `mapstructure:"line_length"`
	QuoteStyle      string `mapstructure:"quote_style"` // "single", "double"
	TrailingNewline bool   `mapstructure:"trailing_newline"`
}

// JSONFormatConfig holds JSON formatting options
type JSONFormatConfig struct {
	Indent          string `mapstructure:"indent"` // "  " or "\t"
	Compact         bool   `mapstructure:"compact"`
	SortKeys        bool   `mapstructure:"sort_keys"`
	TrailingNewline bool   `mapstructure:"trailing_newline"`
}

// MarkdownFormatConfig holds Markdown formatting options
type MarkdownFormatConfig struct {
	LineLength     int    `mapstructure:"line_length"`
	TrailingSpaces bool   `mapstructure:"trailing_spaces"`
	ReferenceStyle string `mapstructure:"reference_style"`  // "collapsed", "full"
	CodeBlockStyle string `mapstructure:"code_block_style"` // "fenced", "indented"
}

var defaultConfig = Config{
	Format: FormatConfig{
		Go: GoFormatConfig{
			Simplify: true,
		},
		YAML: YAMLFormatConfig{
			Indent:          2,
			LineLength:      80,
			QuoteStyle:      "double",
			TrailingNewline: true,
		},
		JSON: JSONFormatConfig{
			Indent:          "  ",
			Compact:         false,
			SortKeys:        false,
			TrailingNewline: true,
		},
		Markdown: MarkdownFormatConfig{
			LineLength:     80,
			TrailingSpaces: false,
			ReferenceStyle: "collapsed",
			CodeBlockStyle: "fenced",
		},
	},
	Security: SecurityConfig{
		Timeout:            parseDurationDefault("5m"),
		Concurrency:        0,
		ConcurrencyPercent: 50,
		Tools:              []string{},
		Enable:             SecurityEnable{Code: true, Vuln: true, Secrets: false},
		ToolTimeouts:       SecurityToolTimeouts{Gosec: 0, Govulncheck: 0},
		TrackSuppressions:  false,
		FailOn:             "high",
	},
	Schema: SchemaConfig{
		Enable:     true,
		AutoDetect: false,
		Patterns:   []string{"schemas/**"},
		Types: SchemaTypes{
			JSONSchema: JSONSchemaOptions{Offline: true},
		},
	},
}

// LoadConfig loads configuration from various sources
func LoadConfig() (*Config, error) {
	v := viper.New()

	// Set defaults
	v.SetDefault("format.go.simplify", defaultConfig.Format.Go.Simplify)
	v.SetDefault("format.yaml.indent", defaultConfig.Format.YAML.Indent)
	v.SetDefault("format.yaml.line_length", defaultConfig.Format.YAML.LineLength)
	v.SetDefault("format.yaml.quote_style", defaultConfig.Format.YAML.QuoteStyle)
	v.SetDefault("format.yaml.trailing_newline", defaultConfig.Format.YAML.TrailingNewline)
	v.SetDefault("format.json.indent", defaultConfig.Format.JSON.Indent)
	v.SetDefault("format.json.compact", defaultConfig.Format.JSON.Compact)
	v.SetDefault("format.json.sort_keys", defaultConfig.Format.JSON.SortKeys)
	v.SetDefault("format.json.trailing_newline", defaultConfig.Format.JSON.TrailingNewline)
	v.SetDefault("format.markdown.line_length", defaultConfig.Format.Markdown.LineLength)
	v.SetDefault("format.markdown.trailing_spaces", defaultConfig.Format.Markdown.TrailingSpaces)
	v.SetDefault("format.markdown.reference_style", defaultConfig.Format.Markdown.ReferenceStyle)
	v.SetDefault("format.markdown.code_block_style", defaultConfig.Format.Markdown.CodeBlockStyle)

	// Security defaults
	v.SetDefault("security.timeout", defaultConfig.Security.Timeout)
	v.SetDefault("security.concurrency", defaultConfig.Security.Concurrency)
	v.SetDefault("security.concurrency_percent", defaultConfig.Security.ConcurrencyPercent)
	v.SetDefault("security.enable.code", defaultConfig.Security.Enable.Code)
	v.SetDefault("security.enable.vuln", defaultConfig.Security.Enable.Vuln)
	v.SetDefault("security.enable.secrets", defaultConfig.Security.Enable.Secrets)
	v.SetDefault("security.tools", defaultConfig.Security.Tools)
	v.SetDefault("security.tool_timeouts.gosec", defaultConfig.Security.ToolTimeouts.Gosec)
	v.SetDefault("security.tool_timeouts.govulncheck", defaultConfig.Security.ToolTimeouts.Govulncheck)
	v.SetDefault("security.track_suppressions", defaultConfig.Security.TrackSuppressions)
	v.SetDefault("security.fail_on", defaultConfig.Security.FailOn)
	v.SetDefault("security.exclude_fixtures", true)
	v.SetDefault("security.fixture_patterns", []string{"tests/fixtures/", "test-fixtures/"})

	// Schema defaults (preview)
	v.SetDefault("schema.enable", defaultConfig.Schema.Enable)
	v.SetDefault("schema.auto_detect", defaultConfig.Schema.AutoDetect)
	v.SetDefault("schema.patterns", defaultConfig.Schema.Patterns)
	v.SetDefault("schema.types.jsonschema.offline", defaultConfig.Schema.Types.JSONSchema.Offline)

	// Configuration file search paths
	v.SetConfigName("goneat")
	v.SetConfigType("yaml")
	v.AddConfigPath(".")     // Current directory
	v.AddConfigPath("$HOME") // Home directory

	// Add goneat home directory if available
	if configDir, err := GetConfigDir(); err == nil {
		v.AddConfigPath(configDir)
	}

	// Environment variables
	v.SetEnvPrefix("GONEAT")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	// Try to read config file (optional); ignore error to use defaults
	_ = v.ReadInConfig()

	var config Config
	if err := v.Unmarshal(&config); err != nil {
		return nil, fmt.Errorf("error unmarshaling config: %v", err)
	}

	return &config, nil
}

// LoadProjectConfig loads project-specific configuration
func LoadProjectConfig() (*Config, error) {
	// First load global config
	config, err := LoadConfig()
	if err != nil {
		return nil, err
	}

	// Look for project-specific config files
	projectConfigs := []string{
		".goneat.yaml",
		".goneat.yml",
		".goneat.json",
		"goneat.yaml",
		"goneat.yml",
		"goneat.json",
	}

	for _, configFile := range projectConfigs {
		if _, err := os.Stat(configFile); err == nil {
			v := viper.New()
			v.SetConfigFile(configFile)

			if err := v.ReadInConfig(); err != nil {
				continue // Try next config file
			}

			// Merge project config with global config
			if err := v.Unmarshal(config); err != nil {
				continue
			}

			break
		}
	}

	return config, nil
}

// GetGoConfig returns Go formatting configuration
func (c *Config) GetGoConfig() GoFormatConfig {
	return c.Format.Go
}

// GetYAMLConfig returns YAML formatting configuration
func (c *Config) GetYAMLConfig() YAMLFormatConfig {
	return c.Format.YAML
}

// GetJSONConfig returns JSON formatting configuration
func (c *Config) GetJSONConfig() JSONFormatConfig {
	return c.Format.JSON
}

// GetMarkdownConfig returns Markdown formatting configuration
func (c *Config) GetMarkdownConfig() MarkdownFormatConfig {
	return c.Format.Markdown
}

// SecurityConfig holds security scanning settings
type SecurityConfig struct {
	Timeout            time.Duration        `mapstructure:"timeout"`
	Concurrency        int                  `mapstructure:"concurrency"`
	ConcurrencyPercent int                  `mapstructure:"concurrency_percent"`
	Tools              []string             `mapstructure:"tools"`
	Enable             SecurityEnable       `mapstructure:"enable"`
	ToolTimeouts       SecurityToolTimeouts `mapstructure:"tool_timeouts"`
	TrackSuppressions  bool                 `mapstructure:"track_suppressions"`
	FailOn             string               `mapstructure:"fail_on"`
	ExcludeFixtures    bool                 `mapstructure:"exclude_fixtures"`
	FixturePatterns    []string             `mapstructure:"fixture_patterns"`
}

type SecurityEnable struct {
	Code    bool `mapstructure:"code"`
	Vuln    bool `mapstructure:"vuln"`
	Secrets bool `mapstructure:"secrets"`
}

type SecurityToolTimeouts struct {
	Gosec       time.Duration `mapstructure:"gosec"`
	Govulncheck time.Duration `mapstructure:"govulncheck"`
}

// GetSecurityConfig returns security configuration
func (c *Config) GetSecurityConfig() SecurityConfig { return c.Security }

// SchemaConfig holds schema validation configuration (preview)
type SchemaConfig struct {
	Enable     bool        `mapstructure:"enable"`
	AutoDetect bool        `mapstructure:"auto_detect"`
	Patterns   []string    `mapstructure:"patterns"`
	Types      SchemaTypes `mapstructure:"types"`
}

type SchemaTypes struct {
	JSONSchema JSONSchemaOptions `mapstructure:"jsonschema"`
}

type JSONSchemaOptions struct {
	Offline bool `mapstructure:"offline"`
}

// GetSchemaConfig returns schema configuration (preview)
func (c *Config) GetSchemaConfig() SchemaConfig { return c.Schema }

// parseDurationDefault is a helper to create default duration values from string literal
func parseDurationDefault(s string) time.Duration {
	d, err := time.ParseDuration(s)
	if err != nil {
		return 0
	}
	return d
}

// GetGoneatHome returns the goneat home directory
func GetGoneatHome() (string, error) {
	// Check environment variable first
	if home := os.Getenv("GONEAT_HOME"); home != "" {
		return home, nil
	}

	// Use standard dev tool convention: ~/.goneat
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get user home directory: %v", err)
	}

	return filepath.Join(homeDir, ".goneat"), nil
}

// EnsureGoneatHome creates the goneat home directory if it doesn't exist
func EnsureGoneatHome() (string, error) {
	homeDir, err := GetGoneatHome()
	if err != nil {
		return "", err
	}

	if err := os.MkdirAll(homeDir, 0750); err != nil {
		return "", fmt.Errorf("failed to create goneat home directory: %v", err)
	}

	return homeDir, nil
}

// GetScratchpadDir returns the scratchpad directory within the goneat home
func GetScratchpadDir() (string, error) {
	homeDir, err := EnsureGoneatHome()
	if err != nil {
		return "", err
	}

	scratchpadDir := filepath.Join(homeDir, "scratchpad")
	if err := os.MkdirAll(scratchpadDir, 0750); err != nil {
		return "", fmt.Errorf("failed to create scratchpad directory: %v", err)
	}

	return scratchpadDir, nil
}

// GetCacheDir returns the cache directory
func GetCacheDir() (string, error) {
	homeDir, err := EnsureGoneatHome()
	if err != nil {
		return "", err
	}
	cacheDir := filepath.Join(homeDir, "cache")
	if err := os.MkdirAll(cacheDir, 0750); err != nil {
		return "", fmt.Errorf("failed to create cache directory: %v", err)
	}
	return cacheDir, nil
}

// GetLogDir returns the log directory
func GetLogDir() (string, error) {
	homeDir, err := EnsureGoneatHome()
	if err != nil {
		return "", err
	}
	logDir := filepath.Join(homeDir, "logs")
	if err := os.MkdirAll(logDir, 0750); err != nil {
		return "", fmt.Errorf("failed to create log directory: %v", err)
	}
	return logDir, nil
}

// GetConfigDir returns the config directory
func GetConfigDir() (string, error) {
	homeDir, err := EnsureGoneatHome()
	if err != nil {
		return "", err
	}
	configDir := filepath.Join(homeDir, "config")
	if err := os.MkdirAll(configDir, 0750); err != nil {
		return "", fmt.Errorf("failed to create config directory: %v", err)
	}
	return configDir, nil
}

// GetWorkDir returns the work directory for temporary files
func GetWorkDir() (string, error) {
	homeDir, err := EnsureGoneatHome()
	if err != nil {
		return "", err
	}
	workDir := filepath.Join(homeDir, "work")
	if err := os.MkdirAll(workDir, 0750); err != nil {
		return "", fmt.Errorf("failed to create work directory: %v", err)
	}
	return workDir, nil
}

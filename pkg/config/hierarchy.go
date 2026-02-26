package config

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/viper"
)

// ConfigSource represents a source of configuration
type ConfigSource interface {
	// Load configuration from this source
	Load(ctx context.Context) (*viper.Viper, error)
	// Get the priority of this source (higher number = higher priority)
	Priority() int
	// Get a human-readable name for this source
	Name() string
}

// HierarchicalConfig manages configuration from multiple sources with precedence
type HierarchicalConfig struct {
	sources []ConfigSource
	cache   map[string]*configCache
	merger  ConfigMerger
}

type configCache struct {
	config    *viper.Viper
	timestamp time.Time
	ttl       time.Duration
}

// ConfigMerger defines how configurations are merged
type ConfigMerger interface {
	Merge(base, overlay *viper.Viper) (*viper.Viper, error)
}

// NewHierarchicalConfig creates a new hierarchical configuration manager
func NewHierarchicalConfig() *HierarchicalConfig {
	return &HierarchicalConfig{
		sources: make([]ConfigSource, 0),
		cache:   make(map[string]*configCache),
		merger:  &DefaultConfigMerger{},
	}
}

// AddSource adds a configuration source
func (h *HierarchicalConfig) AddSource(source ConfigSource) {
	h.sources = append(h.sources, source)
}

// Load loads configuration from all sources and merges them according to precedence
func (h *HierarchicalConfig) Load(ctx context.Context) (*Config, error) {
	// Sort sources by priority (ascending, so we apply lower priority first)
	sortedSources := make([]ConfigSource, len(h.sources))
	copy(sortedSources, h.sources)
	// Simple bubble sort for small number of sources
	for i := range sortedSources {
		for j := i + 1; j < len(sortedSources); j++ {
			if sortedSources[i].Priority() > sortedSources[j].Priority() {
				sortedSources[i], sortedSources[j] = sortedSources[j], sortedSources[i]
			}
		}
	}

	var merged *viper.Viper
	for _, source := range sortedSources {
		// Check cache
		if cached, ok := h.cache[source.Name()]; ok {
			if time.Since(cached.timestamp) < cached.ttl {
				if merged == nil {
					merged = cached.config
				} else {
					var err error
					merged, err = h.merger.Merge(merged, cached.config)
					if err != nil {
						return nil, fmt.Errorf("failed to merge config from %s: %w", source.Name(), err)
					}
				}
				continue
			}
		}

		// Load from source
		sourceConfig, err := source.Load(ctx)
		if err != nil {
			// Log warning but continue with other sources
			continue
		}

		// Cache the config
		h.cache[source.Name()] = &configCache{
			config:    sourceConfig,
			timestamp: time.Now(),
			ttl:       5 * time.Minute,
		}

		// Merge
		if merged == nil {
			merged = sourceConfig
		} else {
			merged, err = h.merger.Merge(merged, sourceConfig)
			if err != nil {
				return nil, fmt.Errorf("failed to merge config from %s: %w", source.Name(), err)
			}
		}
	}

	if merged == nil {
		// No configuration loaded, use defaults
		return &defaultConfig, nil
	}

	// Unmarshal into Config struct
	var config Config
	if err := merged.Unmarshal(&config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal merged config: %w", err)
	}

	return &config, nil
}

// DefaultConfigMerger implements a simple deep merge strategy
type DefaultConfigMerger struct{}

func (m *DefaultConfigMerger) Merge(base, overlay *viper.Viper) (*viper.Viper, error) {
	// Create new viper instance for merged config
	merged := viper.New()

	// Copy all settings from base
	for _, key := range base.AllKeys() {
		merged.Set(key, base.Get(key))
	}

	// Overlay settings from overlay
	for _, key := range overlay.AllKeys() {
		merged.Set(key, overlay.Get(key))
	}

	return merged, nil
}

// FileConfigSource loads configuration from a local file
type FileConfigSource struct {
	path     string
	priority int
}

func NewFileConfigSource(path string, priority int) *FileConfigSource {
	return &FileConfigSource{path: path, priority: priority}
}

func (s *FileConfigSource) Load(ctx context.Context) (*viper.Viper, error) {
	v := viper.New()
	v.SetConfigFile(s.path)

	if err := v.ReadInConfig(); err != nil {
		return nil, err
	}

	return v, nil
}

func (s *FileConfigSource) Priority() int {
	return s.priority
}

func (s *FileConfigSource) Name() string {
	return fmt.Sprintf("file:%s", s.path)
}

// HTTPConfigSource loads configuration from an HTTP endpoint
type HTTPConfigSource struct {
	url      string
	headers  map[string]string
	priority int
	client   *http.Client
}

func NewHTTPConfigSource(url string, priority int) *HTTPConfigSource {
	return &HTTPConfigSource{
		url:      url,
		priority: priority,
		headers:  make(map[string]string),
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (s *HTTPConfigSource) Load(ctx context.Context) (*viper.Viper, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", s.url, nil)
	if err != nil {
		return nil, err
	}

	for k, v := range s.headers {
		req.Header.Set(k, v)
	}

	resp, err := s.client.Do(req) // #nosec G704 - HTTP client for remote config; URL from validated config hierarchy
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close() //nolint:errcheck // Response body close errors are typically ignored in defer

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	v := viper.New()
	v.SetConfigType(detectConfigType(s.url))
	if err := v.ReadConfig(strings.NewReader(string(data))); err != nil {
		return nil, err
	}

	return v, nil
}

func (s *HTTPConfigSource) Priority() int {
	return s.priority
}

func (s *HTTPConfigSource) Name() string {
	return fmt.Sprintf("http:%s", s.url)
}

// S3ConfigSource loads configuration from S3
type S3ConfigSource struct {
	bucket   string
	key      string
	region   string
	priority int
}

func NewS3ConfigSource(bucket, key, region string, priority int) *S3ConfigSource {
	return &S3ConfigSource{
		bucket:   bucket,
		key:      key,
		region:   region,
		priority: priority,
	}
}

func (s *S3ConfigSource) Load(ctx context.Context) (*viper.Viper, error) {
	// Implementation would use AWS SDK
	// This is a placeholder
	return nil, fmt.Errorf("S3 config source not yet implemented")
}

func (s *S3ConfigSource) Priority() int {
	return s.priority
}

func (s *S3ConfigSource) Name() string {
	return fmt.Sprintf("s3://%s/%s", s.bucket, s.key)
}

// GitConfigSource loads configuration from a Git repository
type GitConfigSource struct {
	repo     string
	path     string
	ref      string
	priority int
}

func NewGitConfigSource(repo, path, ref string, priority int) *GitConfigSource {
	return &GitConfigSource{
		repo:     repo,
		path:     path,
		ref:      ref,
		priority: priority,
	}
}

func (s *GitConfigSource) Load(ctx context.Context) (*viper.Viper, error) {
	// Implementation would clone/pull repo and read file
	// This is a placeholder
	return nil, fmt.Errorf("git config source not yet implemented")
}

func (s *GitConfigSource) Priority() int {
	return s.priority
}

func (s *GitConfigSource) Name() string {
	return fmt.Sprintf("git:%s/%s@%s", s.repo, s.path, s.ref)
}

// EnvConfigSource loads configuration from environment variables
type EnvConfigSource struct {
	prefix   string
	priority int
}

func NewEnvConfigSource(prefix string, priority int) *EnvConfigSource {
	return &EnvConfigSource{
		prefix:   prefix,
		priority: priority,
	}
}

func (s *EnvConfigSource) Load(ctx context.Context) (*viper.Viper, error) {
	v := viper.New()
	v.SetEnvPrefix(s.prefix)
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	// Manually bind known environment variables
	// This is necessary because Viper doesn't automatically discover env vars
	knownEnvVars := []string{
		"LOG_LEVEL",
		"FORMAT_USE_GOIMPORTS",
		"FORMAT_FINALIZER_ENSURE_EOF",
		"FORMAT_FINALIZER_TRIM_TRAILING_SPACES",
		"FORMAT_FINALIZER_LINE_ENDINGS",
		"FORMAT_FINALIZER_REMOVE_BOM",
		"SECURITY_FAIL_ON",
		"HOOK_OUTPUT",
		"CACHE_DIR",
		"MAX_ISSUES_DISPLAY",
	}

	for _, envVar := range knownEnvVars {
		_ = v.BindEnv(strings.ToLower(strings.ReplaceAll(envVar, "_", ".")))
	}

	return v, nil
}

func (s *EnvConfigSource) Priority() int {
	return s.priority
}

func (s *EnvConfigSource) Name() string {
	return fmt.Sprintf("env:%s", s.prefix)
}

// detectConfigType attempts to detect config type from URL/path
func detectConfigType(path string) string {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".yaml", ".yml":
		return "yaml"
	case ".json":
		return "json"
	case ".toml":
		return "toml"
	default:
		// Try to detect from URL parameters
		if u, err := url.Parse(path); err == nil {
			if format := u.Query().Get("format"); format != "" {
				return format
			}
		}
		return "yaml" // default
	}
}

// LoadEnterpriseConfig loads configuration using enterprise hierarchy
func LoadEnterpriseConfig(ctx context.Context) (*Config, error) {
	h := NewHierarchicalConfig()

	// Priority levels (higher number = higher priority)
	const (
		PriorityDefault = 0
		PriorityOrg     = 10
		PriorityTeam    = 20
		PriorityProject = 30
		PriorityUser    = 40
		PriorityEnv     = 50
		PriorityCLI     = 60
	)

	// Add default configuration (built-in)
	// This would be implemented as a source that returns defaultConfig

	// Add organization config if URL is set
	if orgConfigURL := os.Getenv("GONEAT_ORG_CONFIG_URL"); orgConfigURL != "" {
		h.AddSource(NewHTTPConfigSource(orgConfigURL, PriorityOrg))
	}

	// Add team config if specified
	if teamConfig := os.Getenv("GONEAT_TEAM_CONFIG"); teamConfig != "" {
		if strings.HasPrefix(teamConfig, "git://") {
			// Parse git URL and add GitConfigSource
		} else if strings.HasPrefix(teamConfig, "http://") || strings.HasPrefix(teamConfig, "https://") {
			h.AddSource(NewHTTPConfigSource(teamConfig, PriorityTeam))
		}
	}

	// Add project config (local file)
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
			h.AddSource(NewFileConfigSource(configFile, PriorityProject))
			break
		}
	}

	// Add user config
	if homeDir, err := os.UserHomeDir(); err == nil {
		userConfig := filepath.Join(homeDir, ".goneat", "config.yaml")
		if _, err := os.Stat(userConfig); err == nil {
			h.AddSource(NewFileConfigSource(userConfig, PriorityUser))
		}
	}

	// Add environment variables (highest priority except CLI)
	h.AddSource(NewEnvConfigSource("GONEAT", PriorityEnv))

	// CLI flags would be handled separately and merged last

	return h.Load(ctx)
}

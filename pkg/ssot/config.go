package ssot

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/fulmenhq/goneat/pkg/logger"
	"gopkg.in/yaml.v3"
)

const (
	defaultConfigPath       = ".goneat/ssot-consumer.yaml"
	defaultLocalConfigPath  = ".goneat/ssot-consumer.local.yaml"
	legacyGoneatConfigPath  = ".goneat/ssot.yaml"
	legacyGoneatLocalConfig = ".goneat/ssot.local.yaml"
	legacyFuldxConfigPath   = ".fuldx/sync-consumer.yaml"
	legacyFuldxLocalConfig  = ".fuldx/sync-consumer.local.yaml"
)

// LoadSyncConfig loads the sync consumer configuration with local override support
// Priority:
//  1. .goneat/ssot.local.yaml (gitignored, for local dev)
//  2. .goneat/ssot.yaml (production config)
//  3. Environment variables (GONEAT_SSOT_{SOURCE}_LOCAL_PATH)
//  4. Convention-based (../{source-name} if exists)
func LoadSyncConfig() (*SyncConfig, error) {
	prodConfigPath := defaultConfigPath
	localConfigPath := defaultLocalConfigPath
	usingLegacy := ""

	// If the new config location does not exist, fall back to legacy files (with warning)
	if _, err := os.Stat(prodConfigPath); os.IsNotExist(err) {
		switch {
		case exists(legacyGoneatConfigPath):
			usingLegacy = legacyGoneatConfigPath
			prodConfigPath = legacyGoneatConfigPath
			localConfigPath = legacyGoneatLocalConfig
		case exists(legacyFuldxConfigPath):
			usingLegacy = legacyFuldxConfigPath
			prodConfigPath = legacyFuldxConfigPath
			localConfigPath = legacyFuldxLocalConfig
		default:
			return nil, fmt.Errorf("sync configuration not found: %s (run 'make bootstrap' or create %s)", defaultConfigPath, defaultConfigPath)
		}
	}

	// Load production config
	prodConfig, err := loadConfigFile(prodConfigPath, true)
	if err != nil {
		return nil, fmt.Errorf("failed to load production config: %w", err)
	}

	// Check for local override
	if _, err := os.Stat(localConfigPath); err == nil {
		// Local override exists - load and merge
		localConfig, err := loadConfigFile(localConfigPath, false)
		if err != nil {
			return nil, fmt.Errorf("failed to load local config override: %w", err)
		}

		// Merge local config over production config
		mergedConfig := mergeConfigs(prodConfig, localConfig)
		mergedConfig.isLocal = true
		config := applyEnvironmentOverrides(mergedConfig)
		if err := validateConfig(config); err != nil {
			return nil, fmt.Errorf("invalid configuration: %w", err)
		}
		return config, nil
	}

	config := applyEnvironmentOverrides(prodConfig)

	if usingLegacy != "" {
		logger.Warn(fmt.Sprintf("Using legacy SSOT consumer config location (%s). Please migrate to %s.", usingLegacy, defaultConfigPath))
	}

	if err := validateConfig(config); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return config, nil
}

// loadConfigFile loads and parses a YAML config file
func loadConfigFile(path string, validate bool) (*SyncConfig, error) {
	// Validate path to prevent directory traversal
	cleanPath := filepath.Clean(path)
	if filepath.IsAbs(cleanPath) {
		// If absolute path provided, ensure it's within safe boundaries
		// For now, we only support relative paths from project root
		return nil, fmt.Errorf("absolute paths not supported: %s", path)
	}

	data, err := os.ReadFile(cleanPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config SyncConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse YAML: %w", err)
	}

	// Validate required fields
	if validate {
		if err := validateConfig(&config); err != nil {
			return nil, fmt.Errorf("invalid configuration: %w", err)
		}
	}

	return &config, nil
}

// mergeConfigs merges local config overrides into production config
// Local config values take precedence over production config
func mergeConfigs(prod, local *SyncConfig) *SyncConfig {
	merged := &SyncConfig{
		Version:  prod.Version,
		Sources:  make([]Source, 0, len(prod.Sources)),
		Strategy: prod.Strategy,
	}

	// Override version if local specifies it
	if local.Version != "" {
		merged.Version = local.Version
	}

	// Merge sources by name
	sourceMap := make(map[string]Source)
	for _, source := range prod.Sources {
		sourceMap[source.Name] = source
	}

	// Apply local overrides
	for _, localSource := range local.Sources {
		if existing, ok := sourceMap[localSource.Name]; ok {
			// Merge fields (local takes precedence)
			if localSource.LocalPath != "" {
				existing.LocalPath = localSource.LocalPath
			}
			if localSource.Repo != "" {
				existing.Repo = localSource.Repo
			}
			if localSource.Ref != "" {
				existing.Ref = localSource.Ref
			}
			if localSource.SyncPathBase != "" {
				existing.SyncPathBase = localSource.SyncPathBase
			}
			if localSource.Output != "" {
				existing.Output = localSource.Output
			}
			if len(localSource.Keys) > 0 {
				existing.Keys = localSource.Keys
			}
			if len(localSource.Assets) > 0 {
				existing.Assets = localSource.Assets
			}
			sourceMap[localSource.Name] = existing
		} else {
			// Add new source from local config
			sourceMap[localSource.Name] = localSource
		}
	}

	// Convert map back to slice
	for _, source := range sourceMap {
		merged.Sources = append(merged.Sources, source)
	}

	// Merge strategy (local overrides if present)
	if local.Strategy.OnConflict != "" {
		merged.Strategy.OnConflict = local.Strategy.OnConflict
	}
	// For booleans, use local value if local config exists
	if len(local.Sources) > 0 {
		merged.Strategy.PruneStale = local.Strategy.PruneStale
		merged.Strategy.VerifyChecksums = local.Strategy.VerifyChecksums
	}

	return merged
}

// applyEnvironmentOverrides applies environment variable overrides
// Environment variables: GONEAT_SSOT_CONSUMER_{SOURCE}_LOCAL_PATH (legacy GONEAT_SSOT_{SOURCE}_LOCAL_PATH / GONEAT_{SOURCE}_LOCAL_PATH)
func applyEnvironmentOverrides(config *SyncConfig) *SyncConfig {
	for i := range config.Sources {
		source := &config.Sources[i]

		// Check for environment variable override
		envKey := strings.ToUpper(strings.ReplaceAll(source.Name, "-", "_"))
		if envPath := os.Getenv(fmt.Sprintf("GONEAT_SSOT_CONSUMER_%s_LOCAL_PATH", envKey)); envPath != "" {
			source.LocalPath = envPath
		} else if envPath := os.Getenv(fmt.Sprintf("GONEAT_SSOT_%s_LOCAL_PATH", envKey)); envPath != "" {
			source.LocalPath = envPath
		} else if source.LocalPath == "" {
			if legacy := os.Getenv(fmt.Sprintf("GONEAT_%s_LOCAL_PATH", envKey)); legacy != "" {
				source.LocalPath = legacy
			}
		}

		// Convention-based fallback: check for ../crucible (or ../source-name)
		if source.LocalPath == "" && source.Repo != "" {
			conventionalPath := filepath.Join("..", source.Name)
			if info, err := os.Stat(conventionalPath); err == nil && info.IsDir() {
				source.LocalPath = conventionalPath
			}
		}
	}

	if config.Strategy.OnConflict == "" {
		config.Strategy.OnConflict = "overwrite"
	}

	return config
}

// validateConfig validates the sync configuration
func validateConfig(config *SyncConfig) error {
	if config.Version == "" {
		return fmt.Errorf("version is required")
	}

	if len(config.Sources) == 0 {
		return fmt.Errorf("at least one source is required")
	}

	// Validate each source
	for i, source := range config.Sources {
		if source.Name == "" {
			return fmt.Errorf("sources[%d].name is required", i)
		}

		// Must have either localPath OR repo+ref
		hasLocal := source.LocalPath != ""
		hasRepo := source.Repo != "" || source.Ref != ""

		if !hasLocal && !hasRepo {
			return fmt.Errorf("sources[%d] (%s): must specify either 'localPath' or 'repo'", i, source.Name)
		}

		// If using repo, must have sync_path_base
		if hasRepo && source.SyncPathBase == "" {
			return fmt.Errorf("sources[%d] (%s): sync_path_base is required when using repo", i, source.Name)
		}

		// Must have assets to sync
		if len(source.Assets) == 0 {
			return fmt.Errorf("sources[%d] (%s): at least one asset is required", i, source.Name)
		}

		// Validate each asset
		for j, asset := range source.Assets {
			if asset.Type == "" {
				return fmt.Errorf("sources[%d].assets[%d]: type is required", i, j)
			}
			if asset.Subdir == "" {
				return fmt.Errorf("sources[%d].assets[%d]: subdir is required", i, j)
			}

			mode := asset.Mode
			if mode == "" {
				mode = "copy"
			}

			switch mode {
			case "copy":
				if len(asset.Paths) == 0 {
					return fmt.Errorf("sources[%d].assets[%d]: at least one path is required when mode=copy", i, j)
				}
				if asset.Link != "" {
					return fmt.Errorf("sources[%d].assets[%d]: link is not supported when mode=copy", i, j)
				}
			case "link":
				if asset.Link == "" {
					return fmt.Errorf("sources[%d].assets[%d]: link target is required when mode=link", i, j)
				}
				if len(asset.Paths) > 0 {
					return fmt.Errorf("sources[%d].assets[%d]: paths cannot be specified when mode=link", i, j)
				}
			default:
				return fmt.Errorf("sources[%d].assets[%d]: unsupported mode %q (expected copy or link)", i, j, asset.Mode)
			}
		}
	}

	return nil
}

// ValidateSource checks if the configured source exists
func ValidateSource(config *SyncConfig) error {
	for _, source := range config.Sources {
		resolved, err := ResolveSource(source)
		if err != nil {
			return fmt.Errorf("source %s: %w", source.Name, err)
		}

		// Check if resolved path exists
		info, err := os.Stat(resolved.Path)
		if os.IsNotExist(err) {
			if source.LocalPath != "" {
				return fmt.Errorf("source %s: local path not found: %s\n\nTo fix:\n  git clone git@github.com:%s.git %s", source.Name, resolved.Path, source.Repo, source.LocalPath)
			}
			return fmt.Errorf("source %s: path not found: %s", source.Name, resolved.Path)
		}
		if err != nil {
			return fmt.Errorf("source %s: failed to access path: %w", source.Name, err)
		}

		if !info.IsDir() {
			return fmt.Errorf("source %s: path is not a directory: %s", source.Name, resolved.Path)
		}
	}

	return nil
}

// ResolveSource resolves a source to a filesystem path
// For now, we only support localPath. In the future, we'll use go-git to clone repos.
func ResolveSource(source Source) (*ResolvedSource, error) {
	// If localPath is specified, use it directly
	if source.LocalPath != "" {
		fullPath := filepath.Join(source.LocalPath, source.SyncPathBase)
		return &ResolvedSource{
			Name:     source.Name,
			Path:     fullPath,
			IsLocal:  true,
			IsCloned: false,
		}, nil
	}

	// TODO: In future, use go-git to clone repo and checkout ref
	// For now, return error if no localPath
	envKey := strings.ToUpper(strings.ReplaceAll(source.Name, "-", "_"))
	return nil, fmt.Errorf("remote repository cloning not yet implemented (repo: %s, ref: %s). Use localPath or set GONEAT_SSOT_CONSUMER_%s_LOCAL_PATH environment variable", source.Repo, source.Ref, envKey)
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

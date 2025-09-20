---
title: Config Library
description: Configuration management and hierarchy system for Goneat applications.
---

# Config Library

Goneat's `pkg/config` provides a comprehensive configuration management system designed for complex applications that need hierarchical configuration, multiple file formats, and runtime validation.

## Purpose

The config library addresses common configuration challenges in Go applications:

- **Hierarchical Configuration**: Merge settings from multiple sources with clear precedence
- **Multiple Formats**: Support for YAML, JSON, and environment variables
- **Runtime Validation**: Schema-based validation of configuration values
- **Type Safety**: Strongly-typed configuration structures
- **Extensibility**: Easy to add new configuration sections and validation rules

## Key Features

- **Multi-source Configuration**: Environment variables, config files, command-line flags
- **Hierarchical Merging**: Clear precedence rules for conflicting settings
- **Schema Validation**: JSON Schema validation for configuration files
- **Type-safe Access**: Strongly-typed configuration structures
- **Hot Reloading**: Runtime configuration updates (when enabled)
- **Format Support**: YAML, JSON, and environment variable parsing
- **Validation**: Required fields, type checking, and custom validation rules

## Installation

```bash
go get github.com/fulmenhq/goneat/pkg/config
```

## Schema Validation

Goneat provides official schemas for configuration validation. Access them using:

```bash
# View available config schemas
goneat docs list | grep config

# View main configuration schema
goneat docs show schemas/config/v1.0.0/goneat-config

# View dates configuration schema
goneat docs show schemas/config/v1.0.0/dates

# Save schema for IDE integration
goneat docs show schemas/config/v1.0.0/goneat-config > goneat-config-schema.json
```

### Validating Custom Configurations

```go
package main

import (
    "fmt"
    "log"

    "github.com/fulmenhq/goneat/pkg/schema"
)

func validateConfig() {
    // Load your configuration data
    configData := []byte(`...your config.yaml content...`)

    // Validate against the official schema
    if err := schema.ValidateYAML(configData, "config/v1.0.0/goneat-config"); err != nil {
        log.Fatalf("Configuration validation failed: %v", err)
    }

    fmt.Println("✅ Configuration is valid!")
}
```

## Basic Usage

### Simple Configuration Loading

```go
package main

import (
    "context"
    "fmt"
    "log"

    "github.com/fulmenhq/goneat/pkg/config"
)

func main() {
    // Load configuration from default locations
    cfg, err := config.LoadDefault()
    if err != nil {
        log.Fatalf("Failed to load config: %v", err)
    }

    // Access configuration values
    fmt.Printf("Format enabled: %v\n", cfg.Format.Enabled)
    fmt.Printf("Security level: %s\n", cfg.Security.Level)
}
```

### Custom Configuration Structure

```go
// Define your configuration structure
type MyConfig struct {
    Database DatabaseConfig `mapstructure:"database"`
    API      APIConfig      `mapstructure:"api"`
    Features FeatureFlags   `mapstructure:"features"`
}

type DatabaseConfig struct {
    Host     string `mapstructure:"host"`
    Port     int    `mapstructure:"port"`
    Database string `mapstructure:"database"`
    SSLMode  string `mapstructure:"ssl_mode"`
}

type APIConfig struct {
    Port         int           `mapstructure:"port"`
    ReadTimeout  time.Duration `mapstructure:"read_timeout"`
    WriteTimeout time.Duration `mapstructure:"write_timeout"`
}

type FeatureFlags struct {
    NewFeature bool `mapstructure:"new_feature"`
    BetaMode   bool `mapstructure:"beta_mode"`
}

// Load custom configuration
func loadMyConfig() (*MyConfig, error) {
    var cfg MyConfig

    // Load from config file
    if err := config.LoadFromFile("config.yaml", &cfg); err != nil {
        return nil, fmt.Errorf("failed to load config: %w", err)
    }

    // Override with environment variables
    if err := config.LoadFromEnv("MYAPP", &cfg); err != nil {
        return nil, fmt.Errorf("failed to load env config: %w", err)
    }

    return &cfg, nil
}
```

## Configuration Hierarchy

The config library supports multiple configuration sources with clear precedence:

1. **Command-line flags** (highest precedence)
2. **Environment variables** (e.g., `MYAPP_DATABASE_HOST`)
3. **Configuration files** (YAML/JSON)
4. **Default values** (lowest precedence)

### Environment Variable Mapping

```go
// Environment variables are mapped using prefix + field path
// MYAPP_DATABASE_HOST -> Database.Host
// MYAPP_API_PORT -> API.Port
// MYAPP_FEATURES_NEW_FEATURE -> Features.NewFeature

cfg, err := config.LoadWithEnvPrefix("MYAPP", &myConfig)
```

## Advanced Features

### Schema Validation

```go
// Define JSON schema for validation
schema := `{
    "$schema": "https://json-schema.org/draft/2020-12/schema",
    "type": "object",
    "properties": {
        "database": {
            "type": "object",
            "properties": {
                "host": {"type": "string", "minLength": 1},
                "port": {"type": "integer", "minimum": 1, "maximum": 65535}
            },
            "required": ["host", "port"]
        }
    }
}`

// Validate configuration against schema
if err := config.ValidateWithSchema(cfg, schema); err != nil {
    log.Fatalf("Configuration validation failed: %v", err)
}
```

### Hierarchical Configuration Files

```go
// Load configuration with hierarchy
// 1. Load base configuration
// 2. Load environment-specific overrides
// 3. Load local overrides (ignored by git)

loader := config.NewHierarchicalLoader()
loader.AddPath("config/base.yaml")
loader.AddPath(fmt.Sprintf("config/%s.yaml", env))
loader.AddPath("config/local.yaml") // .gitignored

cfg, err := loader.Load(&myConfig)
```

### Schema Validation Integration

```go
// Validate configuration with schema
import "github.com/fulmenhq/goneat/pkg/schema"

// Load configuration data
configData, err := os.ReadFile("config.yaml")
if err != nil {
    log.Fatalf("Failed to read config: %v", err)
}

// Validate against embedded schema
if err := schema.ValidateYAML(configData, "config/v1.0.0/goneat-config"); err != nil {
    log.Fatalf("Schema validation failed: %v", err)
}

fmt.Println("✅ Configuration schema validation passed!")
```

### Hot Reloading

```go
// Enable configuration watching
watcher, err := config.NewWatcher("config.yaml")
if err != nil {
    log.Fatalf("Failed to create config watcher: %v", err)
}
defer watcher.Close()

// Watch for configuration changes
go func() {
    for {
        select {
        case <-watcher.Changes():
            // Reload configuration
            newCfg, err := config.LoadFromFile("config.yaml", &myConfig)
            if err != nil {
                log.Printf("Failed to reload config: %v", err)
                continue
            }
            // Update application configuration
            updateAppConfig(newCfg)
        case <-ctx.Done():
            return
        }
    }
}()
```

## API Reference

### Core Functions

#### `LoadDefault() (*Config, error)`

Loads configuration from standard locations with default settings.

#### `LoadFromFile(filename string, cfg interface{}) error`

Loads configuration from a specific file (YAML or JSON).

#### `LoadFromEnv(prefix string, cfg interface{}) error`

Loads configuration from environment variables with the specified prefix.

#### `LoadWithValidation(filename string, cfg interface{}, schema string) error`

Loads and validates configuration against a JSON schema.

### Configuration Types

#### `Config`

Main configuration structure containing all application settings.

```go
type Config struct {
    Format   FormatConfig   `mapstructure:"format"`
    Security SecurityConfig `mapstructure:"security"`
    Schema   SchemaConfig   `mapstructure:"schema"`
}
```

#### `FormatConfig`

Configuration for code formatting options.

```go
type FormatConfig struct {
    Go       GoFormatConfig       `mapstructure:"go"`
    YAML     YAMLFormatConfig     `mapstructure:"yaml"`
    JSON     JSONFormatConfig     `mapstructure:"json"`
    Markdown MarkdownFormatConfig `mapstructure:"markdown"`
}
```

## Best Practices

### Schema Validation

Always validate your configurations against official schemas:

```bash
# Validate main configuration
goneat validate data --schema config/v1.0.0/goneat-config --data config.yaml

# Validate dates configuration
goneat validate data --schema config/v1.0.0/dates --data .goneat/dates.yaml

# Integrate into CI/CD
echo "Validating configuration..."
goneat validate data --schema config/v1.0.0/goneat-config --data config.yaml || exit 1
```

### Configuration File Organization

```
config/
├── base.yaml          # Base configuration
├── development.yaml   # Development overrides
├── production.yaml    # Production overrides
└── local.yaml         # Local overrides (.gitignored)
```

### Environment Variable Naming

```go
// Good: Clear hierarchy
MYAPP_DATABASE_HOST=localhost
MYAPP_DATABASE_PORT=5432
MYAPP_API_TIMEOUT=30s

// Avoid: Unclear structure
DB_HOST=localhost
API_TIMEOUT=30
```

### Validation Strategy

```go
// Validate early in application startup
func init() {
    if err := validateConfig(); err != nil {
        log.Fatalf("Invalid configuration: %v", err)
    }
}

func validateConfig() error {
    // Check required fields
    if cfg.Database.Host == "" {
        return errors.New("database host is required")
    }

    // Validate value ranges
    if cfg.API.Port < 1 || cfg.API.Port > 65535 {
        return errors.New("API port must be between 1 and 65535")
    }

    return nil
}
```

## Error Handling

The config library provides detailed error messages for common issues:

```go
cfg, err := config.LoadFromFile("config.yaml", &myConfig)
if err != nil {
    switch {
    case errors.Is(err, config.ErrFileNotFound):
        log.Printf("Configuration file not found: %v", err)
    case errors.Is(err, config.ErrInvalidFormat):
        log.Printf("Invalid configuration format: %v", err)
    case errors.Is(err, config.ErrValidationFailed):
        log.Printf("Configuration validation failed: %v", err)
    default:
        log.Printf("Configuration error: %v", err)
    }
}
```

## Examples

### Complete Application Configuration

See the Goneat codebase for real-world examples:

- [`cmd/root.go`](https://github.com/fulmenhq/goneat/blob/main/cmd/root.go) - CLI configuration setup
- [`internal/config/config.go`](https://github.com/fulmenhq/goneat/blob/main/internal/config/config.go) - Internal configuration management
- [`pkg/config/config_test.go`](https://github.com/fulmenhq/goneat/blob/main/pkg/config/config_test.go) - Configuration testing patterns

### Custom Configuration with Validation

```go
package main

import (
    "github.com/fulmenhq/goneat/pkg/config"
    "github.com/fulmenhq/goneat/pkg/schema"
)

type AppConfig struct {
    Server ServerConfig `mapstructure:"server"`
    Database DatabaseConfig `mapstructure:"database"`
}

type ServerConfig struct {
    Host string `mapstructure:"host" validate:"required"`
    Port int    `mapstructure:"port" validate:"min=1,max=65535"`
}

type DatabaseConfig struct {
    DSN string `mapstructure:"dsn" validate:"required"`
    MaxConnections int `mapstructure:"max_connections" validate:"min=1"`
}

func main() {
    var cfg AppConfig

    // Load configuration
    if err := config.LoadFromFile("app.yaml", &cfg); err != nil {
        panic(err)
    }

    // Validate configuration
    if err := config.ValidateStruct(&cfg); err != nil {
        panic(err)
    }

    // Use configuration
    startServer(cfg.Server)
    connectDatabase(cfg.Database)
}
```

## Migration Guide

### From Viper

```go
// Before (using Viper directly)
v := viper.New()
v.SetConfigName("config")
v.AddConfigPath(".")
if err := v.ReadInConfig(); err != nil {
    return err
}

// After (using goneat config)
cfg, err := config.LoadDefault()
if err != nil {
    return err
}
```

### From Custom Config

```go
// Before (custom loading)
func loadConfig() (*Config, error) {
    file, err := os.Open("config.yaml")
    if err != nil {
        return nil, err
    }
    defer file.Close()

    var cfg Config
    if err := yaml.NewDecoder(file).Decode(&cfg); err != nil {
        return nil, err
    }

    return &cfg, nil
}

// After (using goneat config)
func loadConfig() (*Config, error) {
    return config.LoadFromFile("config.yaml", &Config{})
}
```

## Troubleshooting

### Common Issues

**Configuration file not found**

```bash
# Check if file exists
ls -la config.yaml

# Check file permissions
stat config.yaml
```

**Environment variables not loading**

```bash
# Check environment variable format
echo $MYAPP_DATABASE_HOST

# Verify prefix casing
env | grep MYAPP
```

**Validation errors**

```bash
# Check configuration file syntax
yamllint config.yaml

# Validate against schema
config validate --schema schema.json config.yaml
```

---

**Version:** 1.0.0
**Last Updated:** September 20, 2025</content>
</xai:function_call: write>
<parameter name="filePath">docs/appnotes/lib/config.md

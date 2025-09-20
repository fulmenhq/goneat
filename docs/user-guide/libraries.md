# Goneat Libraries

Goneat's `pkg/` directory contains reusable Go libraries that solve common developer experience (DX) problems. These libraries are designed for integration into your Go projects, reducing boilerplate and providing battle-tested patterns for CLI tools, build systems, and configuration-heavy applications.

## Getting Started

Import goneat libraries directly in your Go modules:

```bash
go get github.com/fulmenhq/goneat/pkg/config
go get github.com/fulmenhq/goneat/pkg/pathfinder
# ... etc
```

No separate installation is required—goneat's libraries are part of the main module and follow the same release cadence as the CLI tool.

## Available Libraries

| Library                                    | Purpose                            | Key Features                                                                | Use Cases                                                  |
| ------------------------------------------ | ---------------------------------- | --------------------------------------------------------------------------- | ---------------------------------------------------------- |
| [`config`](appnotes/lib/config.md)         | Hierarchical configuration loading | YAML/JSON support, schema validation, environment variables, fallbacks      | CLI apps, microservices, config-driven tools               |
| [`pathfinder`](appnotes/lib/pathfinder.md) | Safe file discovery and traversal  | Gitignore patterns, multi-module support, security boundaries, loaders      | Linters, build tools, file processors, scanners            |
| [`schema`](appnotes/lib/schema.md)         | JSON/YAML schema validation        | Offline validation, detailed error reporting, performance optimized         | Config validation, API contracts, data pipelines           |
| [`ignore`](appnotes/lib/ignore.md)         | Gitignore-style pattern matching   | High-performance matching, hierarchical patterns, negation support          | Build tools, file filters, deployment scripts              |
| [`safeio`](appnotes/lib/safeio.md)         | Secure file I/O operations         | Path traversal protection, permission boundaries, atomic writes, temp files | CLI tools, file processors, security-sensitive apps        |
| [`logger`](appnotes/lib/logger.md)         | Structured logging for CLIs        | STDOUT hygiene, level filtering, context propagation, JSON output           | CLI applications, background workers, API servers          |
| [`exitcode`](appnotes/lib/exitcode.md)     | Standardized CLI exit codes        | Semantic error categorization, Unix conventions, validation                 | CLI tools, shell scripts, CI/CD integration                |
| [`buildinfo`](appnotes/lib/buildinfo.md)   | Embedded build metadata            | Version embedding, release phase validation, dependency tracking            | Binary distribution, monitoring, release management        |
| [`versioning`](appnotes/lib/versioning.md) | Semantic version handling          | Full SemVer 2.0.0 support, ranges, phase integration, validation            | Package managers, release automation, compatibility checks |

## Guidelines

### Public API Stability

- **Stable (v0.2.x+)**: `config/`, `schema/`, `ignore/`, `safeio/`, `logger/`, `exitcode/`, `versioning/`
  - Follow semantic versioning
  - No breaking changes except in major releases
  - Backward compatibility guaranteed

- **Experimental (stabilizing in v0.3.0)**: `pathfinder/`, `buildinfo/`
  - API may change based on feedback
  - Use with caution in production
  - Track changes via GitHub issues

### Import Paths

All libraries are under the main module path:

```go
import (
    "github.com/fulmenhq/goneat/pkg/config"
    "github.com/fulmenhq/goneat/pkg/schema"
    // etc.
)
```

### Version Alignment

Libraries follow goneat's release cadence. Use the same version constraint for all goneat libraries:

```go
require github.com/fulmenhq/goneat v0.2.7
```

This ensures compatibility across the ecosystem.

### Contribution Policy

- **Bug fixes**: Always accepted, backported to stable releases
- **New features**: Require maintainer approval and tests
- **Breaking changes**: Only in major versions (v1.0.0+)
- **Documentation**: Always welcome, see [contribution guidelines](https://github.com/fulmenhq/goneat/blob/main/CONTRIBUTING.md)

## Integration Patterns

### CLI Application Template

```go
package main

import (
    "context"
    "os"

    "github.com/fulmenhq/goneat/pkg/buildinfo"
    "github.com/fulmenhq/goneat/pkg/config"
    "github.com/fulmenhq/goneat/pkg/exitcode"
    "github.com/fulmenhq/goneat/pkg/logger"
    "github.com/fulmenhq/goneat/pkg/safeio"
    "github.com/fulmenhq/goneat/pkg/versioning"
)

type Application struct {
    log      *logger.Logger
    config   *config.Config
    buildInfo *buildinfo.Info
    safeIO   *safeio.Context
}

func NewApplication(ctx context.Context) (*Application, exitcode.Code) {
    log := logger.New(ctx)
    ctx = logger.WithContext(ctx, log)

    // Load configuration with schema validation
    cfg, code := loadConfig(ctx)
    if code != exitcode.ExitSuccess {
        return nil, code
    }

    // Initialize build info
    bi := buildinfo.FromEmbedded()
    if err := bi.Validate(); err != nil {
        log.Warn("Build info validation warning", "error", err)
    }

    // Version compatibility check
    if err := validateAppVersion(cfg, bi); err != nil {
        log.Error("Version compatibility check failed", "error", err)
        return nil, exitcode.ErrConfiguration.Failure("version_check")
    }

    // Initialize safe I/O context
    safeIOCtx := safeio.NewContext(cfg.GetString("app.root_dir"))

    return &Application{
        log:      log,
        config:   cfg,
        buildInfo: bi,
        safeIO:   safeIOCtx,
    }, exitcode.ExitSuccess
}

func loadConfig(ctx context.Context) (*config.Config, exitcode.Code) {
    log := logger.FromContext(ctx)

    cfg, err := config.New(ctx,
        config.WithSchemaPath("config-schema.json"),
        config.WithFiles("app.yaml", ".app.yaml"),
    )
    if err != nil {
        log.Error("Failed to load configuration", "error", err)
        return nil, exitcode.ErrConfiguration.Failure("load")
    }

    // Validate against schema
    if err := cfg.Validate(); err != nil {
        log.Error("Configuration validation failed", "error", err)
        return nil, exitcode.ErrConfiguration.Failure("validation")
    }

    log.Info("Configuration loaded", "version", cfg.GetString("version"))
    return cfg, exitcode.ExitSuccess
}

func validateAppVersion(cfg *config.Config, bi *buildinfo.Info) error {
    minVersionStr := cfg.GetString("app.min_goneat_version")
    if minVersionStr == "" {
        return nil
    }

    minRange, err := versioning.ParseRange(minVersionStr)
    if err != nil {
        return fmt.Errorf("invalid min_goneat_version: %w", err)
    }

    appVer, err := versioning.Parse(bi.Version)
    if err != nil {
        return fmt.Errorf("cannot parse application version: %w", err)
    }

    if !minRange.Test(appVer) {
        return fmt.Errorf("application version %s does not satisfy minimum requirement %s",
            bi.Version, minVersionStr)
    }

    return nil
}

func (app *Application) Run(ctx context.Context) exitcode.Code {
    log := app.log

    log.Info("Application starting",
        "version", app.buildInfo.Version,
        "phase", app.buildInfo.ReleasePhase,
    )

    // Example: Safe file operation
    files, err := app.safeIO.ReadDir("data")
    if err != nil {
        log.Error("Failed to read data directory", "error", err)
        return exitcode.ErrIO.Failure()
    }

    log.Info("Processing files", "count", len(files))

    // Your application logic here...

    return exitcode.ExitSuccess
}

func main() {
    ctx := context.Background()
    app, code := NewApplication(ctx)
    if code != exitcode.ExitSuccess {
        os.Exit(int(code))
    }
    defer app.Close(ctx)

    code = app.Run(ctx)
    os.Exit(int(code))
}

func (app *Application) Close(ctx context.Context) exitcode.Code {
    // Cleanup logic
    return exitcode.ExitSuccess
}
```

### Build Tool Integration

```go
// Example: Custom build tool using goneat libraries
package main

import (
    "context"
    "github.com/fulmenhq/goneat/pkg/config"
    "github.com/fulmenhq/goneat/pkg/ignore"
    "github.com/fulmenhq/goneat/pkg/pathfinder"
    "github.com/fulmenhq/goneat/pkg/safeio"
    "github.com/fulmenhq/goneat/pkg/versioning"
)

type BuildTool struct {
    config   *config.Config
    ignore   *ignore.Matcher
    finder   *pathfinder.Walker
    safeIO   *safeio.Context
    version  *versioning.Version
}

func NewBuildTool(ctx context.Context, rootDir string) (*BuildTool, error) {
    // Load configuration
    cfg, err := config.New(ctx, config.WithFiles(".buildtool.yaml"))
    if err != nil {
        return nil, err
    }

    // Load ignore patterns
    ignorePatterns := cfg.GetStringSlice("build.ignore_patterns")
    ignoreMatcher, err := ignore.NewMatcher(ignorePatterns)
    if err != nil {
        return nil, err
    }

    // Initialize pathfinder
    finder := pathfinder.NewWalker(rootDir,
        pathfinder.WithIgnoreMatcher(ignoreMatcher),
        pathfinder.WithMaxDepth(10),
    )

    // Safe I/O context
    safeIOCtx := safeio.NewContext(rootDir)

    // Version from config or default
    verStr := cfg.GetString("buildtool.version")
    ver, err := versioning.ParseLenient(verStr)
    if err != nil {
        ver = versioning.MustParse("0.1.0")
    }

    return &BuildTool{
        config:   cfg,
        ignore:   ignoreMatcher,
        finder:   finder,
        safeIO:   safeIOCtx,
        version:  ver,
    }, nil
}

func (bt *BuildTool) Build(ctx context.Context) error {
    // Find source files
    sourceFiles := bt.findSourceFiles(ctx)

    // Process files safely
    for _, file := range sourceFiles {
        if err := bt.processFile(ctx, file); err != nil {
            return err
        }
    }

    return nil
}

func (bt *BuildTool) findSourceFiles(ctx context.Context) []string {
    var files []string
    bt.finder.Walk(func(path string, info os.FileInfo) error {
        if !info.IsDir() && bt.isSourceFile(path) {
            files = append(files, path)
        }
        return nil
    })
    return files
}

func (bt *BuildTool) isSourceFile(path string) bool {
    exts := bt.config.GetStringSlice("build.source_extensions")
    ext := filepath.Ext(path)
    for _, allowed := range exts {
        if ext == "."+allowed {
            return true
        }
    }
    return false
}

func (bt *BuildTool) processFile(ctx context.Context, path string) error {
    // Safe file reading
    content, err := bt.safeIO.ReadFile(path)
    if err != nil {
        return fmt.Errorf("failed to read %s: %w", path, err)
    }

    // Process content (compile, lint, etc.)
    processed, err := bt.compileContent(content, path)
    if err != nil {
        return fmt.Errorf("failed to process %s: %w", path, err)
    }

    // Safe file writing
    outputPath := filepath.Join(bt.config.GetString("build.output_dir"), filepath.Base(path))
    if err := bt.safeIO.WriteFileAtomic(outputPath, processed, 0644); err != nil {
        return fmt.Errorf("failed to write %s: %w", outputPath, err)
    }

    return nil
}

func (bt *BuildTool) compileContent(content []byte, path string) ([]byte, error) {
    // Implementation-specific compilation
    return content, nil // Placeholder
}
```

## Migration Guide

If you're using other versioning or configuration libraries, here's how to migrate:

### From Masterminds/semver

```go
// Old code
import "github.com/Masterminds/semver"

v, err := semver.NewVersion("1.0.0")
if err != nil { ... }
next, _ := v.IncMinor()

// New code
import "github.com/fulmenhq/goneat/pkg/versioning"

v, err := versioning.Parse("1.0.0")
if err != nil { ... }
next := v.BumpMinor()
```

### From spf13/viper (for config)

```go
// Old code
import "github.com/spf13/viper"

viper.SetConfigName("config")
viper.ReadInConfig()
version := viper.GetString("version")

// New code
import (
    "github.com/fulmenhq/goneat/pkg/config"
    "github.com/fulmenhq/goneat/pkg/versioning"
)

cfg, _ := config.New(ctx)
versionStr := cfg.GetString("version")
version, _ := versioning.Parse(versionStr)
```

## Troubleshooting

### Common Issues

1. **Import path errors**:

   ```go
   // ❌ Wrong
   import "github.com/fulmenhq/goneat/config"  // Missing /pkg/

   // ✅ Correct
   import "github.com/fulmenhq/goneat/pkg/config"
   ```

2. **Version mismatches**:

   ```go
   // Use consistent versions across all goneat libraries
   require github.com/fulmenhq/goneat v0.2.7  // Single constraint for all
   ```

3. **Schema validation failures**:
   - Ensure your schema files are accessible
   - Check file permissions with `safeio`
   - Use `schema.NewValidator` with error handling

4. **Path traversal errors**:
   - Always use `safeio.Context` for file operations
   - Validate paths before processing
   - Set appropriate root directories

### Debug Tips

```go
// Enable debug logging for troubleshooting
log.SetLevel(logger.LevelDebug)

// Validate configurations early
if err := cfg.Validate(); err != nil {
    log.Error("Config validation failed", "errors", err)
    os.Exit(1)
}

// Check version compatibility
if !req.Test(currentVersion) {
    log.Error("Version incompatibility",
        "required", req.Spec,
        "current", currentVersion,
    )
}
```

## Community and Support

- **Issues**: [GitHub Issues](https://github.com/fulmenhq/goneat/issues) (tag with `library`)
- **Discussions**: [GitHub Discussions](https://github.com/fulmenhq/goneat/discussions) (library usage)
- **Examples**: See `test-fixtures/` in the repository for integration examples
- **Documentation**: Each library has detailed [appnotes](appnotes/lib/) with code samples

## License

All goneat libraries are licensed under the Apache License 2.0. See [LICENSE](https://github.com/fulmenhq/goneat/blob/main/LICENSE) for details.

---

_Generated by Code Scout ([OpenCode](https://opencode.ai/)) under supervision of [@3leapsdave](https://github.com/3leapsdave)_  
_Co-Authored-By: Code Scout <noreply@3leaps.net>_

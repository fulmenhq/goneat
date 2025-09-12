# Ephemeral and Local Data Storage Standard

**Standard**: Guidelines for local data storage, cache management, and ephemeral file handling in goneat

## Overview

Goneat implements a developer tool pattern for local data storage, using the user's home directory with dot-prefixed directories. This standard defines the storage locations, data classification, and cleanup procedures for all local data managed by goneat.

## Storage Locations

### Primary User Data Directory

**Location**: `~/.goneat/`
**Purpose**: User-specific configuration, preferences, and persistent data
**Environment Override**: `GONEAT_HOME` environment variable

#### Directory Structure

```
~/.goneat/
├── .goneatignore          # User-global ignore patterns
├── config.yaml            # User configuration (future)
├── cache/                 # Ephemeral cache data
│   ├── schemas/          # Cached JSON schemas
│   └── reports/          # Cached assessment reports
├── logs/                  # Log files (if enabled)
└── temp/                  # Temporary working files
```

### Repository-Specific Data

**Location**: `{repo}/.goneat/`
**Purpose**: Project-specific configuration and artifacts
**Scope**: Per-repository, version controlled

#### Directory Structure

```
.goneat/
├── hooks.yaml            # Git hook configuration
├── hooks/                # Generated hook scripts
│   ├── pre-commit       # Generated pre-commit hook
│   └── pre-push         # Generated pre-push hook
├── dates.yaml            # Date validation configuration
├── reports/              # Assessment output storage
└── cache/               # Project-specific cache
```

## Data Classification

### Persistent Data (Committed to Git)

- `.goneatignore` - Repository ignore patterns
- `.goneat/hooks.yaml` - Hook configuration
- `.goneat/dates.yaml` - Date validation configuration
- Documentation and configuration files

### Ephemeral Data (Not Committed)

- Cache files (`~/.goneat/cache/*`)
- Temporary files (`~/.goneat/temp/*`)
- Generated hook scripts (`.goneat/hooks/*`)
- Assessment reports (`.goneat/reports/*`)

### Sensitive Data Handling

- No credentials or secrets stored in local directories
- All configuration files are plain text, human-readable
- Environment variables used for sensitive data (API keys, etc.)

## Directory Resolution Logic

### GONEAT_HOME Resolution Priority

1. **Environment Variable**: `GONEAT_HOME` (if set)
2. **Default Location**: `~/.goneat/` (user home directory)
3. **Fallback**: If home directory unavailable, use temporary directory

### Implementation

```go
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
```

## Data Lifecycle Management

### Cache Management

#### Cache Directory Structure

```
~/.goneat/cache/
├── schemas/          # JSON schema cache
│   ├── *.json       # Cached schemas with TTL
│   └── manifest.json # Cache index with timestamps
└── reports/          # Assessment report cache
    └── *.json       # Cached assessment results
```

#### Cache Expiration

- **Schema Cache**: 24 hours TTL
- **Report Cache**: 1 hour TTL
- **Automatic Cleanup**: On startup, remove expired cache files

### Temporary Files

#### Temporary Directory Structure

```
~/.goneat/temp/
├── work-*.tmp        # Temporary work files
├── report-*.json     # Temporary assessment reports
└── hook-*.sh        # Temporary hook scripts
```

#### Cleanup Procedures

- **Automatic**: Files older than 24 hours are cleaned on startup
- **Manual**: `goneat clean` command removes all temporary files
- **Emergency**: `rm -rf ~/.goneat/temp/*` for immediate cleanup

## Platform Considerations

### Operating System Support

#### macOS

- **Primary**: `~/Library/Application Support/goneat/` (future enterprise support)
- **Current**: `~/.goneat/` (developer tool pattern)
- **App Bundle**: N/A (CLI tool, not GUI application)

#### Windows

- **Primary**: `%APPDATA%\goneat\` (future enterprise support)
- **Current**: `%USERPROFILE%\.goneat\` (developer tool pattern)
- **Registry**: Not used (plain file storage only)

#### Linux

- **Primary**: `~/.local/share/goneat/` (future enterprise support)
- **Current**: `~/.goneat/` (developer tool pattern)
- **XDG Base Directory**: Follows XDG conventions for cache and config

### Cross-Platform Compatibility

- **Path Separators**: Automatic handling via `filepath.Join()`
- **Permissions**: Directory permissions set to `0750` (user read/write/execute, group read/execute)
- **File Permissions**: Standard `0644` for files, `0755` for executables
- **Case Sensitivity**: Assumes case-sensitive filesystem (standard for development)

## Environment Variable Reference

### GONEAT_HOME

**Purpose**: Override default goneat home directory location

**Usage**:

```bash
export GONEAT_HOME="/custom/path/to/goneat"
goneat assess
```

**Use Cases**:

- Enterprise deployments with custom directory structures
- Development environments with isolated configurations
- Testing with temporary directories

**Validation**:

- Must be absolute path
- Must be writable by current user
- Parent directory must exist

## Migration and Compatibility

### Version Compatibility

- **v0.1.x - v0.2.x**: Uses `~/.goneat/` exclusively
- **Future**: May support enterprise directory patterns per OS

### Data Migration

When changing storage locations:

1. **Copy Configuration**: Migrate user settings and preferences
2. **Update Cache**: Rebuild cache in new location
3. **Update References**: Update any hardcoded paths in documentation
4. **Test Compatibility**: Verify all features work with new location

### Backward Compatibility

- **Environment Override**: `GONEAT_HOME` works with any version
- **Fallback Logic**: If custom location fails, falls back to default
- **Graceful Degradation**: Missing directories are created automatically

## Security Considerations

### Directory Permissions

```bash
# Recommended permissions
~/.goneat/          # drwxr-x--- (0750)
├── config.yaml    # -rw-r----- (0640) - sensitive config
├── .goneatignore  # -rw-r--r-- (0644) - user preferences
└── cache/         # drwxr-x--- (0750) - ephemeral data
```

### Sensitive Data Protection

- **No Secrets**: Never store API keys, passwords, or tokens
- **Permissions**: Restrictive permissions on sensitive files
- **Encryption**: Plain text storage (no encryption at rest)
- **Cleanup**: Automatic cleanup of temporary sensitive data

### Network Security

- **Local Only**: All data stored locally, no network transmission
- **No Telemetry**: No automatic data collection or transmission
- **Cache Isolation**: Cache data never transmitted externally

## Troubleshooting

### Common Issues

#### Permission Denied

```bash
# Check directory permissions
ls -la ~/.goneat/

# Fix permissions
chmod 0750 ~/.goneat/
chmod 0644 ~/.goneat/config.yaml
```

#### Directory Not Found

```bash
# Create directory manually
mkdir -p ~/.goneat/cache
chmod 0750 ~/.goneat

# Or let goneat create it automatically
goneat assess  # Will create directories as needed
```

#### Custom Location Issues

```bash
# Verify custom location
export GONEAT_HOME="/custom/path"
mkdir -p "$GONEAT_HOME"
chmod 0750 "$GONEAT_HOME"

# Test with goneat
goneat assess
```

## Future Enhancements

### Planned Features

- **Enterprise Directory Support**: OS-specific application directories
- **Cache Compression**: Compressed storage for large cache files
- **Cache Synchronization**: Sync cache across development machines
- **Configuration Profiles**: Multiple named configuration profiles
- **Data Export/Import**: Backup and restore user data

### Extension Points

- **Plugin Storage**: Third-party plugins can store data in `~/.goneat/plugins/`
- **Workspace Config**: Project-specific overrides in `.goneat/config.yaml`
- **Team Config**: Shared configuration in `.goneat/team/`

---

**Status**: Active
**Version**: 1.0
**Last Updated**: September 10, 2025
**Related**: [Repository Operations SOP](../sop/repository-operations-sop.md)

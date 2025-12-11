# Config File Resolution Patterns

**Standard**: Three distinct config file resolution patterns used across goneat assessment runners to ensure consistent, predictable configuration loading behavior.

## 🎯 Purpose

Establish standardized patterns for config file discovery that:

- **Consistency** - Unified behavior across all assessment categories
- **Predictability** - Single files and directories resolve config the same way
- **Flexibility** - Support user customization, repo-specific settings, and tool-specific needs
- **Security** - Prevent path traversal attacks with proper validation

## 📋 The Three Patterns

### **Pattern 1: User-extensible-from-default**

_For category-specific goneat configurations (dates.yaml, security.yaml, etc.)_

**Search Order:**

1. `project/.goneat/{category}.yaml` (project-level override)
2. `GONEAT_HOME/config/{category}.yaml` (user-level default)
3. Built-in defaults (hardcoded in code)

**Behavior:**

- Only one config file is active (first found wins)
- Project settings override user settings override defaults
- Users can customize behavior globally or per-project

**Usage Example:**

```go
// Pattern 1: User-extensible config for dates assessment
resolver := NewConfigResolver(target)
configPath, found := resolver.ResolveConfigFile("dates")
// Looks for: ./project/.goneat/dates.yaml → ~/.goneat/config/dates.yaml → defaults
```

**File Examples:**

- `.goneat/dates.yaml` - Date validation rules
- `.goneat/security.yaml` - Security assessment settings
- `.goneat/assess.yaml` - Assessment overrides (e.g., yamllint scopes)

---

### **Pattern 2: Repo root only**

_For third-party tool configurations (.golangci.yml, .eslintrc, etc.)_

**Search Order:**

1. Working directory only (repo root for dirs, file's directory for single files)

**Behavior:**

- Tool finds its own configuration, not extensible through GONEAT_HOME
- Consistent with how the underlying tools work
- For single files: searches in the file's directory (not the file itself)

**Usage Example:**

```go
// Pattern 2: Repo root tool config for golangci-lint
resolver := NewConfigResolver(target)
workingDir := resolver.GetWorkingDir()
// Single file "pkg/foo.go" → search in "pkg/" directory
// Directory "." → search in current directory
```

**File Examples:**

- `.golangci.yml` - golangci-lint configuration
- `.eslintrc.js` - ESLint configuration
- `prettier.config.js` - Prettier configuration

---

### **Pattern 3: Hierarchical ignore files**

_For ignore patterns (.goneatignore, .gitignore style)_

**Search Order:**

1. Current location (closest to target)
2. Parent directories (walking up to repo root)
3. `GONEAT_HOME/.goneatignore` (user-level global ignore)

**Behavior:**

- All applicable files are active simultaneously
- Closer to target = higher precedence
- Follows .gitignore hierarchy conventions

**Usage Example:**

```go
// Pattern 3: Hierarchical ignore files
ignoreFiles := r.getIgnoreFiles(targetPath)
// Returns: ["./subdir/.goneatignore", "./.goneatignore", "~/.goneat/.goneatignore"]
// All files checked, closest takes precedence
```

**File Examples:**

- `.goneatignore` - Goneat-specific ignore patterns
- `.gitignore` - Git ignore patterns (when used by goneat)

## 🔧 Implementation

### ConfigResolver Utility

The standardized `ConfigResolver` in `internal/assess/runner.go` handles working directory resolution:

```go
type ConfigResolver struct {
    workingDir string
}

// For single files, uses file's directory as working directory
func NewConfigResolver(target string) *ConfigResolver {
    workingDir := target
    if info, err := os.Stat(target); err == nil && !info.IsDir() {
        workingDir = filepath.Dir(target) // File's directory
    }
    return &ConfigResolver{workingDir: workingDir}
}
```

### Pattern-Specific Methods

**Pattern 1**: `ResolveConfigFile(category)` - User-extensible configs
**Pattern 2**: `GetWorkingDir()` - Tool-specific repo configs
**Pattern 3**: `getIgnoreFiles(targetPath)` - Hierarchical ignore files

## 🎭 Usage by Assessment Runners

| Runner       | Pattern   | Config Files          | Purpose               |
| ------------ | --------- | --------------------- | --------------------- |
| **dates**    | Pattern 1 | `.goneat/dates.yaml`  | Date validation rules |
| **lint**     | Pattern 2 | `.golangci.yml`       | golangci-lint config  |
| **security** | Pattern 3 | `.goneatignore`       | Security scan ignores |
| **format**   | Pattern 2 | Tool-specific configs | Formatter settings    |

## ✅ Key Benefits

### Before Standardization

- **Inconsistent**: Single files vs directories behaved differently
- **Fragmented**: Each runner had custom config loading logic
- **Unpredictable**: Users couldn't rely on consistent behavior

### After Standardization

- **Uniform**: `goneat assess single-file.go` and `goneat assess .` use same logic
- **Predictable**: Clear search order documented and tested
- **Secure**: Path validation prevents traversal attacks
- **Maintainable**: Centralized config resolution logic

## 🔍 Troubleshooting

### Config Not Found?

1. **Pattern 1**: Check `.goneat/{category}.yaml` exists with correct YAML syntax
2. **Pattern 2**: Verify config is in working directory (file's dir for single files)
3. **Pattern 3**: Walk directory hierarchy to ensure ignore files are accessible

### Wrong Config Loaded?

- Use `--verbose` to see which config files are discovered
- Check file permissions (must be readable)
- Validate YAML/JSON syntax with `goneat validate data`

### Single File Issues?

- Remember: single files resolve config from file's **directory**, not the file itself
- `goneat assess /path/to/file.go` looks for config in `/path/to/`
- This matches user expectations and tool conventions

---

_This standard ensures consistent, predictable config file resolution across all goneat assessment runners while supporting flexible user customization patterns._

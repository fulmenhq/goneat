# Work Planning and Execution

Goneat provides sophisticated work planning capabilities that allow you to preview, analyze, and optimize your formatting operations before execution.

## Quick Start

### Preview Operations

```bash
# See what would be formatted in the current directory
goneat format --dry-run

# Preview specific folders
goneat format --dry-run --folders src/ tests/

# Check only Go files
goneat format --dry-run --types go
```

### Assessment Operations

```bash
# Run full task execution but don't modify any files
goneat format --no-op

# Assess specific folders without making changes
goneat format --no-op --folders src/ tests/

# Run assessment in parallel mode
goneat format --no-op --strategy parallel --folders .
```

### Generate Execution Plans

```bash
# Generate detailed execution plan
goneat format --plan-only --folders .

# Save plan to file for analysis
goneat format --plan-only --folders . --plan-file execution-plan.json

# View plan with grouping
goneat format --plan-only --folders . --group-by-type
```

## Execution Modes

Goneat provides different execution modes for various use cases:

### Dry Run Mode (`--dry-run`)
- **Purpose**: Preview what would happen without executing
- **Behavior**: Shows the work plan and stops
- **Use Case**: Understand impact before making changes
- **Example**: `goneat format --dry-run --folders src/`

### No-Op Mode (`--no-op`)
- **Purpose**: Full execution but no file modifications
- **Behavior**: Runs all tasks, validates formatting, but doesn't change files
- **Use Case**: Testing task runner, assessment, CI validation
- **Example**: `goneat format --no-op --folders src/`
- **Visual Indicator**: Log messages show `[NO-OP]` indicator

### Check Mode (`--check`)
- **Purpose**: Validate formatting without making changes
- **Behavior**: Similar to no-op but specific to format checking
- **Use Case**: CI/CD pipelines, pre-commit hooks
- **Example**: `goneat format --check --folders src/`

### Normal Mode (default)
- **Purpose**: Actually perform the formatting operations
- **Behavior**: Executes tasks and modifies files as needed
- **Use Case**: Standard formatting workflow
- **Example**: `goneat format --folders src/`

## Work Planning Features

### File Discovery and Filtering

Goneat automatically discovers supported files and applies intelligent filtering:

```bash
# Process specific directories
goneat format --folders src/ pkg/ internal/

# Filter by content type
goneat format --types go,yaml,json

# Limit directory depth
goneat format --max-depth 3

# Combine filters
goneat format --folders src/ --types go --max-depth 2
```

### Redundancy Elimination

Goneat automatically detects and eliminates redundant paths:

```bash
# These are treated as a single operation
goneat format --folders src/ src/internal/ src/pkg/
# Result: src/ encompasses the others, redundancies eliminated
```

### Work Organization

Control how work is organized for optimal execution:

```bash
# Group by content type (recommended for mixed projects)
goneat format --group-by-type

# Group by file size (recommended for large projects)
goneat format --group-by-size

# Default: single group for simple cases
goneat format
```

## Understanding Work Plans

### Work Manifest Structure

When you use `--plan-only`, Goneat generates a structured manifest:

```json
{
  "plan": {
    "command": "format",
    "total_files": 150,
    "filtered_files": 45,
    "execution_strategy": "sequential"
  },
  "work_items": [
    {
      "path": "src/main.go",
      "content_type": "go",
      "size": 2048,
      "estimated_time": 0.5
    }
  ],
  "groups": [
    {
      "name": "Go Files",
      "recommended_parallelization": 4
    }
  ]
}
```

### Execution Time Estimates

Goneat provides realistic time estimates:

```bash
goneat format --plan-only --folders large-project/
# Output:
# 📊 Summary:
#   Total files discovered: 500
#   Files after filtering: 200
#
# ⏱️  Estimated Execution Times:
#   Sequential: 45.2s
#   Parallel (4 workers): 12.3s
#   Parallel (8 workers): 7.1s
```

## Advanced Usage

### CI/CD Integration

```bash
# Generate plan for CI analysis
goneat format --plan-only --folders . --plan-file plan.json

# Check if formatting is needed (exit code indicates status)
goneat format --check --folders src/
# Exit code 0: all files formatted
# Exit code 1: files need formatting

# Run assessment in CI without making changes
goneat format --no-op --folders src/
# Exit code 0: all tasks completed successfully
# Non-zero exit code: some tasks failed

# Apply formatting in CI
goneat format --folders src/
```

### Large Project Optimization

```bash
# Optimize for large monorepos
goneat format --group-by-size --folders . --max-depth 5

# Focus on specific areas
goneat format --folders src/ --types go,yaml

# Get detailed statistics
goneat format --plan-only --folders . --verbose
```

### Custom Workflows

```bash
# Process only recently changed files
goneat format --files $(git diff --name-only)

# Format specific file types in specific locations
goneat format --folders src/ --types go --max-depth 3

# Preview changes before applying
goneat format --dry-run --folders . --group-by-type

# Test task runner on large codebase without making changes
goneat format --no-op --folders . --strategy parallel --max-workers 8

# Assess formatting status across entire monorepo
goneat format --no-op --folders . --types go,yaml,json --group-by-type
```

## Best Practices

### For Development Teams

1. **Use `--dry-run` regularly** to understand what will be processed
2. **Use `--no-op` for testing** task runner on large codebases without risk
3. **Leverage `--group-by-type`** for projects with mixed languages
4. **Use `--plan-file`** to archive execution plans for analysis
5. **Set up CI checks** using `--check` or `--no-op` mode

### For Large Projects

1. **Use `--group-by-size`** to optimize parallelization
2. **Limit `--max-depth`** to avoid processing test fixtures
3. **Filter by `--types`** to focus on specific languages
4. **Use `--plan-only`** to analyze before large operations

### For CI/CD

```yaml
# .github/workflows/format.yml
name: Format Check
on: [pull_request]

jobs:
  format:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      - name: Check formatting
        run: |
          goneat format --check --folders src/ pkg/
          if [ $? -ne 0 ]; then
            echo "Files need formatting. Run: goneat format --folders src/ pkg/"
            exit 1
          fi
```

## Troubleshooting

### Common Issues

**"No files found"**
```bash
# Check what files exist
find . -name "*.go" -o -name "*.yaml" | head -10

# Try without filters
goneat format --plan-only --folders .

# Check supported types
goneat format --types go,yaml,json,markdown
```

**"Too many files"**
```bash
# Limit scope
goneat format --max-depth 3 --folders src/

# Filter by type
goneat format --types go --folders .

# Use specific files
goneat format --files src/main.go src/utils.go
```

**"Slow performance"**
```bash
# Use parallelization
goneat format --group-by-size --folders .

# Limit depth
goneat format --max-depth 4 --folders .

# Focus on specific types
goneat format --types go --folders .
```

### Performance Tuning

```bash
# Get performance insights
goneat format --plan-only --folders . --verbose

# Compare strategies
goneat format --plan-only --folders . --group-by-size
goneat format --plan-only --folders . --group-by-type

# Optimize for your use case
goneat format --folders src/ --types go,yaml --max-depth 3
```

## Integration Examples

### With Git Hooks

```bash
#!/bin/sh
# .git/hooks/pre-commit

# Check formatting before commit
goneat format --check --folders src/
if [ $? -ne 0 ]; then
    echo "Pre-commit hook: files need formatting"
    echo "Run: goneat format --folders src/"
    exit 1
fi
```

### With Make

```makefile
.PHONY: format format-check format-plan

format:
	goneat format --folders src/ pkg/

format-check:
	goneat format --check --folders src/ pkg/

format-plan:
	goneat format --plan-only --folders src/ pkg/ --plan-file format-plan.json
```

### With Scripts

```bash
#!/bin/bash
# format-project.sh

PROJECT_DIRS="src pkg internal"
TYPES="go,yaml,json"

echo "=== Formatting Project ==="
echo "Directories: $PROJECT_DIRS"
echo "Types: $TYPES"
echo ""

# Show plan
goneat format --plan-only --folders $PROJECT_DIRS --types $TYPES

# Confirm
read -p "Proceed with formatting? (y/N) " -n 1 -r
echo
if [[ $REPLY =~ ^[Yy]$ ]]; then
    goneat format --folders $PROJECT_DIRS --types $TYPES
fi
```

This work planning system gives you complete control and visibility into Goneat's operations, making it perfect for both development workflows and automated CI/CD pipelines.
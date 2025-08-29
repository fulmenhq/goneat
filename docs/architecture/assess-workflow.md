# Assess Workflow Architecture

**Author:** Code Scout (Task Execution & Assessment Expert)
**Date:** 2025-08-28
**Purpose:** Comprehensive codebase assessment and workflow planning system

## Vision

The `assess` command serves as a "command of commands" - a unified entry point for comprehensive codebase analysis that:

- **Analyzes** codebases using all available formatting, linting, and analysis tools
- **Prioritizes** issues based on expert developer knowledge or user preferences
- **Plans** remediation workflows with time estimates and parallelization opportunities
- **Outputs** structured reports in both human-readable (Markdown/HTML) and machine-readable (JSON) formats
- **Integrates** with pre-commit hooks, CI/CD pipelines, and agentic workflows
- **Uses a JSON-first pipeline** so all downstream formats (HTML, Markdown) are derived from a single source of truth

## Core Principles

### 1. Unified Assessment
Single command that orchestrates multiple analysis tools vs. running them separately.

### 2. Intelligent Prioritization
Expert-driven defaults with user customization to prevent analysis paralysis.

### 3. Workflow Planning
Not just issue detection, but actionable remediation plans with time estimates.

### 4. Dual Output Formats (JSON-first)
- **JSON**: Canonical; used for automation and to feed HTML/Markdown
- **HTML/Markdown**: Human-friendly, rendered from the same JSON data

### 5. Parallelization Awareness
Identify independent task groups for efficient remediation.

## Concurrency Execution Model

### Worker-Pool for Category Runners
- Assessments across categories (format, static-analysis, lint, etc.) run via a bounded worker-pool.
- Default worker count = 50% of CPU cores (min 1), configurable by flags:
  - `--concurrency <int>`: explicit worker count
  - `--concurrency-percent <int>`: percentage of CPU cores (1-100); used when `--concurrency` is not set
- Failures in any category are recorded per-category; the run continues and final status respects `--fail-on`.

### Rationale
- Categories have different run-time profiles; overlapping them reduces wall-time.
- Simpler, predictable scheduling with bounded concurrency.

### Parallelization in Workflow Plan
- Post-run, issues are grouped by file to suggest parallel groups developers can execute concurrently.
- The plan includes phase-level parallel groups for human execution, distinct from runtime concurrency.

## Logging & Metrics

### Runtime Summary
- Logs include:
  - Workers used and total categories
  - Per-category runtimes (format, static-analysis, lint, …)
  - Total wall-time and total issues discovered

Example:
```
workers=6, categories=3
Runtime: format           115ms
Runtime: static-analysis  812ms
Runtime: lint             1.067s
Total wall-time:          1.067s; total issues: 4
```

### HTML Report Improvements
- Repo name shown prominently, with user-shortened path (~/…)
- Version inferred from `VERSION` or version source file where available
- File-grouped, collapsible issue lists for readability at scale

## Assessment Categories

### Primary Categories (Assessment Areas)

```go
type AssessmentCategory string

const (
    CategoryFormat        AssessmentCategory = "format"
    CategoryLint          AssessmentCategory = "lint"
    CategorySecurity      AssessmentCategory = "security"
    CategoryPerformance   AssessmentCategory = "performance"
    CategoryStaticAnalysis AssessmentCategory = "static-analysis"
)
```

### Secondary Categories (Sub-areas)

#### Format
- `whitespace`: Indentation, trailing spaces, line endings
- `imports`: Import organization and grouping
- `structure`: Code structure and organization

#### Lint
- `style`: Code style and conventions
- `best-practices`: Language-specific best practices
- `consistency`: Code consistency within project

#### Security
- `vulnerability`: Known security vulnerabilities
- `leakage`: Information disclosure risks
- `edge-cases`: Input validation and boundary conditions

#### Performance
- `runtime`: Runtime performance issues
- `memory`: Memory usage optimization
- `concurrency`: Concurrent execution issues

#### Complexity
- `cyclomatic`: Code complexity metrics
- `maintainability`: Code maintainability scores
- `readability`: Code readability assessment

#### Dependencies
- `security`: Vulnerable dependencies
- `updates`: Outdated dependencies
- `compatibility`: Dependency compatibility issues

#### Documentation
- `completeness`: Missing documentation
- `accuracy`: Documentation accuracy
- `consistency`: Documentation consistency

#### Testing
- `coverage`: Test coverage metrics
- `quality`: Test quality assessment
- `integration`: Integration test coverage

## Command Classification

### In Scope (Neat Commands)
Commands that perform code analysis, formatting, or quality assessment:

```go
// Commands that register for assessment
var NeatCommands = []string{
    "format",      // Code formatting
    "lint",        // Code quality analysis
    "check",       // General validation
    "security",    // Security analysis
    "analyze",     // Static analysis
    "test",        // Test execution and coverage
}
```

### Out of Scope
- **Support Commands**: `envinfo`, `help`, `version`
- **Utility Commands**: `version` (management), `init`, `config`
- **Informational**: Status displays, logging, debugging

## Priority Matrix

### Default Priority Order
Based on typical developer workflow and issue characteristics:

```go
var DefaultPriorities = map[AssessmentCategory]int{
    CategoryFormat:        1, // Quick wins, often auto-fixable
    CategorySecurity:      2, // Critical issues, block progress
    CategoryStaticAnalysis: 3, // Code correctness, potential bugs
    CategoryLint:          4, // Code quality, variable effort
    CategoryPerformance:   5, // Optimization, may be deferred
}
```

### Priority Characteristics
- **Format**: Usually quick, low cognitive load, auto-fixable
- **Security**: Critical blockers, immediate attention required
- **Static Analysis**: Code correctness, potential bugs (go vet, etc.)
- **Lint**: Variable effort, some auto-fixable, some require thought
- **Performance**: Optimization opportunities, may be deferred

## Output Formats

### Markdown Report Structure

```markdown
# Codebase Assessment Report
**Generated:** 2025-08-28T10:30:00Z
**Tool:** goneat assess
**Target:** /path/to/project

## Executive Summary
- **Overall Health:** 🟢 Good (85% compliant)
- **Critical Issues:** 0
- **Estimated Fix Time:** 2-3 hours
- **Parallelizable Tasks:** 3 groups identified

## Assessment Results

### 🔧 Format Issues (Priority: 1)
**Status:** ⚠️ 3 issues found
**Estimated Time:** 15 minutes
**Parallelizable:** Yes (3 independent files)

| File | Issues | Severity | Auto-fixable |
|------|--------|----------|--------------|
| src/main.go | 2 | Low | Yes |
| pkg/utils.go | 1 | Low | Yes |

### 🛡️ Security Issues (Priority: 2)
**Status:** ✅ No issues found

## Recommended Workflow
1. **Phase 1 (15 min)**: Auto-fix all format issues
2. **Phase 2 (30 min)**: Address critical lint issues
3. **Phase 3 (45 min)**: Review remaining items

## Parallelization Opportunities
- **Group A**: Files with only format issues (3 files)
- **Group B**: Files with format + simple lint (2 files)
- **Group C**: Complex refactoring needed (1 file)
```

### JSON Schema Structure

```json
{
  "$schema": "https://3leaps.net/schemas/goneat-assessment-v1.0.0.json",
  "metadata": {
    "generated": "2025-08-28T10:30:00Z",
    "tool": "goneat",
    "version": "1.0.0",
    "target": "/path/to/project"
  },
  "summary": {
    "overall_health": 0.85,
    "critical_issues": 0,
    "estimated_time_minutes": 120,
    "parallel_groups": 3
  },
  "categories": {
    "format": {
      "priority": 1,
      "issues_count": 3,
      "estimated_time": 15,
      "parallelizable": true,
      "issues": [
        {
          "file": "src/main.go",
          "line": 42,
          "column": 5,
          "severity": "low",
          "message": "Incorrect indentation",
          "auto_fixable": true,
          "category": "whitespace"
        }
      ]
    }
  },
  "workflow": {
    "phases": [
      {
        "name": "Phase 1",
        "estimated_time": 15,
        "description": "Auto-fix all format issues",
        "parallel_groups": ["group_a"]
      }
    ],
    "parallel_groups": {
      "group_a": {
        "files": ["src/main.go", "pkg/utils.go"],
        "categories": ["format"],
        "estimated_time": 10
      }
    }
  }
}
```

## Code Architecture

### Command Registration System

```go
// internal/assess/registry.go
type CommandRegistration struct {
    Name         string
    Category     AssessmentCategory
    SubCategory  string
    Priority     int
    Description  string
    Runner       AssessmentRunner
}

type AssessmentRunner interface {
    Assess(target string, config Config) (*AssessmentResult, error)
    EstimateTime(issues []Issue) time.Duration
    CanParallelize(issues []Issue) bool
}

// Commands register themselves
func RegisterCommand(reg CommandRegistration) {
    // Add to registry
}
```

### Assessment Engine

```go
// internal/assess/engine.go
type AssessmentEngine struct {
    registry   *CommandRegistry
    prioritizer *Prioritizer
    planner    *WorkflowPlanner
}

func (e *AssessmentEngine) RunAssessment(target string, config Config) (*AssessmentReport, error) {
    // 1. Discover registered commands
    // 2. Run assessments in priority order
    // 3. Analyze results and estimate times
    // 4. Identify parallelization opportunities
    // 5. Generate reports in requested formats
}
```

## Integration Points

### Pre-commit/Pre-push Hooks

```yaml
# .pre-commit-config.yaml
- id: goneat-assess
  name: goneat assessment
  entry: goneat assess --format markdown --fail-on critical
  language: system
  files: \.(go|py|js|ts)$
  pass_filenames: false
```

### CI/CD Integration

```yaml
# .github/workflows/assess.yml
- name: Codebase Assessment
  run: |
    goneat assess --format both --output assessment/
    # Upload reports as artifacts
```

### Agentic Integration

```json
// For AI agents and automation tools
{
  "assessment": {
    "actionable_items": [...],
    "parallel_groups": [...],
    "estimated_effort": "..."
  }
}
```

## Implementation Phases

### Phase 1: Foundation (Current → 2 weeks)
1. **Schema Design**: Define JSON schema and markdown template
2. **Core Assessment Engine**: Framework for running multiple tools
3. **Format Integration**: Start with existing format capabilities
4. **Basic Prioritization**: Simple expert-driven ordering

### Phase 2: Intelligence (2-4 weeks)
1. **Work Estimation Engine**: Time prediction based on issue patterns
2. **Parallelization Analysis**: Identify independent task groups
3. **User Customization**: Priority override system
4. **Pre-commit Integration**: Hook generation capabilities

### Phase 3: Ecosystem (4-6 weeks)
1. **Security Integration**: Static analysis tools
2. **Complexity Metrics**: Code maintainability scoring
3. **Performance Analysis**: Runtime optimization suggestions
4. **CI/CD Integration**: Pipeline assessment capabilities

## Success Metrics

### Functional Completeness
- ✅ **Unified Assessment**: Single command for comprehensive analysis
- ✅ **Structured Outputs**: Both markdown and JSON formats
- ✅ **Intelligent Prioritization**: Expert-driven with user customization
- ✅ **Workflow Planning**: Time estimates and parallelization
- ✅ **Integration Ready**: Pre-commit, CI/CD, and agentic support

### Quality Metrics
- ✅ **Performance**: Efficient analysis without significant overhead
- ✅ **Accuracy**: Reliable issue detection and categorization
- ✅ **Usability**: Clear, actionable reports for developers
- ✅ **Extensibility**: Easy addition of new assessment categories

---

**Status**: Initial architecture defined, ready for schema implementation and testing with format command
**Next**: Refine categories and command classification, then implement core schemas
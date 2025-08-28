# Work Planning and Parallelization Architecture

## Overview

Goneat implements a sophisticated work planning and parallelization system that provides predictable, efficient, and scalable execution of formatting and validation operations across large codebases.

## Core Concepts

### Work Manifest

A **work manifest** is a structured JSON document that describes a complete execution plan for a command. It contains:

- **Plan metadata**: Command, timestamp, working directory, execution strategy
- **Work items**: Individual files to process with metadata
- **Groups**: Logical groupings of work items (by size, type, etc.)
- **Statistics**: Analysis of the workload and performance estimates

### Work Planning Process

1. **Discovery**: Scan directories for supported files
2. **Filtering**: Apply user-specified filters (types, patterns, depth)
3. **Analysis**: Calculate file sizes, types, and processing estimates
4. **Optimization**: Eliminate redundant paths and optimize grouping
5. **Manifest Generation**: Create structured execution plan

### Execution Strategies

#### Sequential Execution
- Process work items one by one
- Simple, predictable, good for debugging
- Baseline for performance comparisons

#### Parallel Execution
- Multiple workers process items simultaneously
- Optimized grouping for efficiency
- Configurable worker limits

#### No-Op Execution
- Full execution pipeline but no file modifications
- Validates all operations without side effects
- Perfect for testing, assessment, and CI validation
- Maintains all performance characteristics of normal execution

## Architecture Components

### Work Planner (`pkg/work/planner.go`)

Responsible for:
- File system traversal and discovery
- Work item creation and metadata collection
- Redundancy elimination
- Grouping strategy application
- Statistical analysis and estimation

### Work Manifest (`schemas/work/work-manifest-v1.0.0.yaml`)

JSON Schema defining the structure of work plans:
- Versioned for compatibility
- Comprehensive validation rules
- Extensible for future features

### Processor Interface Extensions

The `WorkItemProcessor` interface supports multiple execution modes:

```go
type WorkItemProcessor interface {
    ProcessWorkItem(ctx context.Context, item *WorkItem, dryRun bool, noOp bool) ExecutionResult
}
```

- **Normal Mode** (`dryRun=false, noOp=false`): Full execution with file modifications
- **Dry Run Mode** (`dryRun=true, noOp=false`): Validation without execution
- **No-Op Mode** (`dryRun=false, noOp=true`): Full execution pipeline but no file changes

### Command Integration

Format command supports:
- `--dry-run`: Preview execution plan
- `--plan-only`: Generate manifest without execution
- `--plan-file`: Save manifest to file
- `--folders`: Specify target directories
- `--types`: Filter by content types
- `--group-by-size/type`: Control work organization
- `--no-op`: Execute tasks without making changes (assessment mode)

## Parallelization Strategy

### Worker Pool Design

- **Size-based pools**: Large files → dedicated workers
- **Content-type pools**: Specialized workers for different formats
- **Dynamic scaling**: Adjust worker count based on workload

### Execution Flow

1. **Planning Phase**: Generate work manifest
2. **Dispatch Phase**: Route work items to appropriate workers
3. **Execution Phase**: Process items with progress tracking
4. **Aggregation Phase**: Collect results and generate reports

### Resource Management

- **CPU limits**: Respect `--max-workers` or system limits
- **Memory management**: Stream large files, limit concurrent operations
- **I/O optimization**: Batch operations, minimize disk seeks

## User Experience

### Predictability

```bash
# See exactly what will happen
goneat format --dry-run --folders src/

# Get detailed execution plan
goneat format --plan-only --folders src/ --plan-file plan.json
```

### Control

```bash
# Process specific directories
goneat format --folders src/ tests/ --types go,yaml

# Group by file size for optimal parallelization
goneat format --group-by-size --max-workers 4
```

### Transparency

```bash
# See detailed progress and statistics
goneat format --folders . --verbose

# Export execution data for analysis
goneat format --folders . --plan-file execution.json
```

## Implementation Details

### Work Item Lifecycle

1. **Created**: File discovered and metadata collected
2. **Grouped**: Assigned to execution group
3. **Queued**: Added to worker queue
4. **Processed**: Executed by worker
5. **Completed**: Results aggregated

### Error Handling

- **Individual failures**: Don't stop entire execution
- **Retry logic**: Configurable retry attempts for transient failures
- **Partial results**: Report successful and failed items separately

### Performance Optimizations

- **Caching**: Avoid re-scanning unchanged directories
- **Batching**: Group similar operations
- **Streaming**: Handle large files efficiently
- **Profiling**: Built-in performance monitoring

## Future Extensions

### Advanced Grouping

- **Dependency analysis**: Respect import relationships
- **Change detection**: Only process modified files
- **Custom rules**: User-defined grouping strategies

### Distributed Execution

- **Cluster support**: Distribute work across machines
- **Load balancing**: Dynamic worker allocation
- **Result aggregation**: Centralized reporting

### Integration Points

- **CI/CD**: Export manifests for pipeline analysis
- **IDE integration**: Provide work plans for editor plugins
- **Monitoring**: Integration with observability systems

## Configuration Schema

The work manifest follows a versioned JSON Schema:

```yaml
$id: https://schemas.goneat.dev/work-manifest/v1.0.0
title: Goneat Work Manifest Schema
description: Schema for work manifests that describe file processing plans
```

## Benefits

### For Users
- **Predictability**: Know exactly what will be processed
- **Performance**: Optimized execution for large codebases
- **Control**: Fine-grained control over processing
- **Transparency**: Detailed reporting and progress tracking
- **Safety**: No-op mode for risk-free testing and assessment

### For Organizations
- **Scalability**: Handle large monorepos efficiently
- **Consistency**: Standardized execution across teams
- **Audibility**: Complete records of all operations
- **Integration**: Works with existing CI/CD pipelines

### For Development
- **Testability**: Dry-run mode for safe testing
- **Debugging**: Detailed manifests help diagnose issues
- **Extensibility**: Clean architecture for adding new features
- **Maintainability**: Well-documented and modular design

## Conclusion

The work planning and parallelization system provides a solid foundation for scalable, predictable, and efficient code formatting and validation operations. By separating planning from execution, we enable powerful features like dry-run mode, detailed reporting, and optimal resource utilization.

The architecture is designed to scale from small projects to large enterprise codebases while maintaining simplicity for everyday use cases.
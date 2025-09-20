# Goneat Library Documentation

This directory contains comprehensive documentation for Goneat's internal libraries and packages. These libraries provide the core functionality that powers Goneat's features and can be used independently in other Go projects.

## 📚 Documentation Overview

| Library        | Status      | Documentation                                                     |
| -------------- | ----------- | ----------------------------------------------------------------- |
| **buildinfo**  | ✅ Complete | [`buildinfo.md`](buildinfo.md)                                    |
| **config**     | ✅ Complete | [`config.md`](config.md) + Schema validation                      |
| **exitcode**   | ❌ Missing  | _Needs documentation_                                             |
| **format**     | ❌ Missing  | _Needs documentation_                                             |
| **ignore**     | ✅ Complete | [`ignore.md`](ignore.md)                                          |
| **logger**     | ✅ Complete | [`logger.md`](logger.md)                                          |
| **pathfinder** | ✅ Complete | [`pathfinder/README.md`](pathfinder/README.md)                    |
| **safeio**     | ✅ Complete | [`safeio.md`](safeio.md)                                          |
| **schema**     | ✅ Complete | [`schema.md`](schema.md) + [`schema/README.md`](schema/README.md) |
| **tools**      | ✅ Complete | [`tools.md`](tools.md) + Schema validation                        |
| **versioning** | ✅ Complete | [`versioning.md`](versioning.md)                                  |
| **work**       | ❌ Missing  | _Needs documentation_                                             |

**Current Status:** 9/12 libraries documented (75% complete)

## 🎯 Library Categories

### Core Infrastructure

- [**buildinfo**](buildinfo.md) - Build metadata and version embedding
- [**logger**](logger.md) - Structured logging with multiple output formats
- [**safeio**](safeio.md) - Secure file operations with audit trails

### Configuration & Schema

- [**config**](config/) - Configuration management and hierarchy
- [**schema**](schema.md) - JSON Schema validation and processing
- [**pathfinder**](pathfinder/) - Secure file system abstraction

### Processing & Tools

- [**format**](format/) - Code formatting and finalization
- [**ignore**](ignore.md) - Pattern matching and file filtering
- [**tools**](tools/) - Tool management and installation
- [**versioning**](versioning.md) - Semantic versioning utilities

### Workflow & Execution

- [**exitcode**](exitcode/) - Standardized exit codes and error handling
- [**work**](work/) - Task execution and parallel processing

## 📖 Documentation Standards

All library documentation follows these standards:

- **Frontmatter**: YAML metadata with title and description
- **Purpose Section**: Clear explanation of the library's role
- **Key Features**: Bullet-point list of capabilities
- **Installation**: Go module import instructions
- **Schema Validation**: Integration with Goneat's schema library (where applicable)
- **Basic Usage**: Code examples and common patterns
- **API Reference**: Function signatures and parameters
- **Best Practices**: Usage guidelines and anti-patterns
- **Examples**: Real-world usage scenarios

## 🔍 Schema Integration

Several libraries now include schema validation guidance:

- **Tools Library**: Validates `tools.yaml` configurations against `schemas/tools/v1.0.0/tools-config`
- **Config Library**: Validates configuration files against `schemas/config/v1.0.0/goneat-config`
- **Schema Library**: Core validation engine used by other libraries

Access schemas using: `goneat docs show schemas/<type>/v1.0.0/<schema-name>`

## 🚧 Missing Documentation

The following libraries require documentation:

### High Priority

- **`pkg/format`** - Code formatting pipeline and processors
- **`pkg/work`** - Task dispatching and parallel execution

### Medium Priority

- **`pkg/exitcode`** - Exit code constants and error classification

## 🤝 Contributing

To contribute library documentation:

1. **Choose a library** from the missing list above
2. **Follow the documentation standards** outlined above
3. **Include practical examples** from the existing codebase
4. **Test examples** to ensure they work correctly
5. **Submit a pull request** with the new documentation

## 📋 Quality Checklist

Before submitting documentation, ensure it includes:

- [ ] Clear purpose and use cases
- [ ] Installation instructions
- [ ] Basic usage examples
- [ ] API reference for public functions
- [ ] Error handling patterns
- [ ] Best practices and anti-patterns
- [ ] Real-world examples from goneat codebase
- [ ] Cross-references to related libraries

---

**Last Updated:** September 20, 2025
**Documentation Coverage:** 58% (7/12 libraries)</content>
</xai:function_call: write>
<parameter name="filePath">docs/appnotes/lib/README.md

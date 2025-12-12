# CI/CD Integration

This directory contains documentation for integrating goneat into CI/CD pipelines.

## Quality Assurance Strategy

### Comprehensive Linting Suite

Goneat's `lint` category includes multiple specialized linters:

#### Makefile Validation (checkmake)

- **Purpose**: Catch Makefile syntax errors and enforce best practices
- **Severity levels**: High for breaking issues, medium for style violations
- **CI recommendation**: Use `--fail-on high` to catch critical issues while allowing progressive improvement
- **Limitations**: Checkmake rules are hardcoded (5-line body limit, required `.PHONY` declarations)

#### GitHub Actions Validation (actionlint)

- **Purpose**: Validate workflow syntax, security, and best practices
- **Severity levels**: High for syntax/security issues, medium for deprecated actions
- **CI recommendation**: Always enable for workflow quality assurance
- **Capabilities**: Action reference validation, job dependency checking, security scanning

### Recommended CI Configuration

```yaml
- name: Quality Check
  run: goneat assess --categories lint --fail-on high
```

This catches critical issues across all file types:

- Makefile syntax errors and missing phony declarations
- GitHub Actions workflow issues and security problems
- Shell script syntax errors
- YAML formatting issues

Teams can progressively lower the failure threshold as code quality improves.

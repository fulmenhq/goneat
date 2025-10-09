# Authoring Standard

## Overview

This standard defines guidelines for creating and maintaining documentation and code in the goneat repository. It establishes consistent practices for content creation, formatting, and organization.

## Content Standards

### Documentation Structure

All documentation files MUST follow this structure:

1. **Frontmatter** (required) - See [Frontmatter Standard](frontmatter-standard.md)
2. **Overview Section** - Brief introduction and purpose
3. **Main Content** - Detailed information organized by topic
4. **Examples** - Practical code samples and use cases
5. **Related Documentation** - Links to related docs
6. **Metadata Footer** - Status, dates, and authorship

### Code Standards

#### File Headers

All code files MUST include:

- Copyright notice (see [Copyright Template](copyright-template.md))
- Package declaration (for Go files)
- Brief file description in comments

#### Example Go File Structure

```go
/*
Copyright © 2025 3 Leaps (hello@3leaps.net and https://3leaps.net)
*/
package main

// Package main provides the entry point for the goneat CLI tool.
// This package handles command-line argument parsing and dispatches
// to appropriate subcommands for format and lint operations.

import (
    "context"
    "fmt"
    "os"
)

// main is the entry point for the goneat CLI application.
func main() {
    // Implementation
}
```

## Templates and Fragments

### Reusable Templates

Standard templates are available in `docs/standards/templates/`:

- **Copyright Templates**: Language-specific copyright notices
  - `copyright-go.txt` - Go files
  - `copyright-ts.txt` - TypeScript/JavaScript
  - `copyright-py.txt` - Python
  - `copyright-sh.txt` - Shell scripts

- **Frontmatter Templates**: Document metadata
  - `frontmatter-yaml.txt` - YAML frontmatter template

### Using Templates

#### Automated Insertion

```bash
# Insert copyright template for Go files
cat docs/standards/templates/copyright-go.txt > newfile.go
echo "" >> newfile.go
echo "package main" >> newfile.go

# Insert frontmatter template
cat docs/standards/templates/frontmatter-yaml.txt > newdoc.md
echo "" >> newdoc.md
echo "# Document Title" >> newdoc.md
```

#### Manual Copy-Paste

Templates can be copied directly from the template files and customized as needed.

## Writing Guidelines

### Language and Tone

- **Technical Accuracy**: Use precise, correct terminology
- **Clarity**: Write for the intended audience (developers, maintainers)
- **Consistency**: Use consistent terminology throughout
- **Active Voice**: Prefer active voice over passive
- **Present Tense**: Use present tense for timeless content

### Code Examples

- **Runnable**: Examples should be complete and runnable
- **Commented**: Include explanatory comments
- **Error Handling**: Show proper error handling patterns
- **Best Practices**: Demonstrate recommended approaches

### Documentation Organization

#### File Naming

- Use kebab-case: `document-name.md`
- Include version if needed: `api-reference-v2.0.md`
- Be descriptive but concise

#### Directory Structure

```
docs/
├── standards/          # How we define/specify things
├── sop/               # What we agree to do
├── architecture/      # System design and decisions
├── user-guide/        # User-facing documentation
└── development/       # Development process docs
```

## Quality Assurance

### Review Process

1. **Self-Review**: Author reviews for accuracy and clarity
2. **Peer Review**: At least one other maintainer reviews
3. **Technical Review**: Domain expert reviews technical content
4. **Format Check**: Ensure proper frontmatter and formatting

### Validation Checklist

- [ ] Frontmatter complete and valid
- [ ] Copyright notice present (code files)
- [ ] Links functional and properly formatted
- [ ] Code examples runnable and correct
- [ ] Consistent terminology and formatting
- [ ] Appropriate status and review status

## Tooling Integration

### Automated Formatting

```bash
# Format documentation
./dist/goneat format docs/

# Format code
go fmt ./...
gofmt -s -w .
```

### Quality Checks

```bash
# Run all quality checks
make precommit

# Individual checks
make fmt          # Format code and docs
make lint         # Linting
make test         # Tests
```

## Maintenance

### Document Lifecycle

1. **Draft**: Initial creation, work in progress
2. **Review**: Ready for peer review
3. **Approved**: Reviewed and approved for use
4. **Deprecated**: No longer current, needs update
5. **Archived**: Moved to archive, no longer maintained

### Updates

- Update `last_updated` date in frontmatter
- Maintain change history in document when significant
- Review and update related documents as needed

## Related Standards

- [Frontmatter Standard](frontmatter-standard.md) - Document metadata format
- [Copyright Template](copyright-template.md) - Code copyright notices
- [Repository Operations SOP](../sop/repository-operations-sop.md) - Documentation management

---

**Status**: Approved
**Last Updated**: 2025-08-28
**Author**: @3leapsdave</content>
</xai:function_call name="bash">
<parameter name="command">mkdir -p goneat/docs/standards/templates

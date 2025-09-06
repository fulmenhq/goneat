# Document Frontmatter Standard

## Overview

This standard defines the required frontmatter format for all documentation files in the goneat repository. Frontmatter provides structured metadata that enables automated processing, search, and organization of documentation.

## Required Frontmatter Fields

All documentation files MUST include frontmatter using YAML format with the following required fields:

```yaml
---
title: "Document Title"
description: "Brief description of the document's purpose and scope"
author: "Author Name or @handle"
date: "YYYY-MM-DD"
last_updated: "YYYY-MM-DD"
status: "draft|review|approved|deprecated"
tags: ["tag1", "tag2", "tag3"]
---
# Document Content
```

## Field Definitions

### Required Fields

- **title** (string): The document title, used for navigation and indexing
- **description** (string): Brief description (1-2 sentences) explaining the document's purpose
- **author** (string): Author name or handle (e.g., "Dave Thompson" or "@3leapsdave")
- **date** (string): Creation date in ISO 8601 format (YYYY-MM-DD)
- **last_updated** (string): Last modification date in ISO 8601 format (YYYY-MM-DD)
- **status** (enum): Document status - one of: draft, review, approved, deprecated
- **tags** (array): Array of relevant tags for categorization and search

### Optional Fields

- **reviewers** (array): List of reviewers for collaborative documents
- **related_docs** (array): Links to related documentation
- **version** (string): Document version for versioned content
- **category** (string): Document category (e.g., "standards", "sop", "architecture")

## Examples

### Standard Document

```yaml
---
title: "Version Management Architecture"
description: "Architecture and design decisions for the version management system"
author: "@3leapsdave"
date: "2025-08-28"
last_updated: "2025-08-28"
status: "approved"
tags: ["architecture", "versioning", "design"]
---
```

### Collaborative Document

```yaml
---
title: "Code Review Guidelines"
description: "Standards and best practices for code review processes"
author: "@3leapsdave"
date: "2025-08-28"
last_updated: "2025-08-28"
status: "review"
reviewers: ["@code-scout", "@forge-neat"]
tags: ["development", "quality", "collaboration"]
---
```

### Versioned Document

```yaml
---
title: "API Reference v2.0"
description: "Complete API reference for version 2.0 of the goneat CLI"
author: "@3leapsdave"
date: "2025-08-28"
last_updated: "2025-08-28"
status: "approved"
version: "2.0"
tags: ["api", "reference", "cli"]
---
```

## Implementation Guidelines

### File Naming

- Use kebab-case for filenames: `document-title.md`
- Include version in filename if needed: `api-reference-v2.0.md`

### Validation

- Frontmatter must be valid YAML
- All required fields must be present
- Dates must follow ISO 8601 format
- Status must be one of the allowed values

### Tooling Integration

- Frontmatter enables automated documentation processing
- Search and indexing systems can use metadata
- Status tracking for document lifecycle management

## Related Standards

- [Authoring Standard](authoring-standard.md) - Content creation guidelines
- [Copyright Template](copyright-template.md) - Code copyright standards
- [Document Organization SOP](../sop/repository-operations-sop.md) - Repository documentation management

---

**Status**: Approved
**Last Updated**: 2025-08-28
**Author**: @3leapsdave</content>
</xai:function_call name="write">
<parameter name="filePath">goneat/docs/standards/copyright-template.md

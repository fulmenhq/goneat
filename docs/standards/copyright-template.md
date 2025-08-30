# Copyright Template Standard

## Overview

This standard defines the copyright notice format for all code files in the goneat repository. Consistent copyright notices ensure legal compliance and proper attribution across the Fulmen ecosystem.

## Standard Copyright Notice

All code files MUST include the following copyright notice at the top of the file:

```go
/*
Copyright © 2025 3 Leaps (hello@3leaps.net and https://3leaps.net)
*/
```

## Language-Specific Formats

### Go Files

```go
/*
Copyright © 2025 3 Leaps (hello@3leaps.net and https://3leaps.net)
*/
package main
```

### TypeScript/JavaScript Files

```typescript
/*
Copyright © 2025 3 Leaps (hello@3leaps.net and https://3leaps.net)
*/
```

### Python Files

```python
# Copyright © 2025 3 Leaps (hello@3leaps.net and https://3leaps.net)
```

### Shell Scripts

```bash
#!/bin/bash
# Copyright © 2025 3 Leaps (hello@3leaps.net and https://3leaps.net)
```

### YAML Files

```yaml
# Copyright © 2025 3 Leaps (hello@3leaps.net and https://3leaps.net)
---
```

### Markdown Files

```markdown
<!--
Copyright © 2025 3 Leaps (hello@3leaps.net and https://3leaps.net)
-->
```

## Implementation Guidelines

### Placement

- **Top of file**: Copyright notice must be the first content in the file
- **Before package declarations**: Place before `package`, `import`, or other declarations
- **No blank lines before**: No empty lines between copyright and code

### Year Updates

- **Current year**: Use the current year (2025) for new files
- **Range format**: For files spanning multiple years, use: `Copyright © 2024-2025 3 Leaps`
- **Annual updates**: Update copyright year when making significant modifications

### Contact Information

- **Email**: hello@3leaps.net (general inquiries)
- **Website**: https://3leaps.net (company information)
- **No personal emails**: Do not include individual contributor email addresses

## Automated Application

### Template Files

Reusable copyright templates are available in `docs/standards/templates/`:

- `copyright-go.txt` - Go language template
- `copyright-ts.txt` - TypeScript/JavaScript template
- `copyright-py.txt` - Python template
- `copyright-sh.txt` - Shell script template

### IDE Integration

Consider configuring your IDE/editor to automatically insert the copyright notice for new files.

### Bulk Updates

For updating copyright years across multiple files:

```bash
# Find files with 2024 copyright
find . -name "*.go" -exec grep -l "Copyright © 2024" {} \;

# Update copyright year (use with caution)
find . -name "*.go" -exec sed -i 's/Copyright © 2024/Copyright © 2025/g' {} \;
```

## Legal Considerations

### Open Source Compliance

- This copyright notice supports MIT and other permissive licenses
- Does not restrict commercial use or modification
- Provides proper attribution to 3 Leaps

### Fulmen Ecosystem

- Consistent with other Fulmen repositories
- Supports the 3 Leaps brand and contact information
- Enables proper attribution across the ecosystem

## Examples

### Complete Go File

```go
/*
Copyright © 2025 3 Leaps (hello@3leaps.net and https://3leaps.net)
*/
package main

import "fmt"

func main() {
    fmt.Println("Hello, World!")
}
```

### File with Multiple Years

```go
/*
Copyright © 2024-2025 3 Leaps (hello@3leaps.net and https://3leaps.net)
*/
package main
```

## Related Standards

- [Authoring Standard](authoring-standard.md) - Content creation guidelines
- [Frontmatter Standard](frontmatter-standard.md) - Document metadata standards
- [AGENTS.md](../../AGENTS.md) - AI agent attribution standards

---

**Status**: Approved
**Last Updated**: 2025-08-28
**Author**: @3leapsdave</content>
</xai:function_call name="write">
<parameter name="filePath">goneat/docs/standards/authoring-standard.md

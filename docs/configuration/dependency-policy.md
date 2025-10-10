# Dependency Policy Configuration

Configure dependency analysis in `.goneat/dependencies.yaml`.

## Example

```yaml
version: v1
cooling:
  enabled: true
  min_age_days: 7
  min_downloads: 100
licenses:
  forbidden:
    - GPL-3.0
    - AGPL-3.0
policy_engine:
  type: embedded
  rego_files:
    - ".goneat/policies/licenses.rego"
```

## Fields

- **cooling**: Package cooling policy settings.
- **licenses**: License compliance rules.
- **policy_engine**: OPA configuration.

See schema for full details.

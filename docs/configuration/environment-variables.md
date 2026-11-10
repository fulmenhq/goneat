# Environment Variables

This document describes environment variables that control goneat's behavior.

## Schema Validation

### GONEAT_OFFLINE_SCHEMA_VALIDATION

- **Status:** No effect. Schema validation never uses the network: references resolve only to the schema itself, `--ref-dir` schemas and the built-in metaschemas, and `$schema` is always honoured. The variable is still accepted and ignored.

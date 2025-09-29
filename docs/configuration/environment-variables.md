# Environment Variables

This document describes environment variables that control goneat's behavior.

## Schema Validation

### GONEAT_OFFLINE_SCHEMA_VALIDATION

- **Type:** Boolean (true/false)
- **Default:** false
- **Description:** When enabled, schema validation operates in offline mode. This removes `$schema` references from schema documents before validation to prevent network fetches of external meta-schemas. Useful in air-gapped or restricted network environments.
- **Example:** `GONEAT_OFFLINE_SCHEMA_VALIDATION=true goneat schema validate-schema my-schema.json`

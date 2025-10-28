---
title: "Telemetry & Metrics Export"
description: "Lightweight counter and histogram export aligned with the observability logging pipeline."
status: "draft"
last_updated: "2025-10-24"
tags: ["standards", "library", "observability", "metrics"]
---

# Telemetry & Metrics Export

## Overview

This extension module adds a minimal metrics surface to Fulmen helper libraries so that critical paths can be
instrumented from day zero. Events are emitted as JSON (stdout or structured logs) and validated against
`schemas/observability/metrics/v1.0.0/metrics-event.schema.json`, which in turn references the shared
metrics taxonomy (`config/taxonomy/metrics.yaml`).

## Principles

- **Taxonomy-Backed Identifiers** – Metric names/units come from the taxonomy file; libraries MUST not invent
  ad-hoc identifiers without upstreaming them into Crucible.
- **Scalar or Histogram** – `value` MAY be a scalar number (counter/gauge) or a histogram summary object with
  OTLP-style buckets (`{count,sum,buckets[]}`).
- **JSON-First Export** – Metrics are serialised to JSON for pipelines/agents; libraries may adapt the data to
  other sinks but the canonical format is JSON.
- **No External Dependencies** – Implementations use standard library data structures and ship offline.
- **Optional Adoption** – Module is recommended (extension tier) but not mandatory; coverage targets set
  expectations per language.

## API Surface

- `metrics.counter(name).inc(delta?)`
- `metrics.gauge(name).set(value)`
- `metrics.histogram(name).observe(value)`
- `metrics.export()` → array of schema-compliant events
- `metrics.flush()` → emit & clear events, optionally publishing through the logging pipeline

## Default Histogram Buckets (ADR-0007)

Per [ADR-0007](../../architecture/decisions/ADR-0007-telemetry-default-histogram-buckets.md) Fulmen libraries
MUST use the following default bucket boundaries (in **milliseconds**) for any histogram whose metric name ends
with the `_ms` suffix. Implementations MAY allow overrides on a per-metric basis, but the defaults ensure
consistent aggregation and dashboards across languages.

```
[1, 5, 10, 50, 100, 500, 1000, 5000, 10000]
```

These buckets cover fast in-memory operations (≤10 ms), moderate operations (10 – 100 ms), slower I/O/network
workloads (100 – 1 000 ms), and long-running tasks (1 – 10 s). Libraries SHOULD expose helpers that pick the
default buckets automatically when the metric unit in the taxonomy is `ms`.

## Instrumentation Patterns by Module Type

Helper libraries share common module shapes. Pick the instrumentation pattern that balances observability with
runtime overhead so latency-heavy drivers get histograms, hot loops avoid unnecessary cost, and
security-sensitive actions retain the audit signals they need.

### Standard Pattern (Histogram + Counter)

Use for moderate-frequency operations where latency variability is meaningful:

- File and config I/O (config loaders, schema reads)
- Network-bound discovery or sync operations
- Validation flows and other work that may regress silently

Pattern guidance:

1. Capture start time at the beginning of the operation.
2. Use `try/finally` (Python/TypeScript) or `defer` (Go) to record elapsed time into a histogram metric.
3. Increment a success counter in the normal path and an error counter inside exception handling.

This pattern surfaces both throughput and latency information, making regressions easy to detect.

### Performance-Sensitive Pattern (Counter-Only)

Use when the operation is invoked in tight loops or high-volume pipelines where histogram timing cost outweighs
its value:

- Hash computations and encoder/decoder loops (FulHash)
- In-memory catalog lookups (Foundry taxonomy, key/value caches)
- Text normalization and tokenisation hot paths (Docscribe, similarity scoring)

Guidance:

- Increment a counter for successful operations; optionally add an error counter.
- Skip histogram timing; in benchmarked helper libraries the counter cost is <10 ns, while histogram timing
  adds 50 – 100 ns per invocation.
- Document the choice in module-specific ADRs when additional context (call rates, SLAs) is helpful.

### Audit & Compliance Pattern (Counter + Audit Event)

Use when operations must produce audit trails or may leak sensitive data through timing side-channels:

- Authentication/authorization checks and credential refresh
- Privileged configuration or cache mutation paths (e.g., forced sync, flush)
- Security posture probes or rate-limit enforcement

Guidance:

- Emit attempt, success, and failure counters so monitoring captures drift in behaviour.
- Produce a structured audit log event per ADR-0003 with sensitive values redacted; correlate via request or
  operation IDs.
- Avoid histogram timing unless explicitly approved—latency distributions can reveal sensitive workload details.
- Where ongoing state matters (e.g., active sessions), expose a gauge metric updated alongside counters.

### Module-Specific Recommendations

| Module Type           | Recommended Pattern                         | Metrics                                                                  | Rationale                                                    |
| --------------------- | ------------------------------------------- | ------------------------------------------------------------------------ | ------------------------------------------------------------ |
| File / Config I/O     | Standard (histogram + counter)              | `*_ms` latency histogram, success/error counters                         | Latency variance indicates filesystem or network issues      |
| Validation            | Standard (histogram + counter)              | Per-operation latency, error counter                                     | Catch schema drifts and slow validation runs                 |
| Hashing               | Performance-sensitive (counter-only)        | Success counter, error counter                                           | Called tens of thousands of times; timing adds notable cost  |
| Catalog Lookup        | Performance-sensitive (counter-only)        | Lookup counter                                                           | Pure in-memory; latency already microseconds                 |
| Text Processing       | Performance-sensitive (counter-only)        | Operation counter, optional error counter                                | High-frequency workloads; counts sufficient for profiling    |
| Security / Audit Flow | Audit & compliance (counters + audit event) | Attempt/success/failure counters, gauge if applicable, audit log payload | Requires traceable audit trail without revealing timing data |

Apply these recommendations consistently across gofulmen, pyfulmen, tsfulmen, and future foundations so
observability behaviour remains predictable regardless of language.

## Language Examples

**Go**

```go
metrics.Counter("schema_validations").Inc()
metrics.Histogram("config_load_ms").Observe(42)
if err := telemetry.Validate(metrics.Export()); err != nil {
    log.Fatal(err)
}
metrics.Flush()
```

**TypeScript**

```typescript
metrics.counter("schema_validations").inc();
metrics.histogram("config_load_ms").observe(42);
assert(validateMetrics(metrics.export()), "invalid payload");
metrics.flush();
```

**Python**

```python
metrics.counter("schema_validations").inc()
metrics.histogram("config_load_ms").observe(42)
assert validate_metrics(metrics.export())
metrics.flush()
```

## Testing Expectations

- **Coverage**: ≥85 % covering counters, gauges, histograms, tags, and flush behaviour.
- **Fixtures**: `tests/fixtures/metrics/` should include scalar and histogram examples plus failure cases (unknown
  metric, invalid buckets).
- **Integration**: Validate alongside logging middleware (metrics events can be appended to log payloads) and
  schema validation flows.
- **Tooling**: goneat `schema validate` using the updated metrics schema and taxonomy definitions.

## Ecosystem Ties

- **Logging**: Metrics events may be embedded inside progressive logging payloads for SIMPLE/STRUCTURED profiles.
- **Assessment**: Future goneat assess categories can rely on the taxonomy to audit mandatory metrics.
- **Extensibility**: New metrics are proposed via Crucible PRs updating `config/taxonomy/metrics.yaml` so all
  languages stay in sync.
- **ADR Mapping**: Default histogram buckets defined in ADR-0007.

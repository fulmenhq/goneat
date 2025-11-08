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

## Prometheus Exporter Metrics

Helper libraries shipping Prometheus exporters (gofulmen, pyfulmen ≥v0.1.10, tsfulmen ≥v0.1.8) emit
exporter-specific telemetry to monitor exporter health and HTTP exposition performance. These metrics use the
`prometheus_exporter_*` namespace reserved for infrastructure internals.

### Exporter Metrics Specification

| Metric Name                                    | Unit    | Type      | Description                                                                   | Required Labels   | Optional Labels |
| ---------------------------------------------- | ------- | --------- | ----------------------------------------------------------------------------- | ----------------- | --------------- |
| `prometheus_exporter_refresh_duration_seconds` | `s`     | Histogram | Time to refresh/export registry data. Uses ADR-0007 buckets ÷ 1000 (seconds). | `phase`, `result` | `error_type`    |
| `prometheus_exporter_refresh_total`            | `count` | Counter   | Total refresh cycles attempted (success/error breakdown via `result` label).  | `result`          | `error_type`    |
| `prometheus_exporter_refresh_errors_total`     | `count` | Counter   | Failed refresh cycles with error classification.                              | `error_type`      | `phase`         |
| `prometheus_exporter_refresh_inflight`         | `count` | Gauge     | Concurrent refresh operations currently running.                              | None              | `phase`         |
| `prometheus_exporter_http_requests_total`      | `count` | Counter   | HTTP scrape/exposition requests handled by exporter.                          | `status`, `path`  | `client`        |
| `prometheus_exporter_http_errors_total`        | `count` | Counter   | HTTP exposition failures (5xx, auth failures, timeouts).                      | `status`, `path`  | `client`        |
| `prometheus_exporter_restarts_total`           | `count` | Counter   | Exporter refresh loop or HTTP server restarts.                                | `reason`          | None            |

**Implementation note**: Type shown for clarity; metric type remains implicit in taxonomy (inferred from API: `counter()`, `histogram()`, `gauge()`).

### Label Value Standards

Standardized label values ensure portable dashboards and alerts:

| Label        | Allowed Values                          | Used By Metrics                                                                            | Notes                                       |
| ------------ | --------------------------------------- | ------------------------------------------------------------------------------------------ | ------------------------------------------- |
| `phase`      | `collect`, `convert`, `export`          | `prometheus_exporter_refresh_duration_seconds`, `prometheus_exporter_refresh_errors_total` | Refresh pipeline stages                     |
| `result`     | `success`, `error`                      | `prometheus_exporter_refresh_duration_seconds`, `prometheus_exporter_refresh_total`        | Operation outcome                           |
| `error_type` | `validation`, `io`, `timeout`, `other`  | `prometheus_exporter_refresh_errors_total`, optionally on `refresh_*` metrics              | Error classification                        |
| `status`     | `200`, `500`, `503`, etc.               | `prometheus_exporter_http_requests_total`, `prometheus_exporter_http_errors_total`         | HTTP status codes                           |
| `path`       | `/metrics`, `/health`, etc.             | `prometheus_exporter_http_requests_total`, `prometheus_exporter_http_errors_total`         | HTTP endpoint paths                         |
| `client`     | `<ip>`, `<service_name>`, etc.          | Optional for HTTP metrics                                                                  | Scraper ID (high cardinality—use sparingly) |
| `reason`     | `config_change`, `error`, `manual`, etc | `prometheus_exporter_restarts_total`                                                       | Restart trigger                             |

**Cardinality warning**: The `client` label creates one time series per unique client. Only use when multiple scrapers require differentiation; omit otherwise to prevent metric series explosion.

### Histogram Bucket Conversion

**Critical**: Prometheus histograms use **seconds**, not milliseconds.

ADR-0007 default buckets (milliseconds):

```
[1, 5, 10, 50, 100, 500, 1000, 5000, 10000]
```

**Exporters MUST divide by 1000 before emitting** to Prometheus:

```
[0.001, 0.005, 0.01, 0.05, 0.1, 0.5, 1.0, 5.0, 10.0]  (seconds)
```

**Warning**: Do not double-convert; exporters should convert once at emission time. The `_seconds` suffix in the metric name signals that values are already in seconds.

### Dual Counter Emission Rules

Exporters emit both `prometheus_exporter_refresh_total` and `prometheus_exporter_refresh_errors_total` to serve different query patterns:

**Emission Rules**:

1. **Always emit**: `prometheus_exporter_refresh_total{result="success"}` for successful refreshes
2. **Always emit**: `prometheus_exporter_refresh_total{result="error"}` for failed refreshes
3. **Additionally emit**: `prometheus_exporter_refresh_errors_total{error_type="..."}` for errors with detailed classification
4. **Invariant**: `sum(prometheus_exporter_refresh_errors_total) MUST equal sum(prometheus_exporter_refresh_total{result="error"})`

**Use case separation**:

- Use `refresh_total` for overall throughput and success rate dashboards
- Use `refresh_errors_total` for detailed error breakdown by type and phase

**Example PromQL**:

```promql
# Overall success rate
rate(prometheus_exporter_refresh_total{result="success"}[5m])
  / rate(prometheus_exporter_refresh_total[5m])

# Error breakdown by type
sum by (error_type) (rate(prometheus_exporter_refresh_errors_total[5m]))
```

### Prometheus Client Default Metrics

Many Prometheus client libraries emit default metrics:

- Node.js (`prom-client`): `http_requests_total`, `http_request_duration_seconds`, process metrics
- Python (`prometheus-client`): `process_*`, `python_gc_*`, platform metrics
- Go (`prometheus/client_golang`): `go_*`, process metrics

**Recommendation**: Disable client library defaults to prevent duplicate time series and reduce cardinality.

**TypeScript/Node.js (prom-client)**:

```typescript
import { register } from "prom-client";

// Option A: Clear all default metrics
register.clear();

// Option B: Disable default collection
import { collectDefaultMetrics } from "prom-client";
collectDefaultMetrics({ register: null });
```

**Python (prometheus-client)**:

```python
from prometheus_client import REGISTRY, PROCESS_COLLECTOR, PLATFORM_COLLECTOR, GC_COLLECTOR

# Clear default collectors
REGISTRY.unregister(PROCESS_COLLECTOR)
REGISTRY.unregister(PLATFORM_COLLECTOR)
REGISTRY.unregister(GC_COLLECTOR)
```

**Go (prometheus/client_golang)**:

```go
// Use custom registry instead of default
registry := prometheus.NewRegistry()
// Register only taxonomy metrics, not default Go collectors
```

**If defaults must remain**: Document namespace distinction clearly:

- Client defaults: `http_*`, `process_*`, `go_*`, etc.
- Taxonomy metrics: `prometheus_exporter_http_*` (exporter-specific)

Dashboards should reference taxonomy metrics for consistency across languages.

### Compatibility Matrix

| Library  | Prometheus Exporter | Taxonomy Support | Notes                  |
| -------- | ------------------- | ---------------- | ---------------------- |
| gofulmen | ✅ Implemented      | ✅ v0.2.7+       | Uses taxonomy metrics  |
| pyfulmen | ✅ Implemented      | ✅ v0.1.10+      | Uses taxonomy metrics  |
| tsfulmen | 🔄 In progress      | ✅ v0.1.8+       | Implementing in v0.1.8 |

**Minimum Crucible version**: ≥v0.2.7 for `prometheus_exporter_*` metrics and `s` (seconds) unit support.

## Metric Namespace Governance

### Module Prefix Reservation

**Module prefixes are reserved by Crucible** to ensure cross-language consistency and prevent namespace collisions. Each Fulmen module reserves a prefix for its metrics:

**Reserved Prefixes**:

- `foundry_*` — Foundry catalog and pattern matching operations
- `pathfinder_*` — File discovery and validation
- `error_handling_*` — Error wrapping and propagation
- `fulhash_*` — Hashing operations
- `config_load_*` — Configuration loading
- `schema_validation_*` — Schema validation
- `logging_emit_*` — Log emission
- `prometheus_exporter_*` — Prometheus exporter internals
- _(Additional modules reserve prefixes as they're added)_

**Purpose**: Reserved prefixes enable ecosystem-wide dashboards and alerts that compare module behavior across:

- Languages (gofulmen vs pyfulmen vs tsfulmen)
- Services (multiple apps using the same module)
- Environments (dev vs staging vs production)

### Required Module Metrics

Each Fulmen module SHOULD define **required metrics** in the taxonomy (`config/taxonomy/metrics.yaml`) to enable cross-language observability. When a module is implemented in multiple languages, all implementations MUST emit the required metrics with identical names, labels, and semantics.

**Example: Foundry Module**

```yaml
# Required metrics (in taxonomy)
- foundry_lookup_count # Catalog lookup operations
- foundry_mime_detections_total # MIME type detections
- foundry_mime_detection_ms # MIME detection latency
```

Libraries implementing Foundry (gofulmen, pyfulmen, tsfulmen) MUST emit these metrics for module compliance and cross-language consistency.

### Implementer Extensions

**Library implementers MAY add additional metrics** for their module implementations beyond taxonomy requirements, recognizing they may need to adapt if Crucible later adds metrics with the same names.

✅ **Allowed**:

```python
# pyfulmen adds Python-specific Foundry metric (not in taxonomy)
metrics.counter("foundry_cache_hits_total").inc()
```

⚠️ **Caveat**: If Crucible later adds `foundry_cache_hits_total` to the taxonomy with different semantics, pyfulmen must adapt its implementation to match.

**Best Practice**: Petition Crucible (via PR to `config/taxonomy/metrics.yaml`) to add metrics that would benefit ecosystem-wide observability. Metrics useful across all language implementations should be standardized.

**Petition Process**:

1. Open PR adding metric to `config/taxonomy/metrics.yaml`
2. Include description, unit, required labels
3. Provide rationale for cross-language value
4. Wait for approval from `MAINTAINERS.md` before emitting in production

### Application Metric Rules

**Applications MUST NOT use module prefixes** for their business logic metrics unless they meet specific criteria.

**Allowed Application Prefixes**:

- Binary name: `percheron_*`, `groningen_*`, `sumpter_*`
- Organization: `acmecorp_*`, `example_*`
- Service: `api_gateway_*`, `auth_service_*`

**Forbidden**:

- Module prefixes: `foundry_*`, `pathfinder_*`, etc. (reserved for Fulmen modules)
- Infrastructure prefixes: `prometheus_exporter_*` (reserved for exporters)

**Exception**: If an application binary name matches a Fulmen module name (e.g., app named `foundry`), the application:

1. MUST obtain maintainer approval from `MAINTAINERS.md`
2. MUST emit all required module metrics with standard semantics
3. MUST NOT add non-standard metrics using the module prefix

**Application Metric Emission**:
Applications emit their metrics via **Prometheus client libraries directly**, NOT through `metrics-event.schema.json`:

```typescript
// Application code - use Prometheus client directly
import { Counter } from "prom-client";
const jobsProcessed = new Counter({
  name: "percheron_jobs_processed_total",
  help: "Total jobs processed by Percheron",
});
jobsProcessed.inc();
```

**Not**:

```typescript
// Wrong - applications don't use taxonomy for business metrics
metrics.counter("percheron_jobs_processed_total").inc(); // Won't work - not in taxonomy
```

### Three-Tier Metric Architecture

| Tier | Scope                   | In Taxonomy | Emitted Via                 | Prefix Examples                           | Purpose                             |
| ---- | ----------------------- | ----------- | --------------------------- | ----------------------------------------- | ----------------------------------- |
| 1    | **Module Internals**    | ✅ Yes      | `metrics-event.schema.json` | `foundry_*`, `pathfinder_*`, `fulhash_*`  | Cross-language module observability |
| 2    | **Infrastructure**      | ✅ Yes      | `metrics-event.schema.json` | `prometheus_exporter_*`                   | Infrastructure health monitoring    |
| 3    | **Application Metrics** | ❌ No       | Prometheus client libraries | `percheron_*`, `groningen_*`, `sumpter_*` | Application business logic          |

**Key Distinction**:

- **Tier 1 & 2**: Module/infrastructure metrics standardized across all helper libraries → in taxonomy
- **Tier 3**: Application-specific business metrics → NOT in taxonomy, apps choose their own

### Collision Prevention

| Scenario                                    | Rule                                 | Example                                                         |
| ------------------------------------------- | ------------------------------------ | --------------------------------------------------------------- |
| Fulmen module                               | Prefix reserved, metrics in taxonomy | `foundry_lookup_count` (in taxonomy)                            |
| Helper library extending module             | Can add metrics with adaptation risk | `foundry_cache_hits_total` (pyfulmen-specific, not in taxonomy) |
| Application (different name)                | Use binary prefix, NOT in taxonomy   | `percheron_job_duration_ms` (emitted by app)                    |
| Application (same name as module)           | Requires approval + conformance      | App named `foundry` must emit standard `foundry_*`              |
| Application attempting to use module prefix | ❌ Forbidden without approval        | App cannot emit `foundry_custom_metric` arbitrarily             |

**Example Collision Scenario**:

```python
# pyfulmen implements Foundry module
metrics.counter("foundry_lookup_count").inc()  # ✅ Required metric (in taxonomy)
metrics.counter("foundry_cache_hits_total").inc()  # ✅ Extension (not in taxonomy, adaptation risk)

# Application using pyfulmen
from prom_client import Counter
app_jobs = Counter("percheron_jobs_total")  # ✅ Application metric (binary prefix)
app_jobs.inc()

# Wrong - application trying to use module prefix
foundry_custom = Counter("foundry_custom_metric")  # ❌ Forbidden - module prefix
```

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

- **SSOT**: This document (`docs/standards/library/modules/telemetry-metrics.md`) is the **canonical reference** for telemetry implementation guidance. Library teams MUST reference this document when implementing telemetry modules.
- **Taxonomy**: Metric names, units, and required metrics defined in `config/taxonomy/metrics.yaml` (synced to language wrappers via `make sync`).
- **Logging**: Metrics events may be embedded inside progressive logging payloads for SIMPLE/STRUCTURED profiles.
- **Assessment**: Future goneat assess categories can rely on the taxonomy to audit mandatory metrics.
- **Extensibility**: New metrics are proposed via Crucible PRs updating `config/taxonomy/metrics.yaml` so all
  languages stay in sync.
- **ADR Mapping**: Default histogram buckets (ADR-0007), instrumentation patterns (ADR-0008).

---
title: "Fulmen Logging Standard"
description: "Cross-language logging requirements within the observability program"
author: "Codex Assistant"
date: "2025-10-02"
last_updated: "2025-10-09"
status: "draft"
tags: ["observability", "logging", "telemetry"]
---

# Logging Standard

> Status: Draft – targeting first release with the logging/telemetry initiative.

## Scope

This standard governs structured logging across Fulmen repositories. It defines the event envelope, severity model, configuration structure, runtime expectations, and packaging strategy. Logging is a sibling within the broader observability program (metrics, tracing, etc.) and may be consumed independently.

## Event Envelope

All log events MUST emit JSON with the following shape (additional fields allowed unless noted):

| Field            | Type    | Required | Notes                                                                                      |
| ---------------- | ------- | -------- | ------------------------------------------------------------------------------------------ |
| `timestamp`      | string  | ✅       | RFC3339Nano UTC timestamp.                                                                 |
| `severity`       | string  | ✅       | Enum value (`TRACE`, `DEBUG`, `INFO`, `WARN`, `ERROR`, `FATAL`, `NONE`).                   |
| `severityLevel`  | integer | ✅       | Numeric representation (TRACE=0, DEBUG=10, INFO=20, WARN=30, ERROR=40, FATAL=50, NONE=60). |
| `message`        | string  | ✅       | Human-readable message.                                                                    |
| `service`        | string  | ✅       | Service/application name.                                                                  |
| `component`      | string  | ⚠️       | Subsystem/component name; empty string discouraged.                                        |
| `logger`         | string  | ⚠️       | Logger instance identifier (e.g., `gofulmen.pathfinder`).                                  |
| `environment`    | string  | ⚠️       | Deployment environment tag (e.g., `production`, `staging`).                                |
| `context`        | object  | ⚠️       | Arbitrary key/value map (emit `{}` when empty).                                            |
| `contextId`      | string  | ⚠️       | Execution context identifier (job, pipeline, CLI invocation).                              |
| `requestId`      | string  | ⚠️       | Per-request identifier (HTTP `X-Request-ID` header).                                       |
| `correlationId`  | string  | ⚠️       | Cross-service correlation UUID (UUIDv7 generated if caller omits).                         |
| `traceId`        | string  | ⚠️       | REQUIRED when tracing enabled; OpenTelemetry trace identifier.                             |
| `spanId`         | string  | ⚠️       | REQUIRED when tracing enabled; span identifier.                                            |
| `parentSpanId`   | string  | ⚠️       | Optional parent span identifier for nested operations.                                     |
| `operation`      | string  | ⚠️       | Logical operation or handler name (CLI command, HTTP route, job step).                     |
| `durationMs`     | number  | ⚠️       | Operation duration in milliseconds.                                                        |
| `userId`         | string  | ⚠️       | Authenticated user identifier when available.                                              |
| `error`          | object  | ⚠️       | When present: `{ "message": string, "type"?: string, "stack"?: string }`.                  |
| `tags`           | array   | ⚠️       | Optional string array for ad-hoc filtering.                                                |
| `eventId`        | string  | ⚠️       | Optional unique identifier assigned by the producer.                                       |
| `throttleBucket` | string  | ⚠️       | Set when throttling drops are applied.                                                     |
| `redactionFlags` | array   | ⚠️       | Redaction indicators emitted by middleware (e.g., `["pii"]`).                              |

JSON output MUST be newline-delimited when written to files/streams.

### Correlation & Context Propagation

- **Correlation ID (`correlationId`)**: generate a UUIDv7 when the caller does not provide one. Propagate inbound
  values across HTTP (`X-Correlation-ID`) and gRPC metadata. UUIDv7 ensures time-sortable identifiers for Splunk
  and Datadog searches.
- **Request ID (`requestId`)**: represent the current transport request. For HTTP, read/emit `X-Request-ID`.
  For CLI workflows, generate an operation-scoped UUID (prefix optional) and surface it in human output.
- **Context ID (`contextId`)**: tie together larger execution scopes (batch pipeline run, scheduled job, CLI
  session). CLI tools SHOULD reuse a single context ID for the entire invocation while generating distinct
  request IDs per sub-command when appropriate.
- **Tracing IDs**: when OpenTelemetry (or another tracer) is enabled, emit `traceId`, `spanId`, and
  `parentSpanId` for every event within the span. Absence of tracing MUST fall back to correlation/request IDs
  so downstream systems still link records.
- **Operation metadata**: populate `operation`, `durationMs`, and `userId` when available so dashboards can
  aggregate latency and audit activity.

## Severity Enum & Filtering

Severity values and numeric order:

| Name    | Numeric | Description                                       |
| ------- | ------- | ------------------------------------------------- |
| `TRACE` | 0       | Highly verbose diagnostics.                       |
| `DEBUG` | 10      | Debug-level details.                              |
| `INFO`  | 20      | Core operational events.                          |
| `WARN`  | 30      | Something unusual but not breaking.               |
| `ERROR` | 40      | Request/operation failure (recoverable).          |
| `FATAL` | 50      | Unrecoverable failure; program exit expected.     |
| `NONE`  | 60      | Explicitly disable emission (sink-level filters). |

Comparisons (e.g., `< INFO`, `>= WARN`) MUST operate on numeric levels. `NONE` is treated as "filter everything" when used as a minimum level.

## Configuration Model

Configuration is authored in YAML and normalized to JSON. Top-level fields:

- `defaultLevel` – minimum severity (enum).
- `sinks[]` – array of sink entries with `type`, `level`, `options`, `middleware`, and `throttling`.
- `middleware[]` – global middleware chain definitions.
- `encoders` – named encoder configs (e.g., JSON, NDJSON with additional formatting).
- `fields` – static (`fields.static`) and dynamic (`fields.dynamic`) attributes appended to events.
- `throttling` – global defaults (`mode`, `bufferSize`, `dropPolicy`).
- `exports` – optional remote sink definitions (future use).

### Sink Options

Each sink entry includes:

```yaml
- name: console
  type: console
  level: INFO
  encoder: json
  middleware: [redact-secrets]
  throttling:
    mode: non-blocking
    bufferSize: 1000
    dropPolicy: drop-oldest
  options:
    stderrOnly: true
```

Supported sink types: `console`, `file`, `rolling-file`, `memory`, `external` (future). Console sinks MUST force `stderrOnly: true`. File sinks define path, rotation, retention.

### Middleware

Middleware entries define processors applied before emission. Interface semantics:

- **Go**: `type Middleware func(event *Event) (skip bool)` executed sequentially.
- **TypeScript**: `(event: LogEvent) => LogEvent | null` where `null` indicates drop.
- **Rust/Python/C#**: Align with language idioms (e.g., `Layer` in `tracing`, processor list in structlog, `Enricher`/`Filter` in Serilog).

Recommended built-ins: `redact-secrets`, `redact-pii`, `request-context` (injects correlation/request IDs),
`annotate-trace`, `throttle` (wraps queue logic).

### Throttling / Backpressure

Configuration keys:

- `mode`: `blocking` | `non-blocking`.
- `bufferSize`: integer (required when `blocking`).
- `dropPolicy`: `drop-oldest` | `drop-newest` (for non-blocking) | `block`.
- `flushInterval`: optional duration for background flush in non-blocking mode.

Underlying libraries must map these semantics appropriately (see implementation notes).

## Output Channels

- Console sink writes to `stderr` only. Duplicating to `stdout` is forbidden to preserve CLI/streaming guarantees.
- Application output intended for users or upstream pipelines continues to use `stdout` outside of the logging pipeline.

## Runtime API Expectations

Language packages MUST expose:

- Constructors accepting `LoggerOptions` (service, component, min level, middleware list, sinks, throttling config).
- Methods: `Trace`, `Debug`, `Info`, `Warn`, `Error`, `Fatal`, `WithFields`, `WithError`, `Sync` (or idiomatic equivalents).
- Middleware registration API (chain composition).
- Graceful shutdown via `Sync` to flush buffers.

## Cross-Language Implementation

| Language   | Baseline Library                             | Notes                                                                                      |
| ---------- | -------------------------------------------- | ------------------------------------------------------------------------------------------ |
| Go         | `uber-go/zap`                                | Use zapcore for custom levels, middleware, throttling. Provide wrapper in gofulmen.        |
| TypeScript | `pino`                                       | Use transports for async writes, `pino-std-serializers` for error handling, redact plugin. |
| Rust       | `tracing` + `tracing-subscriber`             | Provide helper crate translating config to subscriber layers.                              |
| Python     | `structlog` (over stdlib logging)            | Use processor chains for middleware; offer optional stdlib adapter.                        |
| C#         | `Serilog` via `Microsoft.Extensions.Logging` | Provide configuration mapping and middleware via enrichers/filters.                        |

Each package must be installable standalone (e.g., `fulmen-logging` on PyPI) but can be bundled in a future "observability" meta-package.

## Packaging & Distribution

- Go: `gofulmen` module (`foundation/logging`).
- TypeScript: `@fulmenhq/crucible/logging` entry point.
- Python: `fulmen-logging` PyPI package (optional dependency for full Crucible bundle).
- Rust: `fulmen_logging` crate.
- C#: `Fulmen.Logging` NuGet package.

## Validation & Tooling

- `make release:check` MUST run logging schema validation (AJV or similar) and ensure severity enum alignment.
- Future CLI hook (e.g., via `goneat`) will lint redaction/throttling config.

## Roadmap

- Finalize schema files in `schemas/observability/logging/v1.0.0/`.
- Produce sink capability matrix for documentation.
- Extend to metrics/tracing once logging baseline is shipped.

## Contacts

- Human maintainer: @3leapsdave
- AI steward: @schema-cartographer (🧭)

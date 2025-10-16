---
title: "HTTP REST Standard"
description: "Fulmen-wide conventions for designing and implementing HTTP APIs"
author: "Schema Cartographer"
date: "2025-10-09"
last_updated: "2025-10-09"
status: "draft"
tags: ["standards", "api", "http", "rest", "2025.10.2"]
---

# HTTP REST Standard

## Scope

Applies to Fulmen workhorse and service repositories exposing HTTP endpoints. Establishes naming, versioning,
payload schemas, and error handling conventions to ensure cross-language consistency.

## Endpoint Design

- Base path includes version prefix: `/api/v1/` (CalVer-based services MAY expose `/v2025.10/` when warranted).
- Use plural nouns for resource collections (`/api/v1/jobs`), nested routes for sub-resources (`/api/v1/jobs/{id}/runs`).
- Prefer query parameters for filtering/sorting, request body for complex search payloads.
- Health endpoints live at top-level (`/health/live`, `/health/ready`, `/health/startup`).

## Request & Response Schema

- All JSON payloads MUST validate against schemas under `schemas/api/http/`.
- Success envelope: `{ "success": true, "message": string?, "data": object? }`.
- Error envelope: `{ "success": false, "error": { "code": string, "message": string, "details"?: object } }`.
- Include `requestId` and `correlationId` headers in responses for traceability.
- Canonical schemas:
  - `schemas/api/http/v1.0.0/success-response.schema.json`
  - `schemas/api/http/v1.0.0/error-response.schema.json`
  - `schemas/api/http/v1.0.0/health-response.schema.json`
  - `schemas/api/http/v1.0.0/version-response.schema.json`
- Example payloads live under `examples/api/http/v1.0.0/`.

## HTTP Status Codes

- 2xx for success (`200 OK`, `201 Created`, `202 Accepted`).
- 4xx for client issues (`400 Bad Request`, `404 Not Found`, `422 Unprocessable Entity`).
- 5xx for server faults (`500 Internal Server Error`, `503 Service Unavailable`).
- Avoid `200` for errors; use structured error envelope with appropriate status code.

## Headers

- `Content-Type: application/json; charset=utf-8` for JSON responses.
- `X-Request-ID` and `X-Correlation-ID` echoed from request or generated anew.
- `Cache-Control` explicit for each endpoint (default `no-store` for mutable data).
- `Accept: application/json` required from clients; respond with `406` otherwise.

## Pagination

- Use cursor-based pagination: `GET /api/v1/items?cursor=<token>&limit=50`.
- Response includes `nextCursor` (string or `null`) and `items` array.
- Document maximum page size per endpoint.

## Security

- Support bearer tokens (JWT/OIDC) or API keys depending on service classification.
- Enforce TLS; HTTP-only endpoints limited to local development.
- Document required scopes/permissions in OpenAPI (future deliverable).

## Observability

- Instrument HTTP server middleware to log method, path, status, latency, request/correlation IDs.
- Expose Prometheus metrics (`http_requests_total`, `http_request_duration_seconds`).

## Schema Artifacts

- Schemas stored under `schemas/api/http/v1.0.0/` with JSON Schema draft 2020-12.
- Provide README describing available schemas (`health-response.schema.json`, `error-response.schema.json`).
- Run validation via `goneat schema validate-data` in CI.

## Related Documents

- `docs/standards/repository-structure/README.md`
- `docs/standards/library/modules/ssot-sync.md`
- `docs/standards/observability/logging.md`

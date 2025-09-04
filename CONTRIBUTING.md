# Contributing to Goneat (ALPHA)

Thanks for your interest in Goneat! We’re currently in the ALPHA phase. We value your feedback and early testing while we stabilize core features.

## Current posture

- External pull requests: Temporarily paused except by invitation
- Issues and discussions: Welcome — bug reports, feature requests, and UX feedback
- Security reports: Please report privately to the maintainers; coordinated disclosure only

Rationale: During ALPHA we iterate quickly, may make breaking changes, and are prioritizing velocity and internal dogfooding. This posture prevents churn for contributors while we converge on stable APIs.

## How to help today

- Try the latest release and file issues with clear repro steps
- Share use cases and environment details (OS, shell, repo size)
- Propose design ideas in issues before coding; we can provide guidance

## Road to BETA

We expect to accept public PRs at BETA when APIs stabilize and coverage gates increase. Until then, maintainers may tag issues as “help wanted (invited)” for targeted contributions.

## Development basics

- Build: `make build` (artifacts in `dist/`)
- Tests: `make test` (coverage gating via `make coverage-check`)
- Formatting/Linting: Use `goneat assess` targets in the Makefile

## Code quality

- Tests: Include unit and/or integration tests where practical
- Style: Follow existing patterns and the Go community style
- Documentation: Update README or docs for user-visible changes

## Attribution

- Follow the Agentic Attribution Standard for commits in this repository when applicable
- All AI agent contributions require human supervision and clear attribution

Thanks for helping make Goneat better!

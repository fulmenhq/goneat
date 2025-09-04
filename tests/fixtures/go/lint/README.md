# Lint Test Fixtures

This directory contains Go files with intentional lint violations for testing purposes.

## Files

### errcheck.go

Contains intentional unchecked errors to test linting behavior:

- `fmt.Fprintf` without error checking (line 10)
- `os.WriteFile` without error checking (line 11)

These violations are **intentionally left unchecked** to:

1. Test that lint tools correctly identify errcheck issues
2. Provide examples for documentation
3. Ensure lint configurations work as expected

## Usage

These files should **never be fixed** as they serve as test fixtures for lint validation. If lint tools report these as issues, they are working correctly.

## Suppression

The files include appropriate nolint comments to prevent false positives in CI/CD pipelines while still allowing manual testing of lint behavior.

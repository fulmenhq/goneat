#!/bin/bash
# Mock syft that only outputs legacy text format for version
# Used to test multiline version parsing fallback

case "$1" in
  version)
    # Always output legacy text format (even with --output json)
    cat <<EOF
Application:   syft
Version:       1.33.0
BuildDate:     2024-01-01T00:00:00Z
GitCommit:     abc123def456
GitDescription: v1.33.0
Platform:      linux/amd64
EOF
    ;;
  scan)
    # Still support scan for version test
    cat <<'EOF'
{
  "bomFormat": "CycloneDX",
  "specVersion": "1.5",
  "version": 1,
  "components": []
}
EOF
    ;;
  *)
    echo "Unknown command: $1" >&2
    exit 1
    ;;
esac

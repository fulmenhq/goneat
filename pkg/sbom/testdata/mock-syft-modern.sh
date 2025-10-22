#!/bin/bash
# Mock syft binary that validates modern API usage and generates valid SBOM

# Log all arguments for test verification
echo "MOCK_SYFT_ARGS: $*" >&2

case "$1" in
  version)
    if [[ "$2" == "--output" && "$3" == "json" ]]; then
      echo '{"version":"1.33.0"}'
    else
      # Legacy text format
      cat <<EOF
Application:   syft
Version:       1.33.0
BuildDate:     2024-01-01
GitCommit:     abc123
EOF
    fi
    ;;
  scan)
    # Validate modern API usage and extract output path
    found_output=false
    output_path=""
    prev_arg=""
    for arg in "$@"; do
      if [[ "$prev_arg" == "--output" ]]; then
        found_output=true
        # Extract path from FORMAT=PATH or FORMAT (stdout)
        if [[ "$arg" == *"="* ]]; then
          output_path="${arg#*=}"
        fi
        break
      fi
      prev_arg="$arg"
    done

    if ! $found_output; then
      echo "ERROR: Modern --output FORMAT syntax not detected" >&2
      exit 1
    fi

    # Generate valid CycloneDX SBOM
    sbom_content=$(cat <<'EOF'
{
  "bomFormat": "CycloneDX",
  "specVersion": "1.5",
  "version": 1,
  "metadata": {
    "timestamp": "2025-01-15T10:30:00Z",
    "tools": {
      "components": [
        {
          "type": "application",
          "author": "anchore",
          "name": "syft",
          "version": "1.33.0"
        }
      ]
    },
    "component": {
      "type": "application",
      "name": "fixture-project"
    }
  },
  "components": [
    {
      "bom-ref": "pkg:golang/github.com/google/uuid@v1.6.0",
      "type": "library",
      "name": "github.com/google/uuid",
      "version": "v1.6.0",
      "purl": "pkg:golang/github.com/google/uuid@v1.6.0"
    },
    {
      "bom-ref": "pkg:golang/gopkg.in/yaml.v3@v3.0.1",
      "type": "library",
      "name": "gopkg.in/yaml.v3",
      "version": "v3.0.1",
      "purl": "pkg:golang/gopkg.in/yaml.v3@v3.0.1"
    },
    {
      "bom-ref": "pkg:golang/gopkg.in/check.v1@v0.0.0-20161208181325-20d25e280405",
      "type": "library",
      "name": "gopkg.in/check.v1",
      "version": "v0.0.0-20161208181325-20d25e280405",
      "purl": "pkg:golang/gopkg.in/check.v1@v0.0.0-20161208181325-20d25e280405"
    }
  ],
  "dependencies": [
    {
      "ref": "pkg:golang/github.com/google/uuid@v1.6.0",
      "dependsOn": []
    },
    {
      "ref": "pkg:golang/gopkg.in/yaml.v3@v3.0.1",
      "dependsOn": ["pkg:golang/gopkg.in/check.v1@v0.0.0-20161208181325-20d25e280405"]
    },
    {
      "ref": "pkg:golang/gopkg.in/check.v1@v0.0.0-20161208181325-20d25e280405",
      "dependsOn": []
    }
  ]
}
EOF
)

    # Write to file if path specified, otherwise stdout
    if [[ -n "$output_path" ]]; then
      mkdir -p "$(dirname "$output_path")"
      echo "$sbom_content" > "$output_path"
    else
      echo "$sbom_content"
    fi
    ;;
  *)
    echo "Unknown command: $1" >&2
    exit 1
    ;;
esac

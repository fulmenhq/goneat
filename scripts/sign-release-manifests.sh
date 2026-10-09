#!/usr/bin/env bash
set -euo pipefail
# Both original manifests, both signing tools and independent trust are mandatory.
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
exec python3 "$SCRIPT_DIR/release-assets.py" sign "${1:?'usage: sign-release-manifests.sh <tag> [directory]'}" "${2:-dist/release}"

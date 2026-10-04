#!/usr/bin/env bash
set -euo pipefail

# go-licenses identifies stdlib against its startup GOROOT. Resolve that from
# the same selected toolchain that go list will use, not the scanner compiler.
goroot=$(go env GOROOT)
if [[ -z "$goroot" || "$goroot" != /* || ! -d "$goroot/src" ]]; then
	echo "Cannot resolve the selected Go toolchain GOROOT for license collection" >&2
	exit 1
fi
exec env GOROOT="$goroot" go-licenses "$@"

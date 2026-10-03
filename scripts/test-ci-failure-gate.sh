#!/usr/bin/env bash
set -euo pipefail
# Disposable module: the real Make test-unit recipe must propagate go test failure.
ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
fixture=$(mktemp -d)
trap 'rm -rf "$fixture"' EXIT
printf 'module example.test/required-failure\n\ngo 1.26.0\n' >"$fixture/go.mod"
cat >"$fixture/failure_test.go" <<'GO'
package failure
import "testing"
func TestRequiredFailure(t *testing.T) { t.Fatal("required failure control") }
GO
if make -C "$fixture" -f "$ROOT/Makefile" test-unit >"$fixture/result.log" 2>&1; then
	echo "Required failing test was incorrectly accepted" >&2
	cat "$fixture/result.log" >&2
	exit 1
fi
grep -q 'required failure control' "$fixture/result.log"
echo "Required unit-test failure propagated through make test-unit"

#!/usr/bin/env bash
#
# test-ensure-go.sh
# Deterministic tests for the ensure_go / pin_version_cmp logic in
# scripts/cloud-agent-install.sh. The functions are extracted verbatim from
# the script (so the test tracks the real implementation), stub tools are
# generated in a temp dir, and fixed cases are asserted. No network.

set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
INSTALL_SCRIPT="$SCRIPT_DIR/scripts/cloud-agent-install.sh"

fail=0
tmpdir="$(mktemp -d)"
trap 'rm -rf "$tmpdir"' EXIT

# Extract a function (name + body) verbatim from the install script.
extract_fn() {
	local fn="$1"
	awk -v fn="$fn" '
		$0 ~ "^" fn "\\(\\) \\{" { f=1; brace=0 }
		f {
			print
			n=gsub(/\{/,"{"); brace+=n
			n2=gsub(/\}/,"}"); brace-=n2
			if (brace==0) exit
		}
	' "$INSTALL_SCRIPT"
}

eval "$(extract_fn pin_version_cmp)"
eval "$(extract_fn ensure_go)"

# Stub tool factory: creates a tool that prints a fixed --version output.
make_tool() {
	local name="$1" out="$2" mode="${3:-version}"
	local dir="$tmpdir/bin"
	mkdir -p "$dir"
	cat >"$dir/$name" <<EOF
#!/bin/bash
if [ "\$1" = "--version" ]; then
	if [ "$mode" = "reject" ]; then exit 1; fi
	printf '%s\n' "$out"
	exit 0
fi
exit 0
EOF
	chmod +x "$dir/$name"
}

make_tool t_bare "2.11.2"
make_tool t_vpre "v1.2.3"
make_tool t_reject "unused" reject
make_tool t_empty "" reject

# --- pin_version_cmp cases -------------------------------------------------
expect() {
	local desc="$1" want="$2"
	shift 2
	pin_version_cmp "$@"
	local rc=$?
	if [ "$rc" -eq "$want" ]; then
		echo "✅ $desc"
	else
		echo "❌ $desc: want rc=$want got rc=$rc"
		fail=1
	fi
}

expect "bare version matches bare pin" 0 "$tmpdir/bin/t_bare" "2.11.2"
expect "bare version matches v-pin" 0 "$tmpdir/bin/t_bare" "v2.11.2"
expect "v-prefixed output matches v-pin" 0 "$tmpdir/bin/t_vpre" "v1.2.3"
expect "v-prefixed output matches bare pin" 0 "$tmpdir/bin/t_vpre" "1.2.3"
expect "bare mismatch detected" 1 "$tmpdir/bin/t_bare" "2.12.0"
expect "v-pin mismatch detected" 1 "$tmpdir/bin/t_vpre" "v1.2.4"
expect "tool rejects --version -> no verdict (2)" 2 "$tmpdir/bin/t_reject" "v1.0.0"
expect "empty version output -> no verdict (2)" 2 "$tmpdir/bin/t_empty" "v1.0.0"

# --- ensure_go end-to-end with stubs on PATH -------------------------------
export PATH="$tmpdir/bin:$PATH"

out_mismatch=$(ensure_go t_bare "example.com/t_bare@v2.12.0" 2>&1)
if printf '%s' "$out_mismatch" | grep -q "WARN.*does not match pin"; then
	echo "✅ ensure_go WARNs on determined mismatch"
else
	echo "❌ ensure_go did not WARN on mismatch: $out_mismatch"
	fail=1
fi

out_match=$(ensure_go t_bare "github.com/example/t_bare@v2.11.2" 2>&1)
if printf '%s' "$out_match" | grep -q "WARN"; then
	echo "❌ ensure_go warned on match (false positive)"
	fail=1
else
	echo "✅ ensure_go silent on match"
fi

# goimports/go-licenses-style tool (rejects --version) must produce no WARN.
out_reject=$(ensure_go t_reject "github.com/x/tools/cmd/goimports@v0.48.0" 2>&1)
if printf '%s' "$out_reject" | grep -q "WARN"; then
	echo "❌ ensure_go warned on indeterminate version (should stay silent)"
	fail=1
else
	echo "✅ ensure_go silent on indeterminate (rejects --version)"
fi

if [ "$fail" -eq 0 ]; then
	echo "ALL PASS"
	exit 0
fi
exit 1

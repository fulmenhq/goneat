#!/usr/bin/env bash
# Cursor Cloud Agent install script for goneat.
#
# Runs on the default Cursor base image (which already provides Go, Node,
# Python, and Cargo on the standard PATH) and bootstraps the dev toolchain
# goneat drives from a bare image. It is deliberately image-agnostic so the
# same script works for local/other-agent bootstrap too. It:
#   1. primes the Go module cache,
#   2. installs any dev tool that is not already on PATH, and
#   3. builds the goneat binary with embedded assets.
#
# It is idempotent: re-running is safe and skips tools that already resolve on
# PATH (presence only; it does not replace a mismatched version). Installed
# tools go into ~/.local/bin, which is on PATH via ~/.profile, so no system
# paths or shell profiles are mutated.
#
# Tool versions are pinned to recommended_version in
# config/tools/foundation-tools-defaults.yaml (dev↔CI parity). Do not use
# @latest: golangci-lint v2.13+ requires Go >= 1.26.
#
# The default Cursor image's /usr/bin/go is often Go 1.22 with GOTOOLCHAIN=auto,
# which reports go1.26.0 for this module but will jump to a cached newer
# toolchain (e.g. go1.27.x) during `go install`. Pin GOTOOLCHAIN to the 1.26
# line so pinned tools do not follow that jump. Use 1.26.6 (not go.mod's
# 1.26.0) to match the CI runner and pick up standard-library security fixes.
# Override with GOTOOLCHAIN=... when a newer 1.26 patch is required.
set -euo pipefail
export GOTOOLCHAIN="${GOTOOLCHAIN:-go1.26.6}"

# Pins: config/tools/foundation-tools-defaults.yaml recommended_version.
# goimports has no recommended pin there; match go.mod's golang.org/x/tools.
GOLANGCI_LINT_VERSION="v2.12.2"
GOIMPORTS_VERSION="v0.48.0"
GOSEC_VERSION="v2.28.0"
GOVULNCHECK_VERSION="v1.6.0"
YAMLFMT_VERSION="v0.21.0"
SHFMT_VERSION="v3.13.1"
ACTIONLINT_VERSION="v1.7.12"
GO_LICENSES_VERSION="v2.0.1"
CHECKMAKE_VERSION="v0.3.0"
PRETTIER_VERSION="3.9.6"
YAMLLINT_VERSION="1.38.0"
SHELLCHECK_VERSION="v0.11.0"
# arch-specific SHA256 of the upstream shellcheck release tarball (verified 2026-09-01)
SHELLCHECK_SHA256_X86_64="8c3be12b05d5c177a04c29e3c78ce89ac86f1595681cab149b65b97c4e227198"
SHELLCHECK_SHA256_AARCH64="12b331c1d2db6b9eb13cfca64306b1b157a86eb69db83023e261eaa7e7c14588"

echo "=== cloud-agent-install: environment ==="
id 2>/dev/null || true
echo "HOME=${HOME} PWD=$(pwd)"
echo "GOTOOLCHAIN=${GOTOOLCHAIN}"
go version || true
echo "GOPATH=$(go env GOPATH) GOMODCACHE=$(go env GOMODCACHE)"

LOCAL_BIN="${HOME}/.local/bin"
mkdir -p "${LOCAL_BIN}"
case ":${PATH}:" in
*":${LOCAL_BIN}:"*) : ;;
*) export PATH="${LOCAL_BIN}:${PATH}" ;;
esac
export GOBIN="${LOCAL_BIN}"

echo "=== cloud-agent-install: priming Go module cache ==="
go mod download

# pin_version_cmp <tool> <pinned-token>
# Compare a tool's self-reported version against the pinned version token.
# Returns 0 = match (optional leading v tolerated), 1 = determined mismatch,
# 2 = indeterminate (tool rejects --version or prints no recognizable
# version). The caller treats 2 as "no verdict" per the best-effort contract.
pin_version_cmp() {
	tool="$1"
	pinned_norm="${2#v}"
	out="$("$tool" --version 2>/dev/null)" || return 2
	[ -z "$out" ] && return 2
	inst=$(printf '%s' "$out" | grep -oE 'v?[0-9]+\.[0-9]+(\.[0-9]+)?' | head -n 1 | sed 's/^v//')
	[ -z "$inst" ] && return 2
	[ "$inst" = "$pinned_norm" ] && return 0 || return 1
}

# ensure_go <binary> <go-install-package>
# Installs a Go tool if it is not already on PATH. Presence skips the install
# (cold-install / missing-tool only); the pin is advisory at run time: an
# already-installed binary is NOT downgraded or replaced, and pin_version_cmp
# provides a best-effort version compare that WARNs on a determined mismatch
# (optional leading v normalized; tools without usable --version output get
# no verdict and no warning). Never aborts the whole install on a single
# tool failure; the required-tool check below is the gate.
ensure_go() {
	if command -v "$1" >/dev/null 2>&1; then
		echo "present : $1 -> $(command -v "$1")"
		# Advisory pin check — deterministic verdicts, best-effort contract.
		if
			pinned_ver=$(printf '%s' "$2" | sed -n 's/.*@\(v\{0,1\}[0-9][0-9a-zA-Z.-]*\)$/\1/p')
			[ -n "$pinned_ver" ]
		then
			if pin_version_cmp "$1" "$pinned_ver"; then
				:
			else
				rc=$?
				if [ "$rc" -eq 1 ]; then
					echo "WARN    : $1 installed version does not match pin $pinned_ver (advisory only; not corrected)"
				fi
				# rc=2: tool version indeterminate -> no verdict, no warning
			fi
		fi
	else
		echo "install : $1 ($2)"
		go install "$2" || echo "WARN    : failed to install $1"
	fi
}

echo "=== cloud-agent-install: ensuring Go dev tools ==="
ensure_go golangci-lint "github.com/golangci/golangci-lint/v2/cmd/golangci-lint@${GOLANGCI_LINT_VERSION}"
ensure_go goimports "golang.org/x/tools/cmd/goimports@${GOIMPORTS_VERSION}"
ensure_go gosec "github.com/securego/gosec/v2/cmd/gosec@${GOSEC_VERSION}"
ensure_go govulncheck "golang.org/x/vuln/cmd/govulncheck@${GOVULNCHECK_VERSION}"
ensure_go yamlfmt "github.com/google/yamlfmt/cmd/yamlfmt@${YAMLFMT_VERSION}"
ensure_go shfmt "mvdan.cc/sh/v3/cmd/shfmt@${SHFMT_VERSION}"
ensure_go actionlint "github.com/rhysd/actionlint/cmd/actionlint@${ACTIONLINT_VERSION}"
ensure_go go-licenses "github.com/google/go-licenses/v2@${GO_LICENSES_VERSION}"
# checkmake's module moved to github.com/checkmake/checkmake and its main lives
# under cmd/checkmake (the old github.com/mrtazz path is not `go install`-able).
ensure_go checkmake "github.com/checkmake/checkmake/cmd/checkmake@${CHECKMAKE_VERSION}"

echo "=== cloud-agent-install: ensuring foundation formatters/linters ==="
command -v yamllint >/dev/null 2>&1 || python3 -m pip install --user --quiet "yamllint==${YAMLLINT_VERSION}" || echo "WARN    : failed to install yamllint"
command -v prettier >/dev/null 2>&1 || npm install -g --prefix "${HOME}/.local" "prettier@${PRETTIER_VERSION}" >/dev/null || echo "WARN    : failed to install prettier"
if ! command -v shellcheck >/dev/null 2>&1; then
	sc_ver="${SHELLCHECK_VERSION}"
	sc_arch="$(uname -m)"
	case "$sc_arch" in
	x86_64) sc_expected_sha="${SHELLCHECK_SHA256_X86_64}" ;;
	aarch64 | arm64) sc_expected_sha="${SHELLCHECK_SHA256_AARCH64}" ;;
	*)
		echo "WARN    : unsupported arch for shellcheck checksum verification: $sc_arch"
		sc_expected_sha=""
		;;
	esac
	sc_tmp="$(mktemp -d)"
	{ curl -fsSL "https://github.com/koalaman/shellcheck/releases/download/${sc_ver}/shellcheck-${sc_ver}.linux.${sc_arch}.tar.xz" -o "${sc_tmp}/sc.tar.xz" &&
		if [ -n "$sc_expected_sha" ]; then
			echo "${sc_expected_sha}  ${sc_tmp}/sc.tar.xz" | sha256sum -c - ||
				{
					echo "ERROR   : shellcheck tarball checksum mismatch; aborting shellcheck install" >&2
					exit 1
				}
		else
			echo "WARN    : no shellcheck checksum pin for this arch; skipping checksum verification"
		fi &&
		tar -xJf "${sc_tmp}/sc.tar.xz" -C "${sc_tmp}" &&
		install -m 0755 "${sc_tmp}/shellcheck-${sc_ver}/shellcheck" "${LOCAL_BIN}/shellcheck"; } ||
		echo "WARN    : failed to install shellcheck"
	rm -rf "${sc_tmp}"
fi

# Gate the build on the tools the core dev loop (build/test/lint/assess) needs.
# Optional extras (checkmake, prettier, yamllint, shellcheck) only warn above.
echo "=== cloud-agent-install: verifying required tools ==="
missing=""
for req in go gofmt make git golangci-lint goimports gosec govulncheck go-licenses yamlfmt shfmt actionlint; do
	command -v "$req" >/dev/null 2>&1 || missing="${missing} ${req}"
done
if [ -n "${missing}" ]; then
	echo "ERROR   : required tools missing after install:${missing}" >&2
	exit 1
fi

echo "=== cloud-agent-install: building goneat ==="
make build

echo "=== cloud-agent-install: tool inventory ==="
for t in go gofmt make git golangci-lint goimports gosec govulncheck go-licenses checkmake \
	yamlfmt shfmt actionlint prettier yamllint shellcheck jq yq rg node npm python3 cargo; do
	printf '  %-16s %s\n' "$t" "$(command -v "$t" 2>/dev/null || echo MISSING)"
done

echo "=== cloud-agent-install: done ==="

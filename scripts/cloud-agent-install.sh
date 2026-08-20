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
# PATH. Installed tools go into ~/.local/bin, which is on PATH via ~/.profile,
# so no system paths or shell profiles are mutated.
#
# Tool versions are pinned to recommended_version in
# config/tools/foundation-tools-defaults.yaml (dev↔CI parity). Do not use
# @latest: golangci-lint v2.13+ requires Go >= 1.26 and will download a new
# toolchain on the Go 1.25 Cursor base image.
set -euo pipefail

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

echo "=== cloud-agent-install: environment ==="
id 2>/dev/null || true
echo "HOME=${HOME} PWD=$(pwd)"
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

# ensure_go <binary> <go-install-package>
# Installs a Go tool if it is not already on PATH. Never aborts the whole
# install on a single tool failure; the required-tool check below is the gate.
ensure_go() {
	if command -v "$1" >/dev/null 2>&1; then
		echo "present : $1 -> $(command -v "$1")"
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
	sc_tmp="$(mktemp -d)"
	{ curl -fsSL "https://github.com/koalaman/shellcheck/releases/download/${sc_ver}/shellcheck-${sc_ver}.linux.${sc_arch}.tar.xz" -o "${sc_tmp}/sc.tar.xz" &&
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

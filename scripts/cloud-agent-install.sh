#!/usr/bin/env bash
# Cursor Cloud Agent install script for goneat.
#
# The environment is based on the same container goneat CI uses
# (ghcr.io/fulmenhq/goneat-tools-runner-glibc), which already ships the Go
# toolchain plus the foundation/lint tooling goneat drives. "The container IS
# the contract" (see docs/cicd/local-runner.md), so this script only:
#   1. primes the Go module cache,
#   2. tops up any dev tool the image does not already provide, and
#   3. builds the goneat binary with embedded assets.
#
# It is idempotent: re-running is safe and skips tools that already resolve on
# PATH. Top-up tools install into ~/.local/bin so no system paths are mutated.
set -euo pipefail

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
# Installs the Go tool only when it is not already provided by the base image.
ensure_go() {
	if command -v "$1" >/dev/null 2>&1; then
		echo "present : $1 -> $(command -v "$1")"
	else
		echo "install : $1 ($2)"
		go install "$2"
	fi
}

echo "=== cloud-agent-install: ensuring Go dev tools ==="
ensure_go golangci-lint github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest
ensure_go goimports golang.org/x/tools/cmd/goimports@latest
ensure_go gosec github.com/securego/gosec/v2/cmd/gosec@latest
ensure_go govulncheck golang.org/x/vuln/cmd/govulncheck@latest
ensure_go yamlfmt github.com/google/yamlfmt/cmd/yamlfmt@latest
ensure_go shfmt mvdan.cc/sh/v3/cmd/shfmt@latest
ensure_go actionlint github.com/rhysd/actionlint/cmd/actionlint@latest
ensure_go go-licenses github.com/google/go-licenses/v2@v2.0.1
ensure_go checkmake github.com/mrtazz/checkmake@latest

echo "=== cloud-agent-install: ensuring foundation formatters/linters ==="
command -v yamllint >/dev/null 2>&1 || python3 -m pip install --user --quiet yamllint || true
command -v prettier >/dev/null 2>&1 || npm install -g --prefix "${HOME}/.local" prettier@3 || true
if ! command -v shellcheck >/dev/null 2>&1; then
	sc_ver="v0.10.0"
	sc_arch="$(uname -m)"
	sc_tmp="$(mktemp -d)"
	{ curl -fsSL "https://github.com/koalaman/shellcheck/releases/download/${sc_ver}/shellcheck-${sc_ver}.linux.${sc_arch}.tar.xz" -o "${sc_tmp}/sc.tar.xz" &&
		tar -xJf "${sc_tmp}/sc.tar.xz" -C "${sc_tmp}" &&
		install -m 0755 "${sc_tmp}/shellcheck-${sc_ver}/shellcheck" "${LOCAL_BIN}/shellcheck"; } ||
		echo "shellcheck install skipped"
	rm -rf "${sc_tmp}"
fi

echo "=== cloud-agent-install: building goneat ==="
make build

echo "=== cloud-agent-install: tool inventory ==="
for t in go gofmt make git golangci-lint goimports gosec govulncheck go-licenses checkmake \
	yamlfmt shfmt actionlint prettier yamllint shellcheck jq yq rg node npm python3 cargo; do
	printf '  %-16s %s\n' "$t" "$(command -v "$t" 2>/dev/null || echo MISSING)"
done

echo "=== cloud-agent-install: done ==="

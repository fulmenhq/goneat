#!/usr/bin/env bash
# Install foundation tools the push-gate tools check requires when the copies
# on PATH are missing or older than the versions below. Those versions are the
# recommended versions in .goneat/tools.yaml.
# Files are written under RUNNER_TEMP or TMPDIR, not the work tree.
# When GITHUB_PATH is set, the bin directory is prepended for later steps.

set -euo pipefail

YAMLLINT_VERSION=1.38.0
SHELLCHECK_VERSION=0.11.0

dest="${RUNNER_TEMP:-${TMPDIR:-/tmp}}/goneat-push-gate-tools"
mkdir -p "${dest}/bin"

version_ge() {
  python3 - "$1" "$2" <<'PY'
import sys

def parts(value):
    return tuple(int(piece) for piece in value.strip().lstrip("v").split("."))

sys.exit(0 if parts(sys.argv[1]) >= parts(sys.argv[2]) else 1)
PY
}

yamllint_version() {
  command -v yamllint >/dev/null 2>&1 || return 1
  yamllint --version 2>/dev/null | awk 'NR == 1 { print $2; exit }'
}

shellcheck_version() {
  command -v shellcheck >/dev/null 2>&1 || return 1
  shellcheck --version 2>/dev/null | awk '/^version:/ { print $2; exit }'
}

need_yamllint=1
if current="$(yamllint_version)" && version_ge "${current}" "${YAMLLINT_VERSION}"; then
  need_yamllint=0
fi

need_shellcheck=1
if current="$(shellcheck_version)" && version_ge "${current}" "${SHELLCHECK_VERSION}"; then
  need_shellcheck=0
fi

if [ "${need_yamllint}" -eq 0 ] && [ "${need_shellcheck}" -eq 0 ]; then
  exit 0
fi

if [ "${need_yamllint}" -eq 1 ]; then
  python3 -m pip install --disable-pip-version-check --no-cache-dir \
    --target "${dest}/py" "yamllint==${YAMLLINT_VERSION}"
  cat > "${dest}/bin/yamllint" <<EOF
#!/bin/sh
export PYTHONPATH="${dest}/py\${PYTHONPATH:+:\$PYTHONPATH}"
exec python3 -m yamllint "\$@"
EOF
  chmod 755 "${dest}/bin/yamllint"
fi

if [ "${need_shellcheck}" -eq 1 ]; then
  case "$(uname -m)" in
    x86_64 | amd64) arch=x86_64; hash=8c3be12b05d5c177a04c29e3c78ce89ac86f1595681cab149b65b97c4e227198 ;;
    aarch64 | arm64) arch=aarch64; hash=12b331c1d2db6b9eb13cfca64306b1b157a86eb69db83023e261eaa7e7c14588 ;;
    *)
      echo "unsupported architecture for shellcheck: $(uname -m)" >&2
      exit 1
      ;;
  esac
  archive="${dest}/shellcheck-v${SHELLCHECK_VERSION}.linux.${arch}.tar.xz"
  curl -fsSL -o "${archive}" \
    "https://github.com/koalaman/shellcheck/releases/download/v${SHELLCHECK_VERSION}/shellcheck-v${SHELLCHECK_VERSION}.linux.${arch}.tar.xz"
  echo "${hash}  ${archive}" | sha256sum -c -
  tar -xJf "${archive}" -C "${dest}/bin" --strip-components=1 \
    "shellcheck-v${SHELLCHECK_VERSION}/shellcheck"
  chmod 755 "${dest}/bin/shellcheck"
  rm -f "${archive}"
fi

if [ -n "${GITHUB_PATH:-}" ]; then
  echo "${dest}/bin" >> "${GITHUB_PATH}"
fi
export PATH="${dest}/bin:${PATH}"
echo "push-gate tools installed in ${dest}/bin"

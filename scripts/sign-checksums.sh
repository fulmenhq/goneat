#!/usr/bin/env bash
set -euo pipefail

usage() {
	cat <<'EOF' >&2
Usage: scripts/sign-checksums.sh [--key <path>] [--pub <path>]

Signs dist/release/SHA256SUMS and SHA512SUMS using:
  • GPG (detached armored signatures: *.asc)
  • Minisign (detached signatures: *.minisig)

Options:
  --key <path>   Path to minisign secret key (overrides MINISIGN_SECRET_KEY env).
                 Defaults to $HOME/.minisign/fulmenhq-release.key if present.
  --pub <path>   Path to minisign public key to copy into
                 dist/release/fulmenhq-release-minisign.pub (optional).

Environment:
  MINISIGN_SECRET_KEY   Secret key path if --key not provided.
  MINISIGN_PUBLIC_KEY_SOURCE   Alternative way to provide public key path.
EOF
}

MINISIGN_SECRET_KEY=${MINISIGN_SECRET_KEY:-}
MINISIGN_PUBLIC_KEY_SOURCE=${MINISIGN_PUBLIC_KEY_SOURCE:-}

while [[ $# -gt 0 ]]; do
	case "$1" in
	--key)
		shift
		MINISIGN_SECRET_KEY=${1:-}
		;;
	--key=*)
		MINISIGN_SECRET_KEY=${1#*=}
		;;
	--pub)
		shift
		MINISIGN_PUBLIC_KEY_SOURCE=${1:-}
		;;
	--pub=*)
		MINISIGN_PUBLIC_KEY_SOURCE=${1#*=}
		;;
	-h | --help)
		usage
		exit 0
		;;
	*)
		echo "Unknown argument: $1" >&2
		usage
		exit 1
		;;
	esac
	shift || true
done

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
DIST_DIR="$REPO_ROOT/dist/release"

for sums in SHA256SUMS SHA512SUMS; do
	if [[ ! -f "$DIST_DIR/$sums" ]]; then
		echo "Missing $DIST_DIR/$sums. Run make package first." >&2
		exit 1
	fi
done

if ! command -v gpg >/dev/null 2>&1; then
	echo "gpg command not found" >&2
	exit 1
fi

if ! command -v minisign >/dev/null 2>&1; then
	echo "minisign command not found" >&2
	exit 1
fi

if [[ -z "$MINISIGN_SECRET_KEY" ]]; then
	default_key="$HOME/.minisign/fulmenhq-release.key"
	if [[ -f "$default_key" ]]; then
		MINISIGN_SECRET_KEY="$default_key"
	else
		echo "Minisign secret key not provided. Use --key or set MINISIGN_SECRET_KEY." >&2
		exit 1
	fi
fi

if [[ ! -f "$MINISIGN_SECRET_KEY" ]]; then
	echo "Minisign secret key not found at $MINISIGN_SECRET_KEY" >&2
	exit 1
fi

if [[ -n "$MINISIGN_PUBLIC_KEY_SOURCE" ]]; then
	if [[ ! -f "$MINISIGN_PUBLIC_KEY_SOURCE" ]]; then
		echo "Minisign public key not found at $MINISIGN_PUBLIC_KEY_SOURCE" >&2
		exit 1
	fi
	cp "$MINISIGN_PUBLIC_KEY_SOURCE" "$DIST_DIR/fulmenhq-release-minisign.pub"
	echo "Copied minisign public key to dist/release/fulmenhq-release-minisign.pub"
fi

for sums in SHA256SUMS SHA512SUMS; do
	echo "Signing $sums with GPG..."
	gpg --armor --detach-sign --output "$DIST_DIR/${sums}.asc" "$DIST_DIR/$sums"
	echo "Signing $sums with minisign..."
	minisign -S -s "$MINISIGN_SECRET_KEY" -m "$DIST_DIR/$sums" -x "$DIST_DIR/${sums}.minisig"
	echo "✅ Completed signatures for $sums"
	if [[ -f "$DIST_DIR/fulmenhq-release-minisign.pub" ]]; then
		minisign -Vm "$DIST_DIR/$sums" -p "$DIST_DIR/fulmenhq-release-minisign.pub" >/dev/null
		echo "   ↳ minisign signature verified with local public key"
	else
		echo "   ↳ minisign public key not present; skipping verification"
	fi
done

echo "All checksum signatures completed. Upload using 'make release-upload' when ready."

#!/usr/bin/env sh
set -euo pipefail

PROJECT="goneat"
OWNER="3leaps"
REPO="goneat"
INSTALL_DIR="/usr/local/bin"
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)

case "$ARCH" in
x86_64 | amd64) ARCH=amd64 ;;
aarch64 | arm64) ARCH=arm64 ;;
*)
	echo "Unsupported architecture: $ARCH" >&2
	exit 1
	;;
esac

case "$OS" in
linux) EXT=tar.gz ;;
darwin) EXT=tar.gz ;;
msys* | mingw* | cygwin*)
	echo "Use install.ps1 on Windows"
	exit 1
	;;
*)
	echo "Unsupported OS: $OS" >&2
	exit 1
	;;
esac

if [ "${SUDO:-}" = "" ] && [ ! -w "$INSTALL_DIR" ]; then
	INSTALL_DIR="$HOME/.local/bin"
	mkdir -p "$INSTALL_DIR"
	echo "Installing to $INSTALL_DIR (not writable: /usr/local/bin)"
fi

LATEST_TAG=$(curl -fsSL "https://api.github.com/repos/$OWNER/$REPO/releases/latest" | sed -n 's/.*"tag_name": *"\(v[^\"]*\)".*/\1/p' | head -n1)
if [ -z "$LATEST_TAG" ]; then
	echo "Failed to resolve latest release tag" >&2
	exit 1
fi
VERSION=${LATEST_TAG#v}
ASSET="${PROJECT}_${VERSION}_${OS}_${ARCH}.${EXT}"
BASE_URL="https://github.com/$OWNER/$REPO/releases/download/$LATEST_TAG"
TMPDIR=$(mktemp -d)
trap 'rm -rf "$TMPDIR"' EXIT

curl -fsSL "$BASE_URL/$ASSET" -o "$TMPDIR/$ASSET"
curl -fsSL "$BASE_URL/SHA256SUMS" -o "$TMPDIR/SHA256SUMS"

(cd "$TMPDIR" && shasum -a 256 -c SHA256SUMS 2>/dev/null | grep " $ASSET: OK" >/dev/null) || {
	# Fallback: grep line and compute
	SUM_EXPECTED=$(grep " $ASSET$" "$TMPDIR/SHA256SUMS" | awk '{print $1}')
	SUM_ACTUAL=$(shasum -a 256 "$TMPDIR/$ASSET" | awk '{print $1}')
	[ "$SUM_EXPECTED" = "$SUM_ACTUAL" ] || {
		echo "Checksum mismatch" >&2
		exit 1
	}
}

case "$EXT" in
tar.gz)
	tar -xzf "$TMPDIR/$ASSET" -C "$TMPDIR"
	;;
zip)
	unzip -q "$TMPDIR/$ASSET" -d "$TMPDIR"
	;;
esac

BIN_NAME="$PROJECT"
[ -f "$TMPDIR/$BIN_NAME" ] || {
	echo "Binary $BIN_NAME not found in archive" >&2
	exit 1
}
chmod +x "$TMPDIR/$BIN_NAME"

mv "$TMPDIR/$BIN_NAME" "$INSTALL_DIR/$BIN_NAME"
echo "Installed $BIN_NAME to $INSTALL_DIR"

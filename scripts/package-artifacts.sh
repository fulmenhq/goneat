#!/usr/bin/env bash
set -euo pipefail

PROJECT="goneat"
VERSION=$(cat VERSION)
BIN_DIR="bin"
OUT_DIR="dist/release"

mkdir -p "$OUT_DIR"
rm -f "$OUT_DIR"/SHA256SUMS

package() {
  local os=$1 arch=$2
  local ext=""
  local bin="$BIN_DIR/${PROJECT}-${os}-${arch}"
  local archive_ext="tar.gz"
  local archive_name="${PROJECT}_${VERSION}_${os}_${arch}.${archive_ext}"

  if [[ "$os" == "windows" ]]; then
    ext=".exe"
    bin="$bin$ext"
    archive_ext="zip"
    archive_name="${PROJECT}_${VERSION}_${os}_${arch}.${archive_ext}"
  fi

  if [[ ! -f "$bin" ]]; then
    echo "Skipping $os/$arch: binary not found: $bin" >&2
    return
  fi

  tmpdir=$(mktemp -d)
  trap 'rm -rf "$tmpdir"' RETURN

  local bin_name="$PROJECT$ext"
  cp "$bin" "$tmpdir/$bin_name"
  chmod +x "$tmpdir/$bin_name"

  case "$archive_ext" in
    tar.gz)
      (cd "$tmpdir" && tar -czf "$OUT_DIR/$archive_name" "$bin_name")
      ;;
    zip)
      (cd "$tmpdir" && zip -q "$OUT_DIR/$archive_name" "$bin_name")
      ;;
  esac

  # sha256sum portable
  if command -v shasum >/dev/null 2>&1; then
    shasum -a 256 "$OUT_DIR/$archive_name" | awk '{print $1"  "$2}' >> "$OUT_DIR/SHA256SUMS"
  elif command -v sha256sum >/dev/null 2>&1; then
    sha256sum "$OUT_DIR/$archive_name" >> "$OUT_DIR/SHA256SUMS"
  else
    echo "No sha256 tool available" >&2; exit 1
  fi

  echo "Packaged $archive_name"
}

# Matrix
package linux amd64
package linux arm64
package darwin amd64
package darwin arm64
package windows amd64

# Optional GPG signing
if [[ "${SIGN:-}" == "1" ]]; then
  if command -v gpg >/dev/null 2>&1; then
    gpg --batch --yes --armor --detach-sign -o "$OUT_DIR/SHA256SUMS.asc" "$OUT_DIR/SHA256SUMS"
    echo "Signed SHA256SUMS -> SHA256SUMS.asc"
  else
    echo "SIGN=1 but gpg not found; skipping signature" >&2
  fi
fi

echo "Artifacts in $OUT_DIR:" && ls -lh "$OUT_DIR"
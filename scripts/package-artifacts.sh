#!/usr/bin/env bash
set -euo pipefail

PROJECT="goneat"
VERSION=$(cat VERSION)
BIN_DIR="bin"
OUT_DIR="dist/release"
# Compute absolute output directory to allow packaging from temp dirs
OUT_DIR_ABS="$(mkdir -p "$OUT_DIR" && cd "$OUT_DIR" && pwd)"

mkdir -p "$OUT_DIR"
rm -f "$OUT_DIR"/SHA256SUMS "$OUT_DIR"/SHA512SUMS

compute_hash() {
	local algo=$1
	local archive_name=$2
	local hash
	case "$algo" in
	256)
		if command -v shasum >/dev/null 2>&1; then
			hash=$(cd "$OUT_DIR_ABS" && shasum -a 256 "$archive_name" | awk '{print $1}')
		elif command -v sha256sum >/dev/null 2>&1; then
			hash=$(cd "$OUT_DIR_ABS" && sha256sum "$archive_name" | awk '{print $1}')
		elif command -v openssl >/dev/null 2>&1; then
			hash=$(cd "$OUT_DIR_ABS" && openssl dgst -sha256 "$archive_name" | sed 's/^.*= //')
		else
			echo "No sha256-capable tool available" >&2
			return 1
		fi
		;;
	512)
		if command -v shasum >/dev/null 2>&1; then
			hash=$(cd "$OUT_DIR_ABS" && shasum -a 512 "$archive_name" | awk '{print $1}')
		elif command -v sha512sum >/dev/null 2>&1; then
			hash=$(cd "$OUT_DIR_ABS" && sha512sum "$archive_name" | awk '{print $1}')
		elif command -v openssl >/dev/null 2>&1; then
			hash=$(cd "$OUT_DIR_ABS" && openssl dgst -sha512 "$archive_name" | sed 's/^.*= //')
		else
			echo "No sha512-capable tool available" >&2
			return 1
		fi
		;;
	*)
		echo "Unsupported hash algorithm: $algo" >&2
		return 1
		;;
	esac
	printf "%s" "$hash"
}

append_checksum() {
	local algo=$1
	local archive_name=$2
	local output_file=$3
	local hash
	if ! hash=$(compute_hash "$algo" "$archive_name"); then
		exit 1
	fi
	printf "%s  %s\n" "$hash" "$archive_name" >>"$OUT_DIR_ABS/$output_file"
}

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
		(cd "$tmpdir" && tar -czf "$OUT_DIR_ABS/$archive_name" "$bin_name")
		;;
	zip)
		(cd "$tmpdir" && zip -q "$OUT_DIR_ABS/$archive_name" "$bin_name")
		;;
	esac

	append_checksum 256 "$archive_name" SHA256SUMS
	append_checksum 512 "$archive_name" SHA512SUMS

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
		for sums in SHA256SUMS SHA512SUMS; do
			if [[ -f "$OUT_DIR/$sums" ]]; then
				gpg --batch --yes --armor --detach-sign -o "$OUT_DIR/${sums}.asc" "$OUT_DIR/$sums"
				echo "Signed $sums -> ${sums}.asc"
			fi
		done
	else
		echo "SIGN=1 but gpg not found; skipping signature" >&2
	fi
fi

echo "Artifacts in $OUT_DIR_ABS:" && ls -lh "$OUT_DIR_ABS"

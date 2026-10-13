#!/usr/bin/env bash
# Verify a detached PGP signature on a goneat checksum manifest with the
# release keys committed in the repository, in a throwaway keyring.
#
# Usage: verify-release-pgp.sh <expected-fingerprints.txt> <release-signing-keys.asc> <manifest.asc> <manifest>
#
# The anchors file names the PGP primary; the keys file must hold exactly
# that primary and one signing subkey. Passes only when gpg succeeds, the
# status holds exactly one VALIDSIG made by that subkey, and no expired,
# revoked, bad or unverifiable signature status is reported.
set -euo pipefail

die() {
	echo "error: $*" >&2
	exit 1
}

[[ $# -eq 4 ]] || die "usage: verify-release-pgp.sh <expected-fingerprints.txt> <release-signing-keys.asc> <manifest.asc> <manifest>"
anchors="$1"
keys="$2"
signature="$3"
manifest="$4"
for file in "$anchors" "$keys" "$signature" "$manifest"; do
	[[ -f "$file" && ! -L "$file" ]] || die "regular file required: $file"
done

home="$(mktemp -d)"
trap 'gpgconf --homedir "$home" --kill all >/dev/null 2>&1 || true; rm -rf "$home"' EXIT
chmod 700 "$home"

PRIMARY="$(awk 'NR==1 && $1=="gpg" {print $2}' "$anchors")"
[[ "$PRIMARY" =~ ^[0-9A-F]{40}$ ]] || die "anchors file must start with a gpg <primary fingerprint> line"

gpg --batch --quiet --homedir "$home" --import "$keys" 2>/dev/null || die "key import failed"
listing="$(gpg --batch --homedir "$home" --with-colons --with-subkey-fingerprint --list-keys)"
[[ "$(awk -F: '$1=="pub" {n++} END {print n+0}' <<<"$listing")" == 1 ]] || die "keys file must hold exactly one primary"
[[ "$(awk -F: '$1=="fpr" {print $10; exit}' <<<"$listing")" == "$PRIMARY" ]] || die "keys file primary differs from anchor $PRIMARY"
SUBKEY="$(awk -F: '$1=="sub" {s=1; cap=$12; next} s && $1=="fpr" {if (cap ~ /s/) print $10; s=0}' <<<"$listing")"
[[ "$SUBKEY" =~ ^[0-9A-F]{40}$ ]] || die "keys file must hold exactly one signing subkey"

status="$home/status"
if ! gpg --batch --homedir "$home" --no-auto-key-retrieve --status-fd 3 \
	--verify "$signature" "$manifest" 3>"$status" 2>/dev/null; then
	die "gpg verification failed"
fi
if grep -Eq '^\[GNUPG:\] (EXPKEYSIG|EXPSIG|REVKEYSIG|KEYREVOKED|BADSIG|ERRSIG) ' "$status"; then
	die "expired, revoked, bad or unverifiable signature"
fi
valid="$(grep -c '^\[GNUPG:\] VALIDSIG ' "$status" || true)"
[[ "$valid" == 1 ]] || die "exactly one valid signature required (found $valid)"
read -r -a record < <(grep '^\[GNUPG:\] VALIDSIG ' "$status")
[[ "${record[2]}" == "$SUBKEY" ]] || die "signature not made by pinned signing subkey $SUBKEY"
[[ "${record[${#record[@]} - 1]}" == "$PRIMARY" ]] || die "signing key not bound to pinned primary $PRIMARY"
echo "Good signature on $manifest by pinned signing subkey $SUBKEY"

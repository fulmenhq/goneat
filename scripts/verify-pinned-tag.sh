#!/usr/bin/env bash
# Verify a signed release tag against the public pin committed in the tagged
# commit, in an isolated keyring.
#
# Trust data is read as inert git objects from the tagged commit, never from the
# working tree or the environment (3leaps/crucible PDR-0008 §2.1), and the pins
# are checked against their fingerprint anchors by decernor, bound through
# GONEAT_DECERNOR_BIN (release-verify-anchors.sh). The tag must be annotated,
# name itself, point at a commit whose VERSION is the tag, carry exactly one
# good, unexpired, unrevoked signature made by the pinned signing subkey of
# the anchored primary, and (current layout) be tagged by the identity in
# config/release/tagger-identity.txt. Tags v0.6.4 to v0.6.9 use the legacy pin
# layout, which committed no tagger identity; their tagger is reported, not
# compared.
#
# Usage: verify-pinned-tag.sh TAG   (required; never derived)
set -euo pipefail

die() {
	echo "error: $*" >&2
	exit 1
}

[[ $# -eq 1 ]] || die 'usage: verify-pinned-tag.sh TAG (the release tag is never derived)'
root="$(cd "$(dirname "$0")/.." && pwd -P)"
# Resolve the checker before anything else is read or written.
# shellcheck source=scripts/release-decernor.sh
source "$root/scripts/release-decernor.sh"
resolve_release_decernor
cd "$root"
tag="$1"
[[ "$tag" =~ ^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(-(rc|beta|alpha)\.[0-9]+)?$ ]] ||
	die "canonical release tag vX.Y.Z required: $tag"
object="$(git rev-parse -q --verify "refs/tags/$tag")" || die "tag $tag absent"
[[ "$(git cat-file -t "$object")" == tag ]] || die "$tag must be an annotated tag"
[[ "$(git cat-file tag "$object" | sed -n '/^$/q; s/^tag //p')" == "$tag" ]] || die "$tag object names a different tag"
target="$(git rev-parse "${object}^{commit}")"
[[ "$(git cat-file blob "$target:VERSION" 2>/dev/null | tr -d '\r\n')" == "$tag" ]] ||
	die "$tag does not match VERSION in the tagged commit"

work="$(mktemp -d)"
trap 'gpgconf --homedir "$work/home" --kill all >/dev/null 2>&1 || true; rm -rf "$work"' EXIT
chmod 700 "$work"
mkdir -m 700 "$work/pins"
"$root/scripts/release-verify-anchors.sh" "$target" "$work/pins" >/dev/null
layout="$(cat "$work/pins/layout")"

tagger="$(git cat-file tag "$object" | sed -n '/^$/q; s/^tagger \(.*\) [0-9][0-9]* [+-][0-9][0-9][0-9][0-9]$/\1/p')"
if [[ "$layout" == current ]]; then
	[[ "$tagger" == "$(cat "$work/pins/tagger-identity.txt")" ]] ||
		die "tagger '$tagger' does not match the committed tagger identity"
	tagger_note="matches the committed identity"
else
	tagger_note="not compared (no committed tagger identity in the legacy layout)"
fi

mkdir -m 700 "$work/home"
export GNUPGHOME="$work/home"
gpg --batch --quiet --import "$work/pins/pin.asc" 2>/dev/null || die 'pin import failed'
listing="$(gpg --batch --with-colons --with-subkey-fingerprint --list-keys)"
[[ -z "$(gpg --batch --with-colons --list-secret-keys)" ]] || die 'pin contains secret material'
[[ "$(awk -F: '$1=="pub" {n++} END {print n+0}' <<<"$listing")" == 1 ]] || die 'pin must hold exactly one primary key'
primary="$(awk '$1=="gpg" {print $2}' "$work/pins/anchors.txt")"
signing="$(awk -F: '$1=="sub" {s=1; cap=$12; next} s && $1=="fpr" {if (cap ~ /s/) print $10; s=0}' <<<"$listing")"
[[ "$signing" =~ ^[0-9A-F]{40}$ ]] || die 'pin must hold exactly one signing subkey'

git -c gpg.format=openpgp -c gpg.program=gpg -c gpg.openpgp.program=gpg \
	verify-tag --raw "$object" >"$work/status" 2>&1 || {
	awk '$1=="[GNUPG:]" {print "signature status:", $2}' "$work/status" >&2
	die "$tag signature is invalid under the pin"
}
awk -v primary="$primary" -v signing="$signing" '
	$1=="[GNUPG:]" && $2=="VALIDSIG" {valid++; subkey=$3; signer_primary=$NF}
	$1=="[GNUPG:]" && $2=="GOODSIG" {good++}
	$1=="[GNUPG:]" && $2 ~ /^(EXPKEYSIG|EXPSIG|REVKEYSIG|KEYREVOKED|BADSIG|ERRSIG)$/ {bad++}
	END { if (bad || good != 1 || valid != 1 || signer_primary != primary || subkey != signing) exit 1 }
' "$work/status" || die "$tag is not a single good signature by the pinned signing subkey"

echo "[ok] $tag ($object -> $target) signed by pinned subkey $signing of $primary; $layout pin layout; tagger $tagger_note"

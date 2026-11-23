#!/usr/bin/env bash
# Verify the release pins committed at COMMIT against their fingerprint anchors.
#
# The pins and anchors are read as inert git objects from COMMIT, never from
# the working tree or the environment (3leaps/crucible PDR-0008 §2.1), and are
# checked only by the designated checker, decernor, bound through
# GONEAT_DECERNOR_BIN (§2.2, §2.3).
#
# Two layouts exist, and a commit carries exactly one of them:
#   current  docs/security/goneat-minisign.pub, keys/expected-fingerprints.txt
#            and .ndjson, config/release/tagger-identity.txt
#   legacy   docs/security/release-minisign.pub and keys/expected-fingerprints.txt.
#            Accepted only for the tags that shipped it, v0.6.4 through v0.6.9,
#            named as TAG and resolving to COMMIT; any other commit carrying it
#            is refused. decernor derives the records from the committed
#            exports and verifies the committed text anchors.
# Both carry docs/security/release-signing-keys.asc. Commits before v0.6.4
# carry no pin and are refused.
#
# Usage: release-verify-anchors.sh COMMIT OUTPUT_DIR [TAG]
#   TAG is the release tag COMMIT is verified for; only a legacy-layout tag
#   needs it. OUTPUT_DIR must be an existing, empty directory. The verified blobs are
#   left there for the caller: anchors.txt, anchors.ndjson, pin.asc,
#   minisign.pub, layout, and tagger-identity.txt (current layout only).
set -euo pipefail

die() {
	echo "error: $*" >&2
	exit 1
}

[[ $# -eq 2 || $# -eq 3 ]] || die 'usage: release-verify-anchors.sh COMMIT OUTPUT_DIR [TAG]'
tag="${3:-}"
# The only tags whose commits carry the legacy pin layout.
legacy_tags=" v0.6.4 v0.6.5 v0.6.6 v0.6.7 v0.6.8 v0.6.9 "
commit_ref="$1"
out="$2"
root="$(cd "$(dirname "$0")/.." && pwd -P)"
# shellcheck source=scripts/release-decernor.sh
source "$root/scripts/release-decernor.sh"
resolve_release_decernor
export RELEASE_DECERNOR_BIN
cd "$root"

[[ -d "$out" && ! -L "$out" ]] || die "output directory $out must exist"
[[ -z "$(ls -A "$out")" ]] || die "output directory $out must be empty"
commit="$(git rev-parse -q --verify "${commit_ref}^{commit}")" || die "commit $commit_ref absent"

mode_of() { git ls-tree "$commit" -- "$1" | awk '{print $1}'; }
committed() {
	local path="$1" dest="$2"
	[[ "$(mode_of "$path")" == 100644 ]] || die "committed regular file $path required in $commit"
	git cat-file blob "$commit:$path" >"$dest"
	[[ -s "$dest" ]] || die "committed $path is empty in $commit"
}

current_pub=docs/security/goneat-minisign.pub
legacy_pub=docs/security/release-minisign.pub
has_current="$(git ls-tree "$commit" -- "$current_pub")"
has_legacy="$(git ls-tree "$commit" -- "$legacy_pub")"
if [[ -n "$has_current" && -n "$has_legacy" ]]; then
	die "$commit carries both $current_pub and $legacy_pub; a commit has one minisign pin"
elif [[ -n "$has_current" ]]; then
	layout=current
	committed "$current_pub" "$out/minisign.pub"
	committed keys/expected-fingerprints.ndjson "$out/anchors.ndjson"
	committed config/release/tagger-identity.txt "$out/tagger-identity.txt"
elif [[ -n "$has_legacy" ]]; then
	[[ -n "$tag" && "$legacy_tags" == *" $tag "* ]] ||
		die "$commit carries the legacy pin layout, which only tags v0.6.4 through v0.6.9 may use${tag:+ (got $tag)}"
	[[ "$(git rev-parse -q --verify "refs/tags/${tag}^{commit}")" == "$commit" ]] ||
		die "legacy tag $tag does not resolve to $commit"
	layout=legacy
	committed "$legacy_pub" "$out/minisign.pub"
else
	die "$commit carries no committed minisign pin (releases before v0.6.4 predate the pin)"
fi
committed docs/security/release-signing-keys.asc "$out/pin.asc"
committed keys/expected-fingerprints.txt "$out/anchors.txt"

if [[ "$layout" == legacy ]]; then
	# The legacy layout committed text anchors only. decernor derives the
	# records from the committed exports; validation below then requires the
	# committed text anchors to name exactly those values.
	scratch="$(mktemp -d)"
	trap 'rm -rf "$scratch"' EXIT
	"$RELEASE_DECERNOR_BIN" fingerprint "$out/pin.asc" --class public --kind gpg --fail-on-empty \
		--format ndjson --path-mode none --gpg-role primary >"$scratch/gpg.ndjson"
	"$RELEASE_DECERNOR_BIN" fingerprint "$out/minisign.pub" --class public --kind minisign --fail-on-empty \
		--format ndjson --path-mode none >"$scratch/minisign.all.ndjson"
	grep -F '"fingerprint_scheme":"minisign-public-blob-sha256-v1"' "$scratch/minisign.all.ndjson" \
		>"$scratch/minisign.ndjson" || true
	cat "$scratch/gpg.ndjson" "$scratch/minisign.ndjson" >"$out/anchors.ndjson"
fi

"$root/scripts/validate-release-anchors.sh" "$out/anchors.txt" "$out/anchors.ndjson" \
	"$out/pin.asc" "$out/minisign.pub" >/dev/null || die "committed pins differ from their anchors in $commit"
printf '%s\n' "$layout" >"$out/layout"
echo "[ok] committed pins match their anchors in $commit ($layout layout)"

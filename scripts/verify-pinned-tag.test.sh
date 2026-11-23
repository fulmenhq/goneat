#!/usr/bin/env bash
# Tag controls (3leaps/crucible PDR-0004, PDR-0008 §2.5; EPR-0002): a release
# tag verifies only as one good signature by the signing subkey pinned in the
# tagged commit, tagged by the committed identity, at a commit whose VERSION
# is the tag. Throwaway keys only; the same script runs in the tag workflow.
set -euo pipefail
# shellcheck source=scripts/release-test-fixture.sh
source "$(dirname "$0")/release-test-fixture.sh"
decernor="$(fixture_decernor)"
repo="$fixture_dir/repo"
home="$fixture_dir/gpg"
read -r primary sign_a sign_b < <(fixture_keys "$home")
other_home="$fixture_dir/gpg-other"
read -r _ other_sub _ < <(fixture_keys "$other_home")
fixture_repo "$repo" v1.2.3
fixture_pins "$repo" "$home" "$sign_a" "$decernor"
good="$(git -C "$repo" rev-parse HEAD)"
name="${fixture_tagger% <*}"
email="${fixture_tagger#* <}"
email="${email%>}"

# tag <name> <gpg-home> <selector> [tagger] [commit]: a signed annotated tag.
tag() {
	local tag="$1" gnupg="$2" selector="$3" tagger="${4:-$fixture_tagger}" commit="${5:-HEAD}"
	git -C "$repo" tag -d "$tag" >/dev/null 2>&1 || true
	GNUPGHOME="$gnupg" GIT_COMMITTER_NAME="${tagger% <*}" GIT_COMMITTER_EMAIL="${email}" \
		git -C "$repo" -c gpg.format=openpgp -c gpg.program=gpg -c gpg.openpgp.program=gpg \
		tag -s -u "$selector" -m "Release $tag" "$tag" "$commit"
}
verify() { (cd "$repo" && GONEAT_DECERNOR_BIN="$decernor" ./scripts/verify-pinned-tag.sh "$@"); }

tag v1.2.3 "$home" "$sign_a!"
verify v1.2.3 | grep -q "signed by pinned subkey $sign_a of $primary; current pin layout; tagger matches"
echo '[ok] a tag signed by the pinned subkey verifies'

# The checker is bound by GONEAT_DECERNOR_BIN only.
(cd "$repo" && expect_fail_with 'GONEAT_DECERNOR_BIN is required' env -u GONEAT_DECERNOR_BIN \
	PATH="$(dirname "$decernor"):$PATH" ./scripts/verify-pinned-tag.sh v1.2.3)

# The tag is an explicit, canonical argument.
(cd "$repo" && expect_fail_with 'usage' env GONEAT_DECERNOR_BIN="$decernor" ./scripts/verify-pinned-tag.sh)
expect_fail_with 'canonical release tag' verify 1.2.3
expect_fail_with 'absent' verify v9.9.9

# Signer: a sibling subkey of the same primary, or another key, is refused.
tag v1.2.3 "$home" "$sign_b!"
expect_fail_with 'invalid under the pin' verify v1.2.3
tag v1.2.3 "$other_home" "$other_sub!"
expect_fail_with 'invalid under the pin' verify v1.2.3
echo '[ok] signatures outside the pin are refused'

# Tagger identity must be the committed one.
tag v1.2.3 "$home" "$sign_a!" "Someone Else <$email>"
expect_fail_with 'does not match the committed tagger identity' verify v1.2.3

# Unsigned, lightweight, misnamed tags and a VERSION mismatch are refused.
git -C "$repo" tag -d v1.2.3 >/dev/null
GIT_COMMITTER_NAME="$name" GIT_COMMITTER_EMAIL="$email" git -C "$repo" tag -a -m 'Release v1.2.3' v1.2.3
expect_fail_with 'invalid under the pin' verify v1.2.3
git -C "$repo" tag -d v1.2.3 >/dev/null
git -C "$repo" tag v1.2.3
expect_fail_with 'must be an annotated tag' verify v1.2.3
git -C "$repo" tag -d v1.2.3 >/dev/null
tag v1.2.4 "$home" "$sign_a!"
git -C "$repo" update-ref refs/tags/v1.2.3 "$(git -C "$repo" rev-parse refs/tags/v1.2.4)"
expect_fail_with 'names a different tag' verify v1.2.3
expect_fail_with 'does not match VERSION' verify v1.2.4
git -C "$repo" tag -d v1.2.3 v1.2.4 >/dev/null
echo '[ok] unsigned, lightweight, misnamed and mis-versioned tags are refused'

# The pin is read from the tagged commit: a later commit that swaps the pin
# does not change what an existing tag verifies against, and a tagged commit
# whose pin disagrees with its anchors is refused.
tag v1.2.3 "$home" "$sign_a!"
gpg --homedir "$other_home" --batch --armor --export-options export-minimal --export "$other_sub!" \
	>"$repo/docs/security/release-signing-keys.asc"
fixture_commit "$repo" swap-pgp-pin
verify v1.2.3 >/dev/null 2>&1
printf 'v1.2.5\n' >"$repo/VERSION"
fixture_commit "$repo" bump
tag v1.2.5 "$home" "$sign_a!"
expect_fail_with 'differ from their anchors' verify v1.2.5
git -C "$repo" reset -q --hard "$good"
git -C "$repo" tag -d v1.2.5 >/dev/null
# An uncommitted pin in the working tree is never consulted.
gpg --homedir "$other_home" --batch --armor --export-options export-minimal --export "$other_sub!" \
	>"$repo/docs/security/release-signing-keys.asc"
verify v1.2.3 >/dev/null 2>&1
git -C "$repo" checkout -q -- .
echo '[ok] trust comes only from the tagged commit'

# A pin with a second signing subkey is refused.
gpg --homedir "$home" --batch --armor --export-options export-minimal --export "$primary" \
	>"$repo/docs/security/release-signing-keys.asc"
rm -f "$repo/keys/expected-fingerprints.txt" "$repo/keys/expected-fingerprints.ndjson"
git -C "$repo" add -A
fixture_commit "$repo" two-subkeys
(cd "$repo" && GONEAT_DECERNOR_BIN="$decernor" ./scripts/release-insert-anchors.sh >/dev/null 2>&1)
printf 'v1.2.6\n' >"$repo/VERSION"
fixture_commit "$repo" anchors-two-subkeys
tag v1.2.6 "$home" "$sign_a!"
expect_fail_with 'exactly one signing subkey' verify v1.2.6
git -C "$repo" reset -q --hard "$good"

# Legacy layout (tags v0.6.4 to v0.6.9): verifies, tagger reported not compared.
git -C "$repo" mv docs/security/goneat-minisign.pub docs/security/release-minisign.pub
git -C "$repo" rm -q keys/expected-fingerprints.ndjson config/release/tagger-identity.txt
printf 'v1.2.7\n' >"$repo/VERSION"
fixture_commit "$repo" legacy
tag v1.2.7 "$home" "$sign_a!" "Someone Else <$email>"
verify v1.2.7 | grep -q 'legacy pin layout; tagger not compared'
tag v1.2.7 "$home" "$sign_b!"
expect_fail_with 'invalid under the pin' verify v1.2.7
echo '[ok] pinned tag controls passed'

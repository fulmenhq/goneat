#!/usr/bin/env bash
# Anchor controls (3leaps/crucible PDR-0008 §2.2; EPR-0002): derivation reads
# the committed exports, preserves decernor's records byte-for-byte and never
# overwrites; verification refuses anchors that do not match the exports, in
# both the current and the legacy layout. Throwaway keys only.
set -euo pipefail
# shellcheck source=scripts/release-test-fixture.sh
source "$(dirname "$0")/release-test-fixture.sh"
decernor="$(fixture_decernor)"
repo="$fixture_dir/repo"
read -r _ subkey _ < <(fixture_keys "$fixture_dir/gpg")
fixture_repo "$repo" v1.2.3
mkdir -p "$repo/docs/security" "$repo/config/release"
gpg --homedir "$fixture_dir/gpg" --batch --armor --export-options export-minimal --export "$subkey!" \
	>"$repo/docs/security/release-signing-keys.asc"
fixture_minisign_pub "$repo/docs/security/goneat-minisign.pub" 1
printf '%s\n' "$fixture_tagger" >"$repo/config/release/tagger-identity.txt"
fixture_commit "$repo" pins
insert() { (cd "$repo" && GONEAT_DECERNOR_BIN="$decernor" ./scripts/release-insert-anchors.sh); }
verify_at() {
	local out
	out="$(mktemp -d "$fixture_dir/pins.XXXXXX")"
	(cd "$repo" && GONEAT_DECERNOR_BIN="$decernor" ./scripts/release-verify-anchors.sh "$1" "$out")
}

# The checker is required before anything is derived.
(cd "$repo" && expect_fail_with 'GONEAT_DECERNOR_BIN is required' env -u GONEAT_DECERNOR_BIN \
	DECERNOR_BIN="$decernor" ./scripts/release-insert-anchors.sh)
[[ ! -e "$repo/keys/expected-fingerprints.txt" ]]

# The exports come from the committed tree: an uncommitted change is ignored.
fixture_minisign_pub "$repo/docs/security/goneat-minisign.pub" 2
insert >/dev/null 2>&1
git -C "$repo" checkout -q -- docs/security/goneat-minisign.pub
committed_pub="$fixture_dir/committed.pub"
git -C "$repo" cat-file blob HEAD:docs/security/goneat-minisign.pub >"$committed_pub"
"$decernor" fingerprint verify --anchors "$repo/keys/expected-fingerprints.txt" \
	--anchors-ndjson "$repo/keys/expected-fingerprints.ndjson" \
	--gpg "$repo/docs/security/release-signing-keys.asc" --minisign "$committed_pub" >/dev/null 2>&1

# The NDJSON is decernor's own record lines, byte-for-byte.
"$decernor" fingerprint "$repo/docs/security/release-signing-keys.asc" --class public --kind gpg \
	--format ndjson --path-mode none --gpg-role primary >"$fixture_dir/expected.ndjson"
"$decernor" fingerprint "$committed_pub" --class public --kind minisign --format ndjson --path-mode none |
	grep -F '"fingerprint_scheme":"minisign-public-blob-sha256-v1"' >>"$fixture_dir/expected.ndjson"
cmp "$fixture_dir/expected.ndjson" "$repo/keys/expected-fingerprints.ndjson"
echo '[ok] anchors derived by decernor from the committed exports'

# Existing anchors are never overwritten.
cp "$repo/keys/expected-fingerprints.txt" "$fixture_dir/before.txt"
expect_fail_with 'never overwritten' insert
cmp "$fixture_dir/before.txt" "$repo/keys/expected-fingerprints.txt"
fixture_commit "$repo" anchors
good="$(git -C "$repo" rev-parse HEAD)"
verify_at "$good" >/dev/null 2>&1
echo '[ok] existing anchors are never overwritten; committed anchors verify'

# Anchors that do not match the exports are refused.
fixture_minisign_pub "$repo/docs/security/goneat-minisign.pub" 3
fixture_commit "$repo" swap-minisign
expect_fail_with 'differ from their anchors' verify_at HEAD
git -C "$repo" checkout -q "$good" -- docs/security/goneat-minisign.pub
sed 's/^minisign .*/minisign 0000000000000000000000000000000000000000000000000000000000000000/' \
	"$fixture_dir/before.txt" >"$repo/keys/expected-fingerprints.txt"
fixture_commit "$repo" edit-text-anchor
expect_fail_with 'differ from their anchors' verify_at HEAD
git -C "$repo" checkout -q "$good" -- keys
fixture_commit "$repo" restore
verify_at HEAD >/dev/null 2>&1
echo '[ok] mismatched exports and edited text anchors are refused'

# A pin read from the working tree never rescues the committed one.
fixture_minisign_pub "$repo/docs/security/goneat-minisign.pub" 4
fixture_commit "$repo" swap-again
git -C "$repo" checkout -q "$good" -- docs/security/goneat-minisign.pub
expect_fail_with 'differ from their anchors' verify_at HEAD
git -C "$repo" reset -q --hard "$good"
echo '[ok] the working tree is not consulted'

# Layout rules: one minisign pin per commit, the current layout needs its
# records and tagger identity, and a commit without a pin is refused.
git -C "$repo" mv docs/security/goneat-minisign.pub docs/security/release-minisign.pub
git -C "$repo" rm -q keys/expected-fingerprints.ndjson config/release/tagger-identity.txt
fixture_commit "$repo" legacy-layout
legacy="$(git -C "$repo" rev-parse HEAD)"
verify_at "$legacy" | grep -q 'legacy layout'
cp "$repo/docs/security/release-minisign.pub" "$repo/docs/security/goneat-minisign.pub"
fixture_commit "$repo" both-pins
expect_fail_with 'carries both' verify_at HEAD
git -C "$repo" rm -q docs/security/goneat-minisign.pub
fixture_minisign_pub "$repo/docs/security/release-minisign.pub" 5
fixture_commit "$repo" legacy-mismatch
expect_fail_with 'differ from their anchors' verify_at HEAD
git -C "$repo" rm -q docs/security/release-minisign.pub
fixture_commit "$repo" no-pin
expect_fail_with 'carries no committed minisign pin' verify_at HEAD
git -C "$repo" reset -q --hard "$good"
git -C "$repo" rm -q keys/expected-fingerprints.ndjson
fixture_commit "$repo" no-records
expect_fail_with 'expected-fingerprints.ndjson required' verify_at HEAD
git -C "$repo" reset -q --hard "$good"
git -C "$repo" rm -q config/release/tagger-identity.txt
fixture_commit "$repo" no-tagger
expect_fail_with 'tagger-identity.txt required' verify_at HEAD
echo '[ok] layout controls passed (legacy verifies; both, neither, missing records or tagger refused)'

# A missing committed export fails before any write.
git -C "$repo" reset -q --hard "$good"
rm -rf "$repo/keys"
git -C "$repo" rm -q -r --cached keys
git -C "$repo" rm -q docs/security/goneat-minisign.pub
fixture_commit "$repo" drop-pub
expect_fail_with 'committed regular file' insert
[[ ! -e "$repo/keys/expected-fingerprints.txt" ]]
echo '[ok] anchor controls passed'

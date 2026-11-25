#!/usr/bin/env bash
# Negative controls for the explicit release inputs (3leaps/crucible PDR-0008
# §2.1, §2.3, §2.4; EPR-0002): each refusal is reached and fires for its own
# reason. Needs neither gpg nor a real checker: fake checker binaries stand in.
#
# - a bare RELEASE_TAG from the environment or the command line is refused by
#   the Makefile before any target runs
# - every tag-consuming target refuses a missing or non-canonical
#   GONEAT_RELEASE_TAG, and never falls back to VERSION
# - the checker resolver refuses every malformed GONEAT_DECERNOR_BIN and
#   ignores unprefixed variables and PATH
# - no release step reads a minisign public key from the environment

# Fake checker bodies are written in single quotes on purpose: they expand
# when they run.
# shellcheck disable=SC2016

set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT

failures=0
pass() { echo "[ok] $1"; }
fail() {
	echo "[!!] $1" >&2
	failures=$((failures + 1))
}

# Run make in a clean environment plus the given assignments.
# Usage: in_make "NAME=VALUE ..." TARGET [make args...]
in_make() {
	local assignments="$1"
	shift
	# shellcheck disable=SC2086 # Word splitting of the assignment list is intended.
	env -i PATH="$PATH" HOME="$work" $assignments make --no-print-directory -C "$root" "$@" \
		>"$work/out" 2>"$work/err"
}

make_refused() {
	local label="$1" pattern="$2" assignments="$3"
	shift 3
	if in_make "$assignments" "$@"; then
		fail "$label: accepted"
	elif ! grep -q -- "$pattern" "$work/err"; then
		fail "$label: refused for the wrong reason: $(cat "$work/err")"
	else
		pass "$label: refused"
	fi
}

# --- Bare RELEASE_TAG, any origin, any target --------------------------------
make_refused 'bare RELEASE_TAG from the environment' 'RELEASE_TAG is not accepted' \
	'RELEASE_TAG=v0.6.9' release-notes
make_refused 'bare RELEASE_TAG on the command line' 'RELEASE_TAG is not accepted' \
	'' release-tag RELEASE_TAG=v0.6.9
make_refused 'bare RELEASE_TAG beside GONEAT_RELEASE_TAG' 'RELEASE_TAG is not accepted' \
	'GONEAT_RELEASE_TAG=v0.6.9 RELEASE_TAG=v0.6.9' release-upload
make_refused 'bare RELEASE_TAG on a non-release target' 'RELEASE_TAG is not accepted' \
	'RELEASE_TAG=v0.6.9' version

# --- GONEAT_RELEASE_TAG is required on every tag-consuming target -------------
tag_targets=(
	release-notes release-download release-checksums release-verify-checksums
	release-sign release-verify-signatures release-upload release-tag
	release-tag-verify release-tag-push release-push release-verify-pinned-tag
	update-homebrew-formula update-scoop-manifest
)
[[ -s "$root/VERSION" ]] || {
	echo 'error: VERSION is required for the fallback control' >&2
	exit 1
}
for target in "${tag_targets[@]}"; do
	make_refused "$target without GONEAT_RELEASE_TAG (VERSION present)" \
		'GONEAT_RELEASE_TAG=vX.Y.Z is required' '' "$target"
	make_refused "$target with a non-canonical tag" \
		'GONEAT_RELEASE_TAG=vX.Y.Z is required' 'GONEAT_RELEASE_TAG=0.6.9' "$target"
done
make_refused 'tag with trailing text' 'GONEAT_RELEASE_TAG=vX.Y.Z is required' '' \
	release-notes 'GONEAT_RELEASE_TAG=v0.6.9 extra'
# The Makefile gate uses the verifier's canonical form (no leading zeros).
make_refused 'tag with a leading zero' 'GONEAT_RELEASE_TAG=vX.Y.Z is required' '' \
	release-notes 'GONEAT_RELEASE_TAG=v01.2.3'
make_refused 'unknown prerelease label' 'GONEAT_RELEASE_TAG=vX.Y.Z is required' '' \
	release-notes 'GONEAT_RELEASE_TAG=v0.6.9-dev.1'
# Each target's refusal is its first recipe line, so nothing ran before it.
if in_make '' -n release-upload GONEAT_RELEASE_TAG=v9.9.9 &&
	! grep -q 'VERSION' "$work/out" && grep -q 'upload-release-assets.sh "$GONEAT_RELEASE_TAG"' "$work/out"; then
	pass 'release-upload passes GONEAT_RELEASE_TAG, not VERSION'
else
	fail "release-upload does not pass GONEAT_RELEASE_TAG: $(cat "$work/out" "$work/err")"
fi
if GONEAT_RELEASE_TAG='' bash "$root/scripts/generate-release-notes.sh" >"$work/out" 2>"$work/err"; then
	fail 'generate-release-notes.sh accepted an empty tag'
elif GONEAT_RELEASE_TAG=0.6.9 bash "$root/scripts/generate-release-notes.sh" >"$work/out" 2>&1; then
	fail 'generate-release-notes.sh normalized 0.6.9'
else
	pass 'generate-release-notes.sh takes the tag verbatim'
fi

# --- Minisign public key never comes from the environment ---------------------
make_refused 'release-export-minisign-key is gone' 'No rule to make target' \
	"GONEAT_MINISIGN_PUB=$work/k.pub MINISIGN_PUB=$work/k.pub" release-export-minisign-key
# Static control: no release script or the Makefile reads a minisign public key
# variable. Comments and this test are excluded.
if grep -nE 'MINISIGN_PUB|MINISIGN_PUBLIC_KEY_SOURCE|RELEASE_TRUST|RELEASE_PIN_REF|SIGNING_ENV_PREFIX' \
	"$root/Makefile" "$root"/scripts/release-*.sh "$root"/scripts/*release*.py "$root"/scripts/sign-*.sh \
	"$root"/scripts/verify-*.sh "$root"/scripts/upload-release-assets.sh 2>/dev/null |
	grep -v '^[^:]*:[0-9]*:[[:space:]]*#' | grep -v 'release-inputs.test.sh' |
	grep -v 'test_release_assets' >"$work/hits"; then
	fail "release tooling still reads a trust selector from the environment: $(cat "$work/hits")"
else
	pass 'no release step reads a minisign public key or trust selector from the environment'
fi

# --- Checker binding: GONEAT_DECERNOR_BIN only --------------------------------
# A fake checker that reports the given short and extended identity.
fake_checker() {
	local path="$1" short="$2" version="$3"
	cat >"$path" <<EOF
#!/usr/bin/env bash
case "\$*" in
  "version") echo "$short" ;;
  "version -e") printf 'Version:         %s\nCommit:          0000000\nBuild Date:      2026-01-01T00:00:00Z\nGo Version:      go1.0\nGofulmen:        v0.0.0\nCrucible:        v0.0.0\n' "$version" ;;
  *) exit 2 ;;
esac
EOF
	chmod 755 "$path"
}
mkdir "$work/bin" "$work/dir"
fake_checker "$work/bin/decernor" 'decernor 0.1.8' 0.1.8
fake_checker "$work/old" 'decernor 0.1.7' 0.1.7
fake_checker "$work/impostor" 'notdecernor 9.9.9' 9.9.9
fake_checker "$work/mismatch" 'decernor 0.1.9' 0.1.8
cp "$work/bin/decernor" "$work/noexec"
chmod 644 "$work/noexec"
ln -s "$work/bin/decernor" "$work/link"
cat >"$work/badext" <<'EOF'
#!/usr/bin/env bash
case "$*" in
  "version") echo "decernor 0.1.9" ;;
  "version -e") echo "Version: 0.1.9" ;;
esac
EOF
chmod 755 "$work/badext"

# Run resolve_release_decernor in a clean environment plus the assignments.
resolve() {
	env -i PATH="$PATH" HOME="$work" "$@" bash -c \
		'source "$1" && resolve_release_decernor && echo "$RELEASE_DECERNOR_BIN"' \
		_ "$root/scripts/release-decernor.sh" >"$work/out" 2>"$work/err"
}
resolve_refused() {
	local label="$1" pattern="$2"
	shift 2
	if resolve "$@"; then
		fail "checker $label: accepted"
	elif ! grep -q -- "$pattern" "$work/err"; then
		fail "checker $label: refused for the wrong reason: $(cat "$work/err")"
	else
		pass "checker $label: refused"
	fi
}
resolve_refused 'unset' 'GONEAT_DECERNOR_BIN is required'
resolve_refused 'unset, decernor on PATH' 'GONEAT_DECERNOR_BIN is required' PATH="$work/bin:$PATH"
resolve_refused 'unset, unprefixed DECERNOR_BIN' 'GONEAT_DECERNOR_BIN is required' DECERNOR_BIN="$work/bin/decernor"
resolve_refused 'relative path' 'absolute executable regular file' GONEAT_DECERNOR_BIN=bin/decernor
resolve_refused 'bare name' 'absolute executable regular file' GONEAT_DECERNOR_BIN=decernor PATH="$work/bin:$PATH"
resolve_refused 'missing file' 'absolute executable regular file' GONEAT_DECERNOR_BIN="$work/absent"
resolve_refused 'symlink' 'not a symlink' GONEAT_DECERNOR_BIN="$work/link"
resolve_refused 'not executable' 'absolute executable regular file' GONEAT_DECERNOR_BIN="$work/noexec"
resolve_refused 'directory' 'absolute executable regular file' GONEAT_DECERNOR_BIN="$work/dir"
resolve_refused 'wrong identity' 'does not identify as decernor' GONEAT_DECERNOR_BIN="$work/impostor"
resolve_refused 'too old' 'decernor >= 0.1.8 required' GONEAT_DECERNOR_BIN="$work/old"
resolve_refused 'short and extended versions disagree' 'invalid decernor extended identity' GONEAT_DECERNOR_BIN="$work/mismatch"
resolve_refused 'malformed extended identity' 'invalid decernor extended identity' GONEAT_DECERNOR_BIN="$work/badext"
if resolve GONEAT_DECERNOR_BIN="$work/bin/decernor" && [[ "$(cat "$work/out")" == "$work/bin/decernor" ]]; then
	pass 'checker: absolute decernor 0.1.8 accepted'
else
	fail "checker: valid fake refused: $(cat "$work/err")"
fi

# Ceremony scripts resolve the checker before any side effect.
for invocation in 'release-insert-anchors.sh' "release-verify-anchors.sh HEAD $work/dir" 'verify-pinned-tag.sh v9.9.9'; do
	read -r script args <<<"$invocation"
	# shellcheck disable=SC2086 # Word splitting of the argument list is intended.
	if env -i PATH="$work/bin:$PATH" HOME="$work" DECERNOR_BIN="$work/bin/decernor" \
		bash "$root/scripts/$script" $args >"$work/out" 2>"$work/err"; then
		fail "$script ran without GONEAT_DECERNOR_BIN"
	elif ! grep -q 'GONEAT_DECERNOR_BIN is required' "$work/err"; then
		fail "$script refused for the wrong reason: $(cat "$work/err")"
	else
		pass "$script requires GONEAT_DECERNOR_BIN"
	fi
done
[[ -z "$(ls -A "$work/dir")" ]] || fail 'a refused script wrote output'

if [[ "$failures" -ne 0 ]]; then
	echo "release input controls: $failures failure(s)" >&2
	exit 1
fi
echo '[ok] release input controls passed'

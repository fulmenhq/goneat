#!/usr/bin/env bash
# Shared synthetic fixtures for the release-identity tests. Every key is
# generated here, in a temporary GPG home, for the test only; no operator
# keyring or signing key is read. Source this file; it sets fixture_* variables
# and helpers.

fixture_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
# GPG agent Unix socket paths are bounded; macOS TMPDIR can already be long.
fixture_dir="$(cd "$(mktemp -d /tmp/goneat-rid.XXXXXX)" && pwd -P)"
fixture_cleanup() {
	local home
	for home in "$fixture_dir"/gpg*; do
		[[ -d "$home" ]] && gpgconf --homedir "$home" --kill all >/dev/null 2>&1
	done
	rm -rf "$fixture_dir"
}
trap fixture_cleanup EXIT

# The release scripts a fixture repository carries.
fixture_scripts=(
	release-decernor.sh
	release-insert-anchors.sh
	install-release-anchors.sh
	validate-release-anchors.sh
	release-verify-anchors.sh
	verify-pinned-tag.sh
)
fixture_tagger='Synthetic Tagger <tagger@example.invalid>'

fixture_decernor() {
	local bin="${GONEAT_DECERNOR_BIN:-}"
	if [[ -z "$bin" ]]; then
		# Tests may locate a checker; the release scripts themselves never do.
		bin="$(command -v decernor || true)"
	fi
	[[ "$bin" == /* && -x "$bin" ]] || {
		echo 'error: release identity tests need decernor >= 0.1.8 (set GONEAT_DECERNOR_BIN); no skip' >&2
		exit 1
	}
	# Resolve symlinks so the strict resolver accepts the path.
	python3 -c 'import os,sys; print(os.path.realpath(sys.argv[1]))' "$bin"
}

expect_fail() {
	if "$@" >"$fixture_dir/last.out" 2>&1; then
		echo "error: expected rejection: $*" >&2
		cat "$fixture_dir/last.out" >&2
		exit 1
	fi
}

expect_fail_with() {
	local pattern="$1"
	shift
	expect_fail "$@"
	grep -Eq -- "$pattern" "$fixture_dir/last.out" || {
		echo "error: rejection for '$*' did not mention: $pattern" >&2
		cat "$fixture_dir/last.out" >&2
		exit 1
	}
}

# fixture_keys <home>: a certify-only primary plus two signing subkeys.
# Prints "<primary> <subkey-a> <subkey-b>".
fixture_keys() {
	local home="$1" primary
	mkdir -m 700 "$home"
	gpg --homedir "$home" --batch --pinentry-mode loopback --passphrase '' \
		--quick-gen-key "$fixture_tagger" ed25519 cert 1d >/dev/null 2>&1
	primary="$(gpg --homedir "$home" --batch --with-colons --list-keys 2>/dev/null | awk -F: '$1=="fpr" {print $10; exit}')"
	gpg --homedir "$home" --batch --pinentry-mode loopback --passphrase '' \
		--quick-add-key "$primary" ed25519 sign 1d >/dev/null 2>&1
	gpg --homedir "$home" --batch --pinentry-mode loopback --passphrase '' \
		--quick-add-key "$primary" ed25519 sign 1d >/dev/null 2>&1
	printf '%s %s\n' "$primary" "$(gpg --homedir "$home" --batch --with-colons --with-subkey-fingerprint --list-keys 2>/dev/null |
		awk -F: '$1=="sub" {s=1; next} s && $1=="fpr" {printf "%s ", $10; s=0}')"
}

# fixture_minisign_pub <path> <byte>: a structurally valid synthetic public key.
fixture_minisign_pub() {
	python3 - "$1" "$2" <<'PY'
import base64
import pathlib
import sys
path, byte = sys.argv[1], int(sys.argv[2])
blob = b'Ed' + bytes([byte]) * 8 + bytes([byte]) * 32
pathlib.Path(path).write_text('untrusted comment: synthetic public key\n' + base64.b64encode(blob).decode() + '\n')
PY
}

# fixture_repo <dir> <version>: a git repository carrying the release scripts
# and VERSION, with commit and tag signing off unless a test signs explicitly.
fixture_repo() {
	local repo="$1" version="$2" name
	mkdir -p "$repo/scripts"
	git -C "$repo" init -q -b main
	git -C "$repo" config user.name fixture
	git -C "$repo" config user.email fixture@example.invalid
	git -C "$repo" config commit.gpgsign false
	git -C "$repo" config tag.gpgsign false
	for name in "${fixture_scripts[@]}"; do
		cp "$fixture_root/scripts/$name" "$repo/scripts/$name"
	done
	printf '%s\n' "$version" >"$repo/VERSION"
	fixture_commit "$repo" scripts
}

fixture_commit() {
	git -C "$1" add -A
	git -C "$1" commit -q --allow-empty -m "$2"
}

# fixture_pins <repo> <gpg-home> <subkey> <checker>: commit the current pin
# layout for the primary and exactly <subkey>, then derive (with <checker>) and
# commit the anchors.
fixture_pins() {
	local repo="$1" home="$2" subkey="$3" checker="$4"
	mkdir -p "$repo/docs/security" "$repo/config/release"
	gpg --homedir "$home" --batch --armor --export-options export-minimal --export "$subkey!" \
		>"$repo/docs/security/release-signing-keys.asc"
	fixture_minisign_pub "$repo/docs/security/goneat-minisign.pub" 1
	printf '%s\n' "$fixture_tagger" >"$repo/config/release/tagger-identity.txt"
	fixture_commit "$repo" pins
	(cd "$repo" && GONEAT_DECERNOR_BIN="$checker" ./scripts/release-insert-anchors.sh >/dev/null 2>&1)
	fixture_commit "$repo" anchors
}

#!/usr/bin/env bash

set -euo pipefail

# Signed release tags with a declared tagger identity.
#
# Usage: release-tag.sh create|verify|push
#
#   create  run all guards, create a GPG-signed annotated tag on HEAD, then
#           verify it; a tag that fails verification is deleted (only the tag
#           this invocation created) and the script exits non-zero
#   verify  verify an existing local tag (signature, signer, tagger, target)
#   push    re-check the repository, refuse if origin already has the tag,
#           verify, push only refs/tags/<tag> to origin (never forced), then
#           confirm origin's tag object and target match the local ones.
#           git push runs the pre-push hook: goneat assess --mode check.
#           That is the same assess as make prepush. It does not rewrite
#           tracked files. The dates check needs the full commit history.
#
# Env (operator machine, NOT CI):
#   GONEAT_RELEASE_TAG   - tag to create/verify/push; must equal $(cat VERSION)
#   GONEAT_PGP_KEY_ID    - signing key selector: 40-hex fingerprint or 16-hex
#                           long key id, optionally suffixed with "!" to force
#                           that exact (sub)key
#   GONEAT_GPG_HOMEDIR   - isolated gpg homedir holding the signing key; the
#                           ambient keyring is never consulted
#   GONEAT_TAGGER_NAME   - tagger name recorded on the tag
#   GONEAT_TAGGER_EMAIL  - tagger email; must be a uid email on the key
#
# Key selection rule:
#   forced ("!")  the signature must be made by exactly the selected key
#   unforced      the selector must name a primary key; the signature must be
#                 made by that primary (if it can sign) or one of its live
#                 signing subkeys
# Identity comparisons always use full fingerprints.

REMOTE=origin
BRANCH=main

die() {
	echo "error: $*" >&2
	exit 1
}

require_var() {
	local name=$1 hint=$2
	if [ -z "${!name:-}" ]; then
		die "${name} is required (${hint})"
	fi
}

# --- key resolution --------------------------------------------------------

SELECTOR=""
FORCED=false
SELECTED_FPR=""
PRIMARY_FPR=""
ALLOWED_SIGNERS=""
KEY_UIDS=""

gpg_isolated() {
	gpg --homedir "${GONEAT_GPG_HOMEDIR}" --batch --no-tty "$@"
}

normalize_selector() {
	local raw=$1
	raw=$(printf '%s' "${raw}" | tr -d '[:space:]' | tr '[:lower:]' '[:upper:]')
	FORCED=false
	if [ "${raw%!}" != "${raw}" ]; then
		FORCED=true
		raw=${raw%!}
	fi
	if ! [[ "${raw}" =~ ^[0-9A-F]{40}$ || "${raw}" =~ ^[0-9A-F]{16}$ ]]; then
		die "GONEAT_PGP_KEY_ID must be a 40-hex fingerprint or 16-hex long key id (optionally ending in '!'); short ids, emails, and names are not accepted"
	fi
	SELECTOR=${raw}
}

# Resolve SELECTOR in the isolated keyring. Sets SELECTED_FPR, PRIMARY_FPR,
# ALLOWED_SIGNERS (space-separated full fingerprints), and KEY_UIDS.
resolve_key() {
	local listing resolved
	if [ ! -d "${GONEAT_GPG_HOMEDIR}" ]; then
		die "GONEAT_GPG_HOMEDIR is not a directory"
	fi
	if ! listing=$(gpg_isolated --with-colons --fixed-list-mode --list-keys); then
		die "cannot list keys in GONEAT_GPG_HOMEDIR"
	fi

	# One line per key: <primary-fpr> <fpr> <is-primary> <validity> <caps>.
	# Plus uid lines: uid <primary-fpr> <validity> <uid-string>.
	resolved=$(printf '%s\n' "${listing}" | awk -F: '
		$1 == "pub" { primary = ""; kind = "pub"; validity = $2; caps = $12; want_fpr = 1; next }
		$1 == "sub" { kind = "sub"; validity = $2; caps = $12; want_fpr = 1; next }
		$1 == "fpr" && want_fpr {
			want_fpr = 0
			if (kind == "pub") { primary = $10 }
			printf "key %s %s %s %s %s\n", primary, $10, (kind == "pub" ? "1" : "0"), (validity == "" ? "-" : validity), (caps == "" ? "-" : caps)
			next
		}
		$1 == "uid" { printf "uid %s %s %s\n", primary, ($2 == "" ? "-" : $2), $10; next }
	')

	local matches count
	if [ "${#SELECTOR}" -eq 40 ]; then
		matches=$(printf '%s\n' "${resolved}" | awk -v s="${SELECTOR}" '$1 == "key" && $3 == s')
	else
		matches=$(printf '%s\n' "${resolved}" | awk -v s="${SELECTOR}" '$1 == "key" && substr($3, 25) == s')
	fi
	count=$(printf '%s' "${matches}" | grep -c '^key ' || true)
	if [ "${count}" -eq 0 ]; then
		die "GONEAT_PGP_KEY_ID does not match any key in GONEAT_GPG_HOMEDIR"
	fi
	if [ "${count}" -ne 1 ]; then
		die "GONEAT_PGP_KEY_ID is ambiguous in GONEAT_GPG_HOMEDIR (${count} keys match); use a full fingerprint"
	fi

	local is_primary validity caps
	read -r _ PRIMARY_FPR SELECTED_FPR is_primary validity caps <<<"${matches}"

	usable() {
		case "$1" in
		*r* | *e* | *i* | *d* | *n*) return 1 ;;
		esac
		[[ "$2" == *s* ]]
	}

	if [ "${FORCED}" = true ]; then
		if ! usable "${validity}" "${caps}"; then
			die "selected key ${SELECTED_FPR} is not a live signing-capable key"
		fi
		ALLOWED_SIGNERS=${SELECTED_FPR}
	else
		if [ "${is_primary}" != "1" ]; then
			die "GONEAT_PGP_KEY_ID names a subkey; name the primary key, or force the subkey with a trailing '!'"
		fi
		ALLOWED_SIGNERS=$(printf '%s\n' "${resolved}" | awk -v p="${PRIMARY_FPR}" '
			$1 == "key" && $2 == p {
				v = $5; c = $6
				if (v ~ /[reidn]/) next
				if (c !~ /s/) next
				printf "%s ", $3
			}
		')
		ALLOWED_SIGNERS=${ALLOWED_SIGNERS% }
		if [ -z "${ALLOWED_SIGNERS}" ]; then
			die "primary key ${PRIMARY_FPR} has no live signing-capable key"
		fi
	fi

	KEY_UIDS=$(printf '%s\n' "${resolved}" | awk -v p="${PRIMARY_FPR}" '
		$1 == "uid" && $2 == p && $3 !~ /[re]/ { sub(/^uid [^ ]+ [^ ]+ /, ""); print }
	')
}

require_secret_key() {
	local secrets
	if ! secrets=$(gpg_isolated --with-colons --fixed-list-mode --list-secret-keys); then
		die "cannot list secret keys in GONEAT_GPG_HOMEDIR"
	fi
	# The secret for the selected key must be present and not a stub ("#").
	if ! printf '%s\n' "${secrets}" | awk -F: -v want="${SELECTED_FPR}" '
		($1 == "sec" || $1 == "ssb") { token = $15; pending = 1; next }
		$1 == "fpr" && pending { pending = 0; if ($10 == want && token != "#") found = 1 }
		END { exit(found ? 0 : 1) }
	'; then
		die "secret key for ${SELECTED_FPR} is not available in GONEAT_GPG_HOMEDIR"
	fi
}

require_tagger_uid() {
	local email_lc uid_email match=false
	email_lc=$(printf '%s' "${GONEAT_TAGGER_EMAIL}" | tr '[:upper:]' '[:lower:]')
	while IFS= read -r uid; do
		[ -n "${uid}" ] || continue
		uid_email=$(printf '%s' "${uid}" | sed -n 's/.*<\([^>]*\)>.*/\1/p' | tr '[:upper:]' '[:lower:]')
		if [ -n "${uid_email}" ] && [ "${uid_email}" = "${email_lc}" ]; then
			match=true
		fi
	done <<<"${KEY_UIDS}"
	if [ "${match}" != true ]; then
		echo "error: GONEAT_TAGGER_EMAIL is not a uid email on key ${PRIMARY_FPR}; uids on the key:" >&2
		printf '%s\n' "${KEY_UIDS}" | sed 's/^/  /' >&2
		exit 1
	fi
}

# --- guards ----------------------------------------------------------------

guard_tag_version() {
	require_var GONEAT_RELEASE_TAG "set it to \$(cat VERSION)"
	local expected
	expected="$(tr -d '[:space:]' <VERSION)"
	if [ "${GONEAT_RELEASE_TAG}" != "${expected}" ]; then
		die "GONEAT_RELEASE_TAG (${GONEAT_RELEASE_TAG}) does not match VERSION (${expected})"
	fi
	if ! [[ "${expected}" =~ ^v[0-9]+\.[0-9]+\.[0-9]+(-(rc|beta|alpha)\.[0-9]+)?$ ]]; then
		die "VERSION (${expected}) is not vX.Y.Z[-(rc|beta|alpha).N]"
	fi
}

guard_repo_state() {
	local branch head remote_head
	if [ -n "$(git status --porcelain)" ]; then
		die "working tree is not clean"
	fi
	branch=$(git symbolic-ref --quiet --short HEAD || true)
	if [ "${branch}" != "${BRANCH}" ]; then
		die "release tags are created from ${BRANCH} (current: ${branch:-detached HEAD})"
	fi
	if ! git fetch --quiet --no-tags "${REMOTE}" "+refs/heads/${BRANCH}:refs/remotes/${REMOTE}/${BRANCH}"; then
		die "cannot fetch ${REMOTE}/${BRANCH}"
	fi
	head=$(git rev-parse HEAD)
	remote_head=$(git rev-parse "refs/remotes/${REMOTE}/${BRANCH}")
	if [ "${head}" != "${remote_head}" ]; then
		die "HEAD (${head}) does not match ${REMOTE}/${BRANCH} (${remote_head})"
	fi
}

guard_tag_absent() {
	if git rev-parse --quiet --verify "refs/tags/${GONEAT_RELEASE_TAG}" >/dev/null; then
		die "tag ${GONEAT_RELEASE_TAG} already exists locally; existing tags are never moved or replaced"
	fi
	guard_remote_tag_absent
}

guard_remote_tag_absent() {
	local remote_refs
	if ! remote_refs=$(git ls-remote --tags "${REMOTE}" "refs/tags/${GONEAT_RELEASE_TAG}"); then
		die "cannot query tags on ${REMOTE}"
	fi
	if [ -n "${remote_refs}" ]; then
		die "tag ${GONEAT_RELEASE_TAG} already exists on ${REMOTE}; existing tags are never moved or replaced"
	fi
}

guard_signing_vars() {
	require_var GONEAT_PGP_KEY_ID "40-hex fingerprint or 16-hex long key id of the release signing key"
	require_var GONEAT_GPG_HOMEDIR "isolated gpg homedir holding the release signing key"
	require_var GONEAT_TAGGER_NAME "tagger name recorded on the release tag"
	require_var GONEAT_TAGGER_EMAIL "tagger email; must be a uid email on the signing key"
	case "${GONEAT_TAGGER_NAME}${GONEAT_TAGGER_EMAIL}" in
	*'<'* | *'>'* | *$'\n'*) die "GONEAT_TAGGER_NAME and GONEAT_TAGGER_EMAIL must not contain '<', '>', or newlines" ;;
	esac
}

# --- verify ----------------------------------------------------------------

verify_tag() {
	local tag=$1 type target head tagger_line tagger_name tagger_email status rc
	type=$(git cat-file -t "refs/tags/${tag}" 2>/dev/null || true)
	if [ "${type}" != "tag" ]; then
		die "${tag} is not an annotated tag object (type: ${type:-missing})"
	fi
	if [ "$(git cat-file -p "refs/tags/${tag}" | sed -n '/^$/q; s/^tag //p')" != "${tag}" ]; then
		die "${tag} tag object records a different tag name"
	fi
	target=$(git rev-parse "refs/tags/${tag}^{commit}")
	head=$(git rev-parse HEAD)
	if [ "${target}" != "${head}" ]; then
		die "${tag} points at ${target}, not HEAD (${head})"
	fi

	tagger_line=$(git cat-file -p "refs/tags/${tag}" | sed -n '/^$/q; /^tagger /p')
	tagger_name=$(printf '%s' "${tagger_line}" | sed -n 's/^tagger \(.*\) <[^>]*> [0-9][0-9]* [-+][0-9]\{4\}$/\1/p')
	tagger_email=$(printf '%s' "${tagger_line}" | sed -n 's/^tagger .* <\([^>]*\)> [0-9][0-9]* [-+][0-9]\{4\}$/\1/p')
	if [ "${tagger_name}" != "${GONEAT_TAGGER_NAME}" ] || [ "${tagger_email}" != "${GONEAT_TAGGER_EMAIL}" ]; then
		die "${tagger_line:-no tagger line} does not match GONEAT_TAGGER_NAME/GONEAT_TAGGER_EMAIL"
	fi

	rc=0
	status=$(GNUPGHOME="${GONEAT_GPG_HOMEDIR}" git -c gpg.format=openpgp -c gpg.program=gpg \
		verify-tag --raw "${tag}" 2>&1) || rc=$?
	if [ "${rc}" -ne 0 ]; then
		die "git verify-tag failed for ${tag} (exit ${rc})"
	fi

	local goodsig validsig bad signer primary
	goodsig=$(printf '%s\n' "${status}" | grep -c '^\[GNUPG:\] GOODSIG ' || true)
	validsig=$(printf '%s\n' "${status}" | grep -c '^\[GNUPG:\] VALIDSIG ' || true)
	bad=$(printf '%s\n' "${status}" | grep -cE '^\[GNUPG:\] (BADSIG|ERRSIG|EXPSIG|EXPKEYSIG|REVKEYSIG) ' || true)
	if [ "${goodsig}" -lt 1 ] || [ "${validsig}" -ne 1 ] || [ "${bad}" -ne 0 ]; then
		die "${tag} signature status is not a single good signature (GOODSIG=${goodsig} VALIDSIG=${validsig} bad=${bad})"
	fi
	signer=$(printf '%s\n' "${status}" | awk '$2 == "VALIDSIG" { print $3 }')
	primary=$(printf '%s\n' "${status}" | awk '$2 == "VALIDSIG" { print $NF }')

	if [ "${FORCED}" = true ]; then
		if [ "${signer}" != "${SELECTED_FPR}" ]; then
			die "${tag} was signed by ${signer}, not the selected key ${SELECTED_FPR}"
		fi
	else
		if [ "${primary}" != "${PRIMARY_FPR}" ]; then
			die "${tag} was signed under primary ${primary}, not ${PRIMARY_FPR}"
		fi
		case " ${ALLOWED_SIGNERS} " in
		*" ${signer} "*) ;;
		*) die "${tag} was signed by ${signer}, which is not a live signing key of ${PRIMARY_FPR}" ;;
		esac
	fi

	echo "verified: ${tag} -> ${target}"
	echo "  signer:  ${signer}"
	echo "  primary: ${primary}"
	echo "  tagger:  ${tagger_name} <${tagger_email}>"
}

# --- commands --------------------------------------------------------------

cmd_create() {
	guard_tag_version
	guard_repo_state
	guard_tag_absent
	guard_signing_vars
	normalize_selector "${GONEAT_PGP_KEY_ID}"
	resolve_key
	require_secret_key
	require_tagger_uid

	local sign_key=${PRIMARY_FPR}
	if [ "${FORCED}" = true ]; then
		sign_key="${SELECTED_FPR}!"
	fi

	# --cleanup=verbatim keeps the message byte-exact, so it must end in a
	# newline; without one the signature is appended to the message line and
	# git cannot find it.
	GNUPGHOME="${GONEAT_GPG_HOMEDIR}" \
		GIT_COMMITTER_NAME="${GONEAT_TAGGER_NAME}" \
		GIT_COMMITTER_EMAIL="${GONEAT_TAGGER_EMAIL}" \
		git -c gpg.format=openpgp -c gpg.program=gpg \
		tag -s -u "${sign_key}" --cleanup=verbatim \
		"${GONEAT_RELEASE_TAG}" -m "Release ${GONEAT_RELEASE_TAG}"$'\n'

	# Verify in a fresh process: `set -e` is suspended for commands tested by
	# `if`, including subshells, so an in-process call would not fail closed.
	if ! bash "${BASH_SOURCE[0]}" verify; then
		git tag -d "${GONEAT_RELEASE_TAG}" >/dev/null
		die "deleted local tag ${GONEAT_RELEASE_TAG}: it failed verification"
	fi
	echo "created signed local tag ${GONEAT_RELEASE_TAG}; not pushed"
}

cmd_verify() {
	guard_tag_version
	guard_signing_vars
	normalize_selector "${GONEAT_PGP_KEY_ID}"
	resolve_key
	verify_tag "${GONEAT_RELEASE_TAG}"
}

cmd_push() {
	local tag=${GONEAT_RELEASE_TAG:-} local_obj head remote_refs remote_obj remote_target
	guard_tag_version
	tag=${GONEAT_RELEASE_TAG}
	guard_repo_state
	guard_signing_vars
	normalize_selector "${GONEAT_PGP_KEY_ID}"
	resolve_key
	guard_remote_tag_absent
	verify_tag "${tag}"

	git push "${REMOTE}" "refs/tags/${tag}:refs/tags/${tag}"

	local_obj=$(git rev-parse "refs/tags/${tag}")
	head=$(git rev-parse HEAD)
	if ! remote_refs=$(git ls-remote --tags "${REMOTE}" "refs/tags/${tag}" "refs/tags/${tag}^{}"); then
		die "pushed ${tag} but cannot read it back from ${REMOTE}"
	fi
	remote_obj=$(printf '%s\n' "${remote_refs}" | awk -v r="refs/tags/${tag}" '$2 == r { print $1 }')
	remote_target=$(printf '%s\n' "${remote_refs}" | awk -v r="refs/tags/${tag}^{}" '$2 == r { print $1 }')
	if [ "${remote_obj}" != "${local_obj}" ] || [ "${remote_target}" != "${head}" ]; then
		die "${REMOTE} ${tag} (${remote_obj:-missing} -> ${remote_target:-missing}) does not match local (${local_obj} -> ${head})"
	fi
	echo "pushed ${tag} only: ${REMOTE} has ${local_obj} -> ${head}"
}

case "${1:-}" in
create) cmd_create ;;
verify) cmd_verify ;;
push) cmd_push ;;
*) die "usage: release-tag.sh create|verify|push" ;;
esac

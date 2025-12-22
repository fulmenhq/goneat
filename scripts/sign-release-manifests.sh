#!/usr/bin/env bash

set -euo pipefail

# Release signing: minisign (required) + optional PGP.
# Signs checksum manifests only (SHA256SUMS, SHA512SUMS).
#
# Usage: scripts/sign-release-manifests.sh <tag> [dir]
#
# Env:
#   SIGNING_ENV_PREFIX - prefix for "<PREFIX>_" env var lookups (ex: GONEAT)
#   SIGNING_APP_NAME   - human-readable name for signing metadata (ex: goneat)
#   MINISIGN_KEY       - minisign secret key path (required)
#   MINISIGN_PUB       - minisign public key path (optional; copied into dir; derived from MINISIGN_KEY if unset)
#   PGP_KEY_ID         - gpg key/email/fingerprint for PGP signing (optional)
#   GPG_HOMEDIR        - gpg homedir (required if PGP_KEY_ID is set)
#   CI                 - if "true", signing is refused (safety guard)

TAG=${1:?'usage: scripts/sign-release-manifests.sh <tag> [dir]'}
DIR=${2:-dist/release}

if [ "${CI:-}" = "true" ]; then
	echo "error: signing is disabled in CI" >&2
	exit 1
fi

if [ ! -d "$DIR" ]; then
	echo "error: directory $DIR not found" >&2
	exit 1
fi

SIGNING_ENV_PREFIX=${SIGNING_ENV_PREFIX:-}
SIGNING_APP_NAME=${SIGNING_APP_NAME:-goneat}

get_var() {
	local name="$1"

	# Prefer prefixed variables when SIGNING_ENV_PREFIX is set.
	if [ -n "${SIGNING_ENV_PREFIX}" ]; then
		local prefixed_name="${SIGNING_ENV_PREFIX}_${name}"
		local prefixed_val="${!prefixed_name:-}"
		if [ -n "$prefixed_val" ]; then
			echo "$prefixed_val"
			return 0
		fi
	fi

	local val="${!name:-}"
	if [ -n "$val" ]; then
		echo "$val"
		return 0
	fi

	echo ""
}

MINISIGN_KEY="$(get_var MINISIGN_KEY)"
MINISIGN_PUB="$(get_var MINISIGN_PUB)"
PGP_KEY_ID="$(get_var PGP_KEY_ID)"
GPG_HOMEDIR="$(get_var GPG_HOMEDIR)"

# Back-compat with earlier naming
if [ -z "$GPG_HOMEDIR" ]; then
	GPG_HOMEDIR="$(get_var GPG_HOME)"
fi

has_minisign=false
has_pgp=false

if [ -z "${MINISIGN_KEY}" ]; then
	echo "error: MINISIGN_KEY (or ${SIGNING_ENV_PREFIX}_MINISIGN_KEY) is required" >&2
	exit 1
fi

if [ ! -f "${MINISIGN_KEY}" ]; then
	echo "error: MINISIGN_KEY not found: ${MINISIGN_KEY}" >&2
	exit 1
fi

if ! command -v minisign >/dev/null 2>&1; then
	echo "error: minisign not found in PATH" >&2
	echo "  install: brew install minisign (macOS) or see https://jedisct1.github.io/minisign/" >&2
	exit 1
fi

has_minisign=true
echo "minisign signing enabled (key: ${MINISIGN_KEY})"

if [ -n "${PGP_KEY_ID}" ]; then
	if ! command -v gpg >/dev/null 2>&1; then
		echo "error: PGP_KEY_ID set but gpg not found in PATH" >&2
		exit 1
	fi
	if [ -z "${GPG_HOMEDIR}" ]; then
		echo "error: GPG_HOMEDIR (or ${SIGNING_ENV_PREFIX}_GPG_HOMEDIR) must be set for PGP signing" >&2
		exit 1
	fi
	if ! gpg --homedir "${GPG_HOMEDIR}" --list-secret-keys "${PGP_KEY_ID}" >/dev/null 2>&1; then
		echo "error: secret key ${PGP_KEY_ID} not found in GPG_HOMEDIR=${GPG_HOMEDIR}" >&2
		exit 1
	fi
	has_pgp=true
	echo "PGP signing enabled (key: ${PGP_KEY_ID}, homedir: ${GPG_HOMEDIR})"
fi

echo ""

if [ ! -f "${DIR}/SHA256SUMS" ]; then
	echo "error: ${DIR}/SHA256SUMS not found (run make release-checksums first)" >&2
	exit 1
fi

timestamp=$(date -u +"%Y-%m-%dT%H:%M:%SZ")

if [ -z "${MINISIGN_PUB}" ]; then
	derived_pub="${MINISIGN_KEY%.key}.pub"
	if [ -f "${derived_pub}" ]; then
		MINISIGN_PUB="${derived_pub}"
	fi
fi

if [ -n "${MINISIGN_PUB}" ]; then
	if [ ! -f "${MINISIGN_PUB}" ]; then
		echo "error: MINISIGN_PUB not found: ${MINISIGN_PUB}" >&2
		exit 1
	fi
	pub_dst="${DIR}/fulmenhq-release-minisign.pub"
	if [ -e "${pub_dst}" ] && [ ! -w "${pub_dst}" ]; then
		echo "error: cannot overwrite ${pub_dst} (permission denied)" >&2
		echo "  This file is intentionally treated as an artifact output." >&2
		echo "  Fix: run 'make release-clean' to reset dist/release, then re-run release-download/checksums/sign." >&2
		exit 1
	fi
	cp "${MINISIGN_PUB}" "${pub_dst}"
	echo "Copied minisign public key to ${pub_dst}"
fi

sign_minisign() {
	local manifest="$1"
	local base="${DIR}/${manifest}"

	if [ ! -f "${base}" ]; then
		return 0
	fi

	echo "🔏 [minisign] Signing ${manifest}"
	rm -f "${base}.minisig"
	if [ -r /dev/tty ]; then
		minisign -S -s "${MINISIGN_KEY}" -t "${SIGNING_APP_NAME} ${TAG} ${timestamp}" -m "${base}" </dev/tty
	else
		minisign -S -s "${MINISIGN_KEY}" -t "${SIGNING_APP_NAME} ${TAG} ${timestamp}" -m "${base}"
	fi
}

sign_pgp() {
	local manifest="$1"
	local base="${DIR}/${manifest}"

	if [ ! -f "${base}" ]; then
		return 0
	fi

	echo "🔏 [PGP] Signing ${manifest}"
	rm -f "${base}.asc"
	gpg --batch --yes --armor --homedir "${GPG_HOMEDIR}" --local-user "${PGP_KEY_ID}" --detach-sign -o "${base}.asc" "${base}"
}

if [ "${has_minisign}" = true ]; then
	sign_minisign "SHA256SUMS"
	sign_minisign "SHA512SUMS"
fi

if [ "${has_pgp}" = true ]; then
	sign_pgp "SHA256SUMS"
	sign_pgp "SHA512SUMS"

	echo "🔑 Exporting GPG public key..."
	pub_key_dst="${DIR}/fulmenhq-release-signing-key.asc"
	rm -f "${pub_key_dst}"
	gpg --homedir "${GPG_HOMEDIR}" --armor --export "${PGP_KEY_ID}" >"${pub_key_dst}"
	echo "   ↳ wrote ${pub_key_dst}"
fi

echo ""
echo "✅ Signing complete for ${TAG}"
echo "   minisign: SHA256SUMS.minisig$([ -f "${DIR}/SHA512SUMS" ] && echo ", SHA512SUMS.minisig")"
if [ "${has_pgp}" = true ]; then
	echo "   PGP: SHA256SUMS.asc$([ -f "${DIR}/SHA512SUMS" ] && echo ", SHA512SUMS.asc")"
fi

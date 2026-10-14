# Goneat v0.6.4 — Committed Release Keys and Faster Startup

**Release Date**: 2026-10-14
**Status**: Stable

## TL;DR

- **Release keys are committed to the repository.** Starting with v0.6.4, the public signing keys and their fingerprints are in the repository. Release signatures are checked against the keys committed at the release tag.
- **One-command PGP verification.** `scripts/verify-release-pgp.sh` checks a release manifest signature against the committed signing subkey in a throwaway keyring.
- **Faster startup.** Commands no longer start package manager processes to extend `PATH`. `goneat version` drops from about 210 ms to about 25 ms on a machine with common package managers installed.
- **Apple jq.** The tools check reads `jq-1.7.1-apple` and other tool-prefixed version strings instead of reporting them as unparseable.

## What Changed

### Release signing

- `keys/expected-fingerprints.txt`, `docs/security/release-signing-keys.asc` and `docs/security/release-minisign.pub` hold the release keys.
- `make release-verify-signatures` and the release upload read the keys from the release tag and need no operator trust inputs. `GONEAT_RELEASE_PIN_REF` and `GONEAT_RELEASE_TRUST=env` are explicit overrides. Signing requires the operator keys to match the committed keys.
- `scripts/verify-release-pgp.sh` uses a throwaway keyring and requires a good, unexpired, unrevoked signature by the pinned signing subkey.
- The [release signing guide](https://github.com/fulmenhq/goneat/blob/v0.6.4/docs/security/release-signing.md) lists the current keys and the verification steps.

### Startup

- Every command extended `PATH` with package manager shim directories, and first ran each manager's version command to check that it was installed. `PATH` extension now checks the directories alone and starts no process. `goneat doctor` still reports manager versions.
- A mise or scoop shim directory that exists is added to `PATH` even when the manager itself is not on `PATH`. Shim directories are appended, so they do not shadow tools already on `PATH`.

### Tools check

- Tool version detection reads tool-prefixed versions such as `jq-1.8.1` and `jq-1.7.1-apple`. Prerelease suffixes are kept.

## Verifying This Release

v0.6.4 is the first release whose keys can be fetched from its tag:

```bash
RELEASE_VERSION="v0.6.4"
BASE="https://github.com/fulmenhq/goneat/releases/download/${RELEASE_VERSION}"
KEYS="https://raw.githubusercontent.com/fulmenhq/goneat/${RELEASE_VERSION}"

curl -LO "${BASE}/SHA256SUMS"
curl -LO "${BASE}/SHA256SUMS.minisig"
curl -LO "${BASE}/SHA256SUMS.asc"
curl -L -o release-minisign.pub "${KEYS}/docs/security/release-minisign.pub"
curl -L -o release-signing-keys.asc "${KEYS}/docs/security/release-signing-keys.asc"
curl -L -o expected-fingerprints.txt "${KEYS}/keys/expected-fingerprints.txt"
curl -L -o verify-release-pgp.sh "${KEYS}/scripts/verify-release-pgp.sh"

minisign -Vm SHA256SUMS -p release-minisign.pub
bash verify-release-pgp.sh expected-fingerprints.txt release-signing-keys.asc SHA256SUMS.asc SHA256SUMS
```

Compare `expected-fingerprints.txt` with the fingerprints in the release signing guide before trusting it.

## Go 1.27

gosec v2.29.0 still cannot read Go 1.27 package data. The workarounds in the [v0.6.3 notes](https://github.com/fulmenhq/goneat/blob/v0.6.4/docs/releases/v0.6.3.md#go-127) still apply.

## Upgrading

Building from source or with `go install` requires **Go 1.26 or later**. Release workflow pins are Go 1.26.9. CI runner images are fulmen-toolbox v0.5.9.

v0.6.4 release archives are Darwin ARM64, Linux amd64, Linux ARM64, Windows amd64, and Windows ARM64. Darwin amd64 downloads end at v0.6.1.

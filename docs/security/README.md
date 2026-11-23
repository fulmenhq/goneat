# Security Documentation

This directory contains security-related documentation for goneat.

## Structure

```
docs/security/
├── README.md                 # This file
├── release-signing-keys.asc  # Release pin: OpenPGP public export
├── goneat-minisign.pub       # Release pin: minisign public key
├── release-signing.md        # Signing and verification procedures
├── decisions/                # Security Decision Records (SDRs)
│   └── SDR-NNN-title.md      # Individual decisions
└── bulletins/                # Security bulletins (user-facing announcements)
    └── YYYY-MM-SBXXX.md      # Security Bulletin format
```

## Verifying a release

Official goneat releases use a GPG-signed annotated Git tag (`vX.Y.Z`) and
`SHA256SUMS` and `SHA512SUMS` manifests, each signed with minisign and with
GPG. These are separate signatures. The layout follows
[3leaps/crucible PDR-0008](https://github.com/3leaps/crucible/blob/main/docs/decisions/PDR-0008-release-public-identity-in-tree.md).

Establish the expected GPG primary and exact signing subkey, and the minisign
public-blob fingerprint, through an independently trusted maintainer channel
before accepting downloaded public keys. A key distributed beside a signature
is not independent authentication.

| File                                     | Contents                                                                                   |
| ---------------------------------------- | ------------------------------------------------------------------------------------------ |
| `docs/security/release-signing-keys.asc` | Public OpenPGP export: the approved primary and exactly one signing subkey, nothing else   |
| `docs/security/goneat-minisign.pub`      | The minisign public key that signs the checksum manifests                                  |
| `keys/expected-fingerprints.txt`         | Two lines: the OpenPGP primary fingerprint, then the minisign public-blob SHA-256          |
| `keys/expected-fingerprints.ndjson`      | The checker's fingerprint records for the same two keys                                    |
| `config/release/tagger-identity.txt`     | The exact git tagger identity for release tags                                             |
| `config/release/decernor-ci-sha256.txt`  | Digest and file name of the checker archive the tag workflow downloads                     |

The public pins are `release-signing-keys.asc` and `goneat-minisign.pub`.
Release staging extracts both from the verified tagged commit as inert blobs,
not from the current checkout, the environment or an external public-key
locator. They are published as the release assets
`fulmenhq-release-signing-key.asc` and `fulmenhq-release-minisign.pub`; those
asset names predate this layout and are kept for existing verifiers. Each
published key asset must match the committed file at the tag byte-for-byte;
fingerprint verification remains required.

The text and NDJSON anchors in each tagged commit are derived by decernor
0.1.8 or newer, twice and compared, never typed by hand. The minisign
fingerprint covers the decoded public blob, not its comment or the whole file.
Verify the exported public material against both anchors with that tool, then
use the approved minisign public key and the pinned OpenPGP subkey to verify
both manifests, and check every listed digest. Require the complete expected
asset set: five archives, both manifests, four signatures and both keys.

Release steps take the checker only from `GONEAT_DECERNOR_BIN`: an absolute
path to a regular, executable, non-symlink file that identifies as decernor
0.1.8 or newer. There is no fallback to `PATH`. To check the pins committed at
`HEAD`, and a published tag, from a clone:

```bash
export GONEAT_DECERNOR_BIN=/absolute/path/to/decernor
make release-verify-pin
GONEAT_RELEASE_TAG=vX.Y.Z make release-verify-pinned-tag
```

Tag verification reads committed public data from the tagged commit in an
isolated keyring and requires one good, unexpired signature by the exact
signing subkey of the anchored primary, and the committed tagger identity. The
forge's own "verified" flag is an additional check. Historical tags are not
retroactively signed.

Tags `v0.6.4` through `v0.6.9` predate this layout: they commit
`docs/security/release-minisign.pub` and text anchors only, and no tagger
identity. The same tools verify them; decernor derives the records from the
committed exports and checks the committed text anchors, and the tagger is
reported, not compared. Tags up to and including `v0.6.3` carry no committed
pin and are refused; see [release signing](release-signing.md#release-key-pin).

The tag workflow verifies the committed pins against their anchors and the tag
signature against the pinned subkey, with the digest-pinned checker, in a
read-only job that the release job depends on. The release job creates a draft
release and needs write permission for that; it does not sign, and a draft is
published only by the maintainer's verified upload. A passing tag workflow is a
consistency signal; it is not release authorization.

## Security Decision Records (SDRs)

SDRs document significant security-related decisions, including:

- Vulnerability assessments and remediation strategies
- False positive analysis and justification
- Security architecture decisions
- Dependency security evaluations

**Format:** `SDR-NNN-short-title.md`

**Template:** See `decisions/TEMPLATE.md`

## Security Bulletins

Bulletins are user-facing announcements for security-relevant releases:

- CVE fixes
- Security improvements
- Breaking changes with security implications

**Format:** `YYYY-MM-SBXXX.md` (e.g., `2026-01-SB001.md`)

## Machine-Readable Configuration

Vulnerability allowlists and policy are maintained in `.goneat/dependencies.yaml`:

```yaml
vulnerabilities:
  allow:
    - id: GHSA-xxxx-xxxx-xxxx
      status: false_positive|accepted_risk|mitigated
      reason: "Brief one-line explanation"
      sdr: docs/security/decisions/SDR-NNN-title.md # Full repo-relative path
      analysis: |
        2-3 line summary so readers can understand the rationale without
        opening the SDR. Include key facts: why it's safe, what was verified.
      verified_by: "@handle"
      verified_date: YYYY-MM-DD
      expires: YYYY-MM-DD # Optional expiry for accepted risks
```

**Best practices:**

- Use full repo-relative path in `sdr:` for easy navigation
- Include `analysis:` summary for quick understanding without opening the SDR
- Keep `reason:` as a one-liner; use `analysis:` for detail

## Process

1. **New vulnerability found:** Triage severity and exploitability
2. **Remediation or suppression:** Fix if possible, document if not
3. **SDR created:** For non-trivial decisions (false positives, accepted risks)
4. **Config updated:** Add to allowlist with SDR reference
5. **Bulletin published:** If user-facing impact (releases with fixes)

## References

- [SECURITY.md](/SECURITY.md) - Vulnerability reporting policy
- [.goneat/dependencies.yaml](/.goneat/dependencies.yaml) - Vulnerability policy config

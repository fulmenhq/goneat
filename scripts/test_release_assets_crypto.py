#!/usr/bin/env python3
"""Actual GPG/minisign checks with ephemeral, unprotected fixture keys only.

A fixture git repository carries copies of the release scripts and commits the
public pin at fixture tags, so pinned verification runs exactly as it does in a
goneat checkout: trust comes from git objects at the tag, checked by decernor,
never from the working tree or the operator environment.

Needs gpg, minisign and decernor >= 0.1.8 (GONEAT_DECERNOR_BIN, or decernor on
PATH; the release scripts themselves never search PATH for it).
"""

from contextlib import ExitStack
from datetime import datetime, timezone
import hashlib
import importlib.util
import json
import os
from pathlib import Path
import shutil
import subprocess
import tempfile

SCRIPTS = Path(__file__).resolve().parent
SPEC = importlib.util.spec_from_file_location(
    "release_assets", SCRIPTS / "release-assets.py"
)
ASSETS = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(ASSETS)
COPIED = (
    "release-assets.py",
    "release-platforms.sh",
    "sign-release-manifests.sh",
    "verify-manifest-signatures.sh",
    "release-decernor.sh",
    "release-verify-anchors.sh",
    "validate-release-anchors.sh",
)
LEGACY_PIN_MINI = "docs/security/release-minisign.pub"
TAGGER = "Synthetic Tagger <tagger@example.invalid>\n"


def quiet(argv, **kwargs):
    return subprocess.run(argv, check=True, capture_output=True, text=True, **kwargs)


def fixture_decernor():
    candidate = os.environ.get("GONEAT_DECERNOR_BIN") or shutil.which("decernor")
    if not candidate:
        raise RuntimeError(
            "release crypto tests need decernor >= 0.1.8 (set GONEAT_DECERNOR_BIN); no skip"
        )
    # Tests may locate a checker; the release scripts never do. Resolve
    # symlinks so the strict resolver accepts the path.
    return os.path.realpath(candidate)


def derive_anchors(decernor, pgp_public, mini_public):
    """Anchor text and records for the given exports, produced by decernor."""

    def records(path, kind, *extra):
        return quiet(
            [
                decernor,
                "fingerprint",
                str(path),
                "--class",
                "public",
                "--kind",
                kind,
                "--fail-on-empty",
                "--format",
                "ndjson",
                "--path-mode",
                "none",
                *extra,
            ]
        ).stdout.splitlines(keepends=True)

    gpg_line = records(pgp_public, "gpg", "--gpg-role", "primary")
    mini_line = [
        line
        for line in records(mini_public, "minisign")
        if '"fingerprint_scheme":"minisign-public-blob-sha256-v1"' in line
    ]
    assert len(gpg_line) == 1 and len(mini_line) == 1
    text = (
        "gpg " + json.loads(gpg_line[0])["fingerprint"] + "\n"
        "minisign " + json.loads(mini_line[0])["fingerprint"] + "\n"
    )
    return text, gpg_line[0] + mini_line[0]


class Fixture:
    def __init__(self, root, cleanup, decernor):
        self.root = root
        self.cleanup = cleanup
        self.decernor = decernor
        self.repo = root / "repo"
        (self.repo / "scripts").mkdir(parents=True)
        for name in COPIED:
            shutil.copy2(SCRIPTS / name, self.repo / "scripts" / name)
        self.git("init", "-q", "-b", "main")
        self.git("config", "user.name", "fixture")
        self.git("config", "user.email", "fixture@example.invalid")
        self.git("config", "commit.gpgsign", "false")
        self.git("config", "tag.gpgsign", "false")
        self.git("add", "scripts")
        self.git("commit", "-q", "-m", "fixture scripts")

    def git(self, *argv):
        return quiet(["git", "-C", str(self.repo), *argv]).stdout

    def git_bytes(self, spec):
        return subprocess.run(
            ["git", "-C", str(self.repo), "cat-file", "blob", spec],
            check=True,
            capture_output=True,
        ).stdout

    def pgp_key(self, name):
        home = self.root / ("gpg-" + name)
        home.mkdir(mode=0o700)
        self.cleanup.callback(
            subprocess.run,
            ["gpgconf", "--homedir", str(home), "--kill", "all"],
            check=False,
            capture_output=True,
        )
        base = [
            "gpg",
            "--batch",
            "--homedir",
            str(home),
            "--pinentry-mode",
            "loopback",
            "--passphrase",
            "",
        ]
        # Production shape: certify-only primary plus exactly one signing subkey.
        quiet(
            base
            + [
                "--quick-generate-key",
                f"{name} <{name}@example.invalid>",
                "ed25519",
                "cert",
                "1d",
            ]
        )
        primary = ASSETS.primary_fingerprint(
            quiet(
                [
                    "gpg",
                    "--batch",
                    "--homedir",
                    str(home),
                    "--with-colons",
                    "--fingerprint",
                    "--list-keys",
                ]
            ).stdout
        )
        quiet(base + ["--quick-add-key", primary, "ed25519", "sign", "1d"])
        public = self.root / f"{name}.asc"
        public.write_text(
            quiet(
                [
                    "gpg",
                    "--batch",
                    "--homedir",
                    str(home),
                    "--armor",
                    "--export",
                    primary,
                ]
            ).stdout
        )
        return home, primary, public

    def minisign_key(self, name):
        private = self.root / f"{name}.key"
        public = self.root / f"{name}.pub"
        quiet(["minisign", "-G", "-W", "-s", str(private), "-p", str(public)])
        return private, public

    def write_pin(self, pgp, mini, anchors=None, layout="current"):
        """Replace the pin files in the work tree with the requested layout."""
        for path in (
            ASSETS.PIN_PGP,
            ASSETS.PIN_MINI,
            LEGACY_PIN_MINI,
            ASSETS.PIN_ANCHORS,
            ASSETS.PIN_ANCHORS_NDJSON,
            ASSETS.PIN_TAGGER,
        ):
            target = self.repo / path
            if target.exists() or target.is_symlink():
                target.unlink()
        for directory in ("keys", "docs/security", "config/release"):
            (self.repo / directory).mkdir(parents=True, exist_ok=True)
        source = anchors or (pgp, mini)
        text, records = derive_anchors(self.decernor, source[0][2], source[1][1])
        shutil.copyfile(pgp[2], self.repo / ASSETS.PIN_PGP)
        (self.repo / ASSETS.PIN_ANCHORS).write_text(text)
        if layout in ("current", "both"):
            shutil.copyfile(mini[1], self.repo / ASSETS.PIN_MINI)
            (self.repo / ASSETS.PIN_ANCHORS_NDJSON).write_text(records)
            (self.repo / ASSETS.PIN_TAGGER).write_text(TAGGER)
        if layout in ("legacy", "both"):
            shutil.copyfile(mini[1], self.repo / LEGACY_PIN_MINI)

    def commit_pin(self, tag, pgp, mini, anchors=None, layout="current"):
        self.write_pin(pgp, mini, anchors, layout)
        self.git("add", "-A")
        self.git("commit", "-q", "--allow-empty", "-m", "pin " + tag)
        self.git("tag", "-f", "-a", "-m", tag, tag)
        return self.git("rev-parse", "HEAD").strip()

    def retarget(self, tag, commit):
        self.git("tag", "-f", "-a", "-m", tag, tag, commit)

    def release(self, tag):
        directory = self.root / ("release-" + tag)
        directory.mkdir()
        names = ASSETS.archive_names(tag)
        for name in names:
            (directory / name).write_bytes(b"fixture archive: " + name.encode())
        for manifest, algorithm in zip(ASSETS.MANIFESTS, ("sha256", "sha512")):
            (directory / manifest).write_text(
                "".join(
                    f"{ASSETS.digest(directory / name, algorithm)}  {name}\n"
                    for name in reversed(names)
                )
            )
        return directory


def main():
    for name in ("git", "gpg", "gpgconf", "minisign"):
        if not shutil.which(name):
            raise RuntimeError(f"required crypto fixture tool missing: {name}; no skip")
    decernor = fixture_decernor()
    # GPG agent Unix socket paths are bounded; macOS TMPDIR can already be long.
    with (
        tempfile.TemporaryDirectory(prefix="goneat-crypto-", dir="/tmp") as temporary,
        ExitStack() as cleanup,
    ):
        root = Path(temporary)
        fx = Fixture(root, cleanup, decernor)
        approved = fx.pgp_key("approved")
        approved_mini = fx.minisign_key("approved")
        other = fx.pgp_key("other")
        other_mini = fx.minisign_key("other")
        good = fx.commit_pin("v0.6.2", approved, approved_mini)

        base_env = {
            key: value
            for key, value in os.environ.items()
            if not key.startswith(("GONEAT_", "SIGNING_", "GNUPG", "RELEASE_"))
            and key
            not in ("CI", "PGP_KEY_ID", "GPG_HOMEDIR", "MINISIGN_PUB", "MINISIGN_KEY")
        }
        base_env["GONEAT_DECERNOR_BIN"] = decernor

        def signer(pgp, mini):
            return dict(
                GONEAT_GPG_HOMEDIR=str(pgp[0]),
                GONEAT_PGP_KEY_ID=pgp[1],
                GONEAT_MINISIGN_KEY=str(mini[0]),
            )

        invocations = []
        passed = []

        def invoke(
            operation, tag, directory, expected_success=True, env=None, reason=None
        ):
            wrapper = fx.repo / "scripts" / operation
            helper = fx.repo / "scripts" / "release-assets.py"
            identities = {
                str(p): hashlib.sha256(p.read_bytes()).hexdigest()
                for p in (wrapper, helper)
            }
            command = ["bash", str(wrapper), tag, str(directory)]
            started = datetime.now(timezone.utc).isoformat()
            result = subprocess.run(
                command,
                env={**base_env, **(env or {})},
                capture_output=True,
                text=True,
                timeout=60,
            )
            assert identities == {
                name: hashlib.sha256(Path(name).read_bytes()).hexdigest()
                for name in identities
            }
            invocations.append(
                {
                    "command": command,
                    "started": started,
                    "completed": datetime.now(timezone.utc).isoformat(),
                    "exit": result.returncode,
                    "expected_success": expected_success,
                    "script_sha256": identities,
                }
            )
            if (result.returncode == 0) != expected_success:
                raise RuntimeError(
                    f"{operation} {tag}: unexpected exit {result.returncode}\n{result.stdout}\n{result.stderr}"
                )
            if reason and reason not in result.stderr:
                raise RuntimeError(
                    f"{operation} {tag}: expected '{reason}', got:\n{result.stderr}"
                )
            return result

        verify = "verify-manifest-signatures.sh"
        sign = "sign-release-manifests.sh"

        # --- Pinned: signing matches the pin; verification needs no operator env.
        release = fx.release("v0.6.2")
        originals = {p.name: p.read_bytes() for p in release.iterdir()}
        invoke(sign, "v0.6.2", release, env=signer(approved, approved_mini))
        result = invoke(verify, "v0.6.2", release)
        assert "pin committed at v0.6.2" in result.stdout, result.stdout
        assert "current layout" in result.stdout, result.stdout
        for name, pin in (
            (ASSETS.PUBLIC_PGP, ASSETS.PIN_PGP),
            (ASSETS.PUBLIC_MINI, ASSETS.PIN_MINI),
        ):
            assert (release / name).read_bytes() == fx.git_bytes(f"{good}:{pin}"), name
        passed.append("pinned-verify-without-operator-env")
        passed.append("staged-public-keys-are-committed-pins")

        for name in ASSETS.SIGNATURES:
            path = release / name
            signature = path.read_bytes()
            path.write_bytes(b"corrupt signature")
            invoke(verify, "v0.6.2", release, False)
            path.write_bytes(signature)
            passed.append("tampered-" + name)
            path.unlink()
            invoke(verify, "v0.6.2", release, False)
            path.write_bytes(signature)
            passed.append("missing-" + name)
        for name in ASSETS.MANIFESTS:
            path = release / name
            original = path.read_bytes()
            path.write_bytes(original + b"unexpected\n")
            invoke(verify, "v0.6.2", release, False)
            path.write_bytes(original)
            passed.append("tampered-" + name)

        # --- The checker is bound by GONEAT_DECERNOR_BIN only.
        no_checker = {k: v for k, v in base_env.items() if k != "GONEAT_DECERNOR_BIN"}
        result = subprocess.run(
            ["bash", str(fx.repo / "scripts" / verify), "v0.6.2", str(release)],
            env={
                **no_checker,
                "DECERNOR_BIN": decernor,
                "PATH": f"{Path(decernor).parent}:{no_checker.get('PATH', '')}",
            },
            capture_output=True,
            text=True,
            timeout=60,
        )
        assert (
            result.returncode != 0
            and "GONEAT_DECERNOR_BIN is required" in result.stderr
        ), result.stderr
        invoke(
            verify,
            "v0.6.2",
            release,
            False,
            {"GONEAT_DECERNOR_BIN": "decernor"},
            reason="absolute executable regular file",
        )
        passed.append("checker-required-no-path-or-unprefixed-fallback")

        # --- Environment variables that once selected trust do nothing now.
        bad = fx.commit_pin("pin-other", other, other_mini)
        fx.retarget("v0.6.2", good)
        result = invoke(
            verify,
            "v0.6.2",
            release,
            env={
                **signer(other, other_mini),
                "GONEAT_MINISIGN_PUB": str(other_mini[1]),
                "MINISIGN_PUB": str(other_mini[1]),
                "GONEAT_RELEASE_TRUST": "env",
                "GONEAT_RELEASE_PIN_REF": bad,
            },
        )
        assert "pin committed at v0.6.2" in result.stdout, result.stdout
        fx.retarget("v0.6.2", bad)
        invoke(
            verify,
            "v0.6.2",
            release,
            False,
            {
                "GONEAT_RELEASE_PIN_REF": good,
                "GONEAT_RELEASE_TRUST": "env",
                **signer(approved, approved_mini),
                "GONEAT_MINISIGN_PUB": str(approved_mini[1]),
            },
            reason="bundled PGP key does not match the committed pin",
        )
        fx.retarget("v0.6.2", good)
        passed.append("environment-does-not-override-pin")

        # A valid signature by a key outside the pin is not authorized, and
        # bundled release keys never become a trust root.
        path = release / "SHA256SUMS.asc"
        approved_signature = path.read_bytes()
        path.unlink()
        quiet(
            [
                "gpg",
                "--batch",
                "--homedir",
                str(other[0]),
                "--armor",
                "--local-user",
                other[1],
                "--detach-sign",
                "-o",
                str(path),
                str(release / "SHA256SUMS"),
            ]
        )
        invoke(verify, "v0.6.2", release, False)
        path.write_bytes(approved_signature)
        passed.append("valid-signature-from-unpinned-key")
        bundled = release / ASSETS.PUBLIC_MINI
        bundled_bytes = bundled.read_bytes()
        shutil.copyfile(other_mini[1], bundled)
        invoke(verify, "v0.6.2", release, False, reason="bundled minisign key")
        bundled.write_bytes(
            bundled_bytes.replace(b"untrusted comment:", b"untrusted comment: edited")
        )
        invoke(verify, "v0.6.2", release, False, reason="bundled minisign key")
        bundled.write_bytes(bundled_bytes)
        passed.append("bundled-minisign-key-differs-from-pin")

        # --- Pin mismatches: the release tag is moved to a commit whose
        # committed pin disagrees; nothing is selected by environment.
        def refused_at(label, commit, reason):
            fx.retarget("v0.6.2", commit)
            invoke(verify, "v0.6.2", release, False, reason=reason)
            fx.retarget("v0.6.2", good)
            passed.append(label)

        refused_at(
            "pgp-pin-mismatch",
            fx.commit_pin("pin-pgp-mismatch", other, approved_mini),
            "bundled PGP key does not match the committed pin",
        )
        refused_at(
            "minisign-pin-mismatch",
            fx.commit_pin("pin-minisign-mismatch", approved, other_mini),
            "bundled minisign key does not match the committed pin",
        )
        refused_at(
            "pgp-key-differs-from-anchor",
            fx.commit_pin(
                "pin-anchor-pgp",
                approved,
                approved_mini,
                anchors=(other, approved_mini),
            ),
            "committed pins differ from their anchors",
        )
        refused_at(
            "minisign-key-differs-from-anchor",
            fx.commit_pin(
                "pin-anchor-minisign",
                approved,
                approved_mini,
                anchors=(approved, other_mini),
            ),
            "committed pins differ from their anchors",
        )
        fx.write_pin(approved, approved_mini)
        pin = fx.repo / ASSETS.PIN_MINI
        pin.unlink()
        pin.symlink_to(approved_mini[1])
        fx.git("add", "-A")
        fx.git("commit", "-q", "-m", "symlink pin")
        refused_at(
            "symlinked-pin-file-refused",
            fx.git("rev-parse", "HEAD").strip(),
            "committed regular file",
        )
        fx.write_pin(approved, approved_mini)
        (fx.repo / ASSETS.PIN_ANCHORS_NDJSON).unlink()
        fx.git("add", "-A")
        fx.git("commit", "-q", "-m", "no ndjson")
        refused_at(
            "current-layout-requires-ndjson",
            fx.git("rev-parse", "HEAD").strip(),
            "keys/expected-fingerprints.ndjson required",
        )
        refused_at(
            "both-minisign-pins-refused",
            fx.commit_pin("pin-both", approved, approved_mini, layout="both"),
            "carries both",
        )
        fx.write_pin(approved, approved_mini)
        for path in (ASSETS.PIN_MINI, ASSETS.PIN_ANCHORS_NDJSON):
            (fx.repo / path).unlink()
        fx.git("add", "-A")
        fx.git("commit", "-q", "-m", "no minisign pin")
        refused_at(
            "commit-without-pin-refused",
            fx.git("rev-parse", "HEAD").strip(),
            "carries no committed minisign pin",
        )
        fx.git("tag", "-d", "v0.6.2")
        invoke(verify, "v0.6.2", release, False, reason="pin ref not found")
        fx.retarget("v0.6.2", good)
        passed.append("missing-release-tag")
        # The working tree is never consulted: a matching pin left uncommitted
        # does not rescue a tag whose committed pin disagrees.
        fx.retarget("v0.6.2", bad)
        fx.write_pin(approved, approved_mini)
        invoke(
            verify,
            "v0.6.2",
            release,
            False,
            reason="bundled PGP key does not match the committed pin",
        )
        fx.git("checkout", "-q", "--", ".")
        fx.retarget("v0.6.2", good)
        passed.append("working-tree-pin-ignored")

        # --- Legacy layout (tags v0.6.4 to v0.6.9): text anchors only, verified
        # by decernor; a release on that layout still verifies.
        fx.commit_pin("v0.6.1", approved, approved_mini, layout="legacy")
        legacy = fx.release("v0.6.1")
        invoke(sign, "v0.6.1", legacy, env=signer(approved, approved_mini))
        result = invoke(verify, "v0.6.1", legacy)
        assert "legacy layout" in result.stdout, result.stdout
        refused_at(
            "legacy-anchor-mismatch-refused",
            fx.commit_pin(
                "pin-legacy-anchor",
                approved,
                approved_mini,
                anchors=(approved, other_mini),
                layout="legacy",
            ),
            "committed pins differ from their anchors",
        )
        passed.append("legacy-layout-verifies")

        # --- Signing refuses operator keys that differ from the pin.
        unsigned = fx.release("v0.6.9")
        fx.git("tag", "-a", "-m", "v0.6.9", "v0.6.9", good)
        before = {p.name: p.read_bytes() for p in unsigned.iterdir()}
        invoke(
            sign,
            "v0.6.9",
            unsigned,
            False,
            signer(other, approved_mini),
            reason="operator PGP identity differs from the committed pin",
        )
        invoke(
            sign,
            "v0.6.9",
            unsigned,
            False,
            signer(approved, other_mini),
            reason="minisign failed",
        )
        invoke(
            sign,
            "v0.6.9",
            unsigned,
            False,
            {**signer(approved, approved_mini), "GONEAT_MINISIGN_KEY": ""},
            reason="required to sign",
        )
        assert before == {p.name: p.read_bytes() for p in unsigned.iterdir()}
        passed.append("sign-refuses-operator-keys-outside-pin")

        # --- Rotation: v0.7.0 commits a new pin; each release verifies against
        # the pin in force at its own tag, whatever HEAD carries.
        fx.commit_pin("v0.7.0", other, other_mini)
        rotated = fx.release("v0.7.0")
        invoke(sign, "v0.7.0", rotated, env=signer(other, other_mini))
        invoke(verify, "v0.7.0", rotated)
        invoke(verify, "v0.6.2", release)
        passed.append("rotation-each-release-verifies-against-its-tag")

        # --- Documented consumer PGP check (scripts/verify-release-pgp.sh).
        consumer = SCRIPTS / "verify-release-pgp.sh"
        anchors = root / "consumer-anchors.txt"
        anchors.write_text(f"gpg {approved[1]}\nminisign {'0' * 64}\n")
        other_anchors = root / "consumer-other-anchors.txt"
        other_anchors.write_text(f"gpg {other[1]}\nminisign {'0' * 64}\n")

        def consume(
            keys, anchor_file, manifest, expected_success, reason=None, env=None
        ):
            result = subprocess.run(
                [
                    "bash",
                    str(consumer),
                    str(anchor_file),
                    str(keys),
                    str(release / (manifest + ".asc")),
                    str(release / manifest),
                ],
                env={**base_env, **(env or {})},
                capture_output=True,
                text=True,
                timeout=60,
            )
            if (result.returncode == 0) != expected_success:
                raise RuntimeError(
                    f"consumer {manifest}: exit {result.returncode}\n{result.stderr}"
                )
            if reason and reason not in result.stderr:
                raise RuntimeError(
                    f"consumer {manifest}: expected '{reason}', got:\n{result.stderr}"
                )

        consume(approved[2], anchors, "SHA256SUMS", True)
        consume(approved[2], anchors, "SHA512SUMS", True)
        passed.append("consumer-pgp-good-signature")
        consume(other[2], anchors, "SHA256SUMS", False, "primary differs from anchor")
        consume(other[2], other_anchors, "SHA256SUMS", False, "gpg verification failed")
        passed.append("consumer-pgp-unpinned-key")
        original = (release / "SHA256SUMS").read_bytes()
        (release / "SHA256SUMS").write_bytes(original + b"tampered\n")
        consume(approved[2], anchors, "SHA256SUMS", False, "gpg verification failed")
        (release / "SHA256SUMS").write_bytes(original)
        passed.append("consumer-pgp-tampered-manifest")
        # Fixture subkeys expire after one day; at +2 days gpg still prints
        # VALIDSIG and may exit 0, and the consumer check must refuse.
        future = datetime.now(timezone.utc).timestamp() + 2 * 86400
        faked = root / "gpg-faked"
        faked.mkdir()
        (faked / "gpg").write_text(
            "#!/bin/sh\n"
            f'exec {shutil.which("gpg")} --faked-system-time {int(future)} "$@"\n'
        )
        (faked / "gpg").chmod(0o755)
        consume(
            approved[2],
            anchors,
            "SHA256SUMS",
            False,
            env={"PATH": f"{faked}:{base_env.get('PATH', '')}"},
        )
        passed.append("consumer-pgp-expired-signing-key")

        invoke(sign, "v0.6.2", release, False, signer(approved, approved_mini))
        passed.append("existing-signatures-not-overwritten")
        assert all(
            (release / name).read_bytes() == data for name, data in originals.items()
        )
        hashes = {
            name: hashlib.sha256((release / name).read_bytes()).hexdigest()
            for name in originals
        }
        # ExitStack stops every fixture agent even when a negative check fails.
    assert not root.exists()
    print(
        json.dumps(
            {
                "passed": passed,
                "invocations": invocations,
                "count": len(passed),
                "originals_preserved": hashes,
                "fixture_cleanup_verified": True,
                "scope": "Ephemeral fixture keys and fixture git repository; no production key, network or release mutation",
            },
            indent=2,
        )
    )


if __name__ == "__main__":
    main()

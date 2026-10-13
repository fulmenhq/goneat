#!/usr/bin/env python3
"""Actual GPG/minisign checks with ephemeral, unprotected fixture keys only.

A fixture git repository carries copies of the release scripts and commits the
public pin at fixture tags, so pinned verification runs exactly as it does in a
goneat checkout: trust comes from git objects at the tag, never from the
working tree or the operator environment.
"""

from contextlib import ExitStack
from datetime import datetime, timezone
import base64
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
)


def quiet(argv, **kwargs):
    return subprocess.run(argv, check=True, capture_output=True, text=True, **kwargs)


class Fixture:
    def __init__(self, root, cleanup):
        self.root = root
        self.cleanup = cleanup
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

    def commit_pin(self, tag, pgp, mini, anchor_primary=None, anchor_mini=None):
        (self.repo / "keys").mkdir(exist_ok=True)
        (self.repo / "docs" / "security").mkdir(parents=True, exist_ok=True)
        for source, path in ((pgp[2], ASSETS.PIN_PGP), (mini[1], ASSETS.PIN_MINI)):
            target = self.repo / path
            if target.is_symlink():
                target.unlink()
            shutil.copyfile(source, target)
        blob = base64.b64decode(mini[1].read_text().splitlines()[1])
        (self.repo / ASSETS.PIN_ANCHORS).write_text(
            f"gpg {anchor_primary or pgp[1]}\n"
            f"minisign {anchor_mini or hashlib.sha256(blob).hexdigest()}\n"
        )
        self.git("add", "-A")
        self.git("commit", "-q", "--allow-empty", "-m", "pin " + tag)
        self.git("tag", "-a", "-m", tag, tag)

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
    # GPG agent Unix socket paths are bounded; macOS TMPDIR can already be long.
    with (
        tempfile.TemporaryDirectory(prefix="goneat-crypto-", dir="/tmp") as temporary,
        ExitStack() as cleanup,
    ):
        root = Path(temporary)
        fx = Fixture(root, cleanup)
        approved = fx.pgp_key("approved")
        approved_mini = fx.minisign_key("approved")
        other = fx.pgp_key("other")
        other_mini = fx.minisign_key("other")
        fx.commit_pin("v0.6.2", approved, approved_mini)

        base_env = {
            key: value
            for key, value in os.environ.items()
            if not key.startswith(("GONEAT_", "SIGNING_", "GNUPG"))
            and key
            not in ("CI", "PGP_KEY_ID", "GPG_HOMEDIR", "MINISIGN_PUB", "MINISIGN_KEY")
        }

        def signer(pgp, mini):
            return dict(
                GONEAT_GPG_HOMEDIR=str(pgp[0]),
                GONEAT_PGP_KEY_ID=pgp[1],
                GONEAT_MINISIGN_KEY=str(mini[0]),
                GONEAT_MINISIGN_PUB=str(mini[1]),
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

        # --- Pinned default: signing matches the pin; verification needs no operator env.
        release = fx.release("v0.6.2")
        originals = {p.name: p.read_bytes() for p in release.iterdir()}
        invoke(sign, "v0.6.2", release, env=signer(approved, approved_mini))
        result = invoke(verify, "v0.6.2", release)
        assert "committed release-key pin" in result.stdout, result.stdout
        passed.append("pinned-verify-without-operator-env")

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

        # Exported operator trust variables do not displace the pin.
        result = invoke(verify, "v0.6.2", release, env=signer(other, other_mini))
        assert "committed release-key pin" in result.stdout, result.stdout
        passed.append("operator-env-does-not-override-pin")

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
        invoke(verify, "v0.6.2", release, False)
        bundled.write_bytes(bundled_bytes)
        passed.append("bundled-minisign-key-differs-from-pin")

        # --- Pin mismatches, each committed at its own tag and selected by ref.
        fx.commit_pin("pin-pgp-mismatch", other, approved_mini)
        invoke(
            verify,
            "v0.6.2",
            release,
            False,
            {"GONEAT_RELEASE_PIN_REF": "refs/tags/pin-pgp-mismatch"},
            reason="bundled PGP key does not match approved identity",
        )
        passed.append("pgp-pin-mismatch")
        fx.commit_pin("pin-minisign-mismatch", approved, other_mini)
        invoke(
            verify,
            "v0.6.2",
            release,
            False,
            {"GONEAT_RELEASE_PIN_REF": "refs/tags/pin-minisign-mismatch"},
            reason="bundled minisign key does not match",
        )
        passed.append("minisign-pin-mismatch")
        fx.commit_pin(
            "pin-anchor-pgp", approved, approved_mini, anchor_primary=other[1]
        )
        invoke(
            verify,
            "v0.6.2",
            release,
            False,
            {"GONEAT_RELEASE_PIN_REF": "refs/tags/pin-anchor-pgp"},
            reason="pinned PGP key differs from its anchor",
        )
        passed.append("pgp-key-differs-from-anchor")
        fx.commit_pin(
            "pin-anchor-minisign", approved, approved_mini, anchor_mini="0" * 64
        )
        invoke(
            verify,
            "v0.6.2",
            release,
            False,
            {"GONEAT_RELEASE_PIN_REF": "refs/tags/pin-anchor-minisign"},
            reason="pinned minisign key differs from its anchor",
        )
        passed.append("minisign-key-differs-from-anchor")
        pin = fx.repo / ASSETS.PIN_MINI
        pin.unlink()
        pin.symlink_to(approved_mini[1])
        fx.git("add", "-A")
        fx.git("commit", "-q", "-m", "symlink pin")
        fx.git("tag", "-a", "-m", "pin-symlink", "pin-symlink")
        invoke(
            verify,
            "v0.6.2",
            release,
            False,
            {"GONEAT_RELEASE_PIN_REF": "refs/tags/pin-symlink"},
            reason="committed regular file required",
        )
        passed.append("symlinked-pin-file-refused")
        invoke(
            verify,
            "v0.6.2",
            release,
            False,
            {"GONEAT_RELEASE_PIN_REF": "refs/tags/absent"},
            reason="pin ref not found",
        )
        passed.append("missing-pin-ref")
        # The working tree is never consulted: a matching pin left uncommitted
        # does not rescue a tag whose committed pin disagrees.
        fx.commit_pin("pin-worktree", other, other_mini)
        for source, path in (
            (approved[2], ASSETS.PIN_PGP),
            (approved_mini[1], ASSETS.PIN_MINI),
        ):
            shutil.copyfile(source, fx.repo / path)
        invoke(
            verify,
            "v0.6.2",
            release,
            False,
            {"GONEAT_RELEASE_PIN_REF": "refs/tags/pin-worktree"},
            reason="bundled PGP key does not match approved identity",
        )
        passed.append("working-tree-pin-ignored")

        # --- Signing refuses operator keys that differ from the pin.
        unsigned = fx.release("v0.6.9")
        fx.git("checkout", "-q", "--", ".")
        fx.git("tag", "-a", "-m", "v0.6.9", "v0.6.9", "refs/tags/v0.6.2")
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
            reason="operator minisign key differs from the committed pin",
        )
        assert before == {p.name: p.read_bytes() for p in unsigned.iterdir()}
        passed.append("sign-refuses-operator-keys-outside-pin")

        # --- Rotation: v0.7.0 commits a new pin; each release verifies against
        # the pin in force at its own tag, and not against the other.
        fx.commit_pin("v0.7.0", other, other_mini)
        rotated = fx.release("v0.7.0")
        invoke(sign, "v0.7.0", rotated, env=signer(other, other_mini))
        invoke(verify, "v0.7.0", rotated)
        invoke(verify, "v0.6.2", release)
        invoke(
            verify,
            "v0.6.2",
            release,
            False,
            {"GONEAT_RELEASE_PIN_REF": "refs/tags/v0.7.0"},
            reason="bundled PGP key does not match approved identity",
        )
        invoke(
            verify,
            "v0.7.0",
            rotated,
            False,
            {"GONEAT_RELEASE_PIN_REF": "refs/tags/v0.6.2"},
            reason="bundled PGP key does not match approved identity",
        )
        passed.append("rotation-each-release-verifies-against-its-tag")

        # --- Explicit operator-environment mode keeps the independent-trust rules.
        env_mode = {**signer(approved, approved_mini), "GONEAT_RELEASE_TRUST": "env"}
        result = invoke(verify, "v0.6.2", release, env=env_mode)
        assert "operator environment" in result.stdout, result.stdout
        invoke(
            verify,
            "v0.6.2",
            release,
            False,
            {**env_mode, "GONEAT_PGP_KEY_ID": "C" * 40},
        )
        invoke(
            verify,
            "v0.6.2",
            release,
            False,
            {**env_mode, "GONEAT_MINISIGN_PUB": str(other_mini[1])},
        )
        invoke(
            verify,
            "v0.6.2",
            release,
            False,
            {**env_mode, "GONEAT_MINISIGN_PUB": str(release / ASSETS.PUBLIC_MINI)},
            reason="outside release directory",
        )
        invoke(
            verify,
            "v0.6.2",
            release,
            False,
            {"GONEAT_RELEASE_TRUST": "env"},
            reason="mandatory approved trust inputs",
        )
        invoke(
            verify,
            "v0.6.2",
            release,
            False,
            {"GONEAT_RELEASE_TRUST": "bundled"},
            reason="pin or env",
        )
        passed.append("env-mode-independent-trust-rules")

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

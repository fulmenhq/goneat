#!/usr/bin/env python3
"""Actual GPG/minisign checks with ephemeral, unprotected fixture keys only."""

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

SPEC = importlib.util.spec_from_file_location(
    "release_assets", Path(__file__).with_name("release-assets.py")
)
ASSETS = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(ASSETS)


def main():
    for name in ("gpg", "gpgconf", "minisign"):
        if not shutil.which(name):
            raise RuntimeError(f"required crypto fixture tool missing: {name}; no skip")
    # GPG agent Unix socket paths are bounded; macOS TMPDIR can already be long.
    with (
        tempfile.TemporaryDirectory(prefix="goneat-crypto-", dir="/tmp") as temporary,
        ExitStack() as cleanup,
    ):
        root = Path(temporary)
        home = root / "gpg"
        home.mkdir(mode=0o700)
        cleanup.callback(
            subprocess.run,
            ["gpgconf", "--homedir", str(home), "--kill", "gpg-agent"],
            check=False,
            capture_output=True,
        )
        generated = subprocess.run(
            [
                "gpg",
                "--batch",
                "--homedir",
                str(home),
                "--pinentry-mode",
                "loopback",
                "--passphrase",
                "",
                "--quick-generate-key",
                "goneat fixture <fixture@example.invalid>",
                "ed25519",
                "sign",
                "1d",
            ],
            capture_output=True,
            text=True,
        )
        if generated.returncode:
            raise RuntimeError("fixture GPG key generation failed: " + generated.stderr)
        records = subprocess.check_output(
            [
                "gpg",
                "--batch",
                "--homedir",
                str(home),
                "--with-colons",
                "--fingerprint",
                "--list-keys",
            ],
            text=True,
        )
        fingerprint = ASSETS.primary_fingerprint(records)
        private = root / "fixture.key"
        public = root / "fixture.pub"
        subprocess.run(
            ["minisign", "-G", "-W", "-s", str(private), "-p", str(public)],
            check=True,
            capture_output=True,
        )
        directory = root / "release"
        directory.mkdir()
        tag = "v0.6.2"
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
        originals = {
            name: (directory / name).read_bytes()
            for name in names + list(ASSETS.MANIFESTS)
        }
        env = {
            key: value
            for key, value in os.environ.items()
            if not key.startswith(("GONEAT_", "SIGNING_"))
            and key
            not in ("CI", "PGP_KEY_ID", "GPG_HOMEDIR", "MINISIGN_PUB", "MINISIGN_KEY")
        }
        env.update(
            GONEAT_GPG_HOMEDIR=str(home),
            GONEAT_PGP_KEY_ID=fingerprint,
            GONEAT_MINISIGN_KEY=str(private),
            GONEAT_MINISIGN_PUB=str(public),
        )
        invocations = []

        def invoke(operation, expected_success=True, overrides=None):
            wrapper = Path(__file__).with_name(operation).resolve()
            helper = Path(__file__).with_name("release-assets.py").resolve()
            identities = {
                str(p): hashlib.sha256(p.read_bytes()).hexdigest()
                for p in (wrapper, helper)
            }
            command = ["bash", str(wrapper), tag, str(directory)]
            started = datetime.now(timezone.utc).isoformat()
            result = subprocess.run(
                command,
                env={**env, **(overrides or {})},
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
                    f"{operation}: unexpected exit {result.returncode}\n{result.stdout}\n{result.stderr}"
                )
            return result

        invoke("sign-release-manifests.sh")
        invoke("verify-manifest-signatures.sh")
        passed = ["sign-and-verify-four-real-signatures"]
        for name in ASSETS.SIGNATURES:
            path = directory / name
            signature = path.read_bytes()
            path.write_bytes(b"corrupt signature")
            invoke("verify-manifest-signatures.sh", False)
            path.write_bytes(signature)
            passed.append("tampered-" + name)
            path.unlink()
            invoke("verify-manifest-signatures.sh", False)
            path.write_bytes(signature)
            passed.append("missing-" + name)
        for name in ASSETS.MANIFESTS:
            path = directory / name
            original = path.read_bytes()
            path.write_bytes(original + b"unexpected\n")
            invoke("verify-manifest-signatures.sh", False)
            path.write_bytes(original)
            passed.append("tampered-" + name)
        invoke("verify-manifest-signatures.sh", False, {"GONEAT_PGP_KEY_ID": "C" * 40})
        passed.append("wrong-approved-pgp-identity")
        # A cryptographically valid signature by another key in the same keyring
        # is not authorized merely because gpg can verify it.
        subprocess.run(
            [
                "gpg",
                "--batch",
                "--homedir",
                str(home),
                "--pinentry-mode",
                "loopback",
                "--passphrase",
                "",
                "--quick-generate-key",
                "other fixture <other@example.invalid>",
                "ed25519",
                "sign",
                "1d",
            ],
            check=True,
            capture_output=True,
        )
        path = directory / "SHA256SUMS.asc"
        approved_signature = path.read_bytes()
        path.unlink()
        subprocess.run(
            [
                "gpg",
                "--batch",
                "--homedir",
                str(home),
                "--armor",
                "--local-user",
                "other@example.invalid",
                "--detach-sign",
                "-o",
                str(path),
                str(directory / "SHA256SUMS"),
            ],
            check=True,
            capture_output=True,
        )
        invoke("verify-manifest-signatures.sh", False)
        path.write_bytes(approved_signature)
        passed.append("valid-signature-from-unapproved-key-in-same-keyring")
        wrong_private = root / "wrong.key"
        wrong_public = root / "wrong.pub"
        subprocess.run(
            ["minisign", "-G", "-W", "-s", str(wrong_private), "-p", str(wrong_public)],
            check=True,
            capture_output=True,
        )
        invoke(
            "verify-manifest-signatures.sh",
            False,
            {"GONEAT_MINISIGN_PUB": str(wrong_public)},
        )
        passed.append("wrong-approved-minisign-key")
        invoke(
            "verify-manifest-signatures.sh",
            False,
            {"GONEAT_MINISIGN_PUB": str(directory / ASSETS.PUBLIC_MINI)},
        )
        passed.append("downloaded-key-not-trust-root")
        invoke("sign-release-manifests.sh", False)
        passed.append("existing-signatures-not-overwritten")
        assert originals == {
            name: (directory / name).read_bytes() for name in originals
        }
        hashes = {
            name: hashlib.sha256((directory / name).read_bytes()).hexdigest()
            for name in originals
        }
        # ExitStack stops this fixture's agent even when a negative check fails.
    assert not root.exists()
    print(
        json.dumps(
            {
                "passed": passed,
                "invocations": invocations,
                "count": len(passed),
                "originals_preserved": hashes,
                "fixture_cleanup_verified": True,
                "scope": "Ephemeral fixture keys, no production key, network or release mutation",
            },
            indent=2,
        )
    )


if __name__ == "__main__":
    main()

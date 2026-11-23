#!/usr/bin/env python3
"""Offline release fixtures: no production keys, network calls or publication."""

import base64
from contextlib import contextmanager
import importlib.util
import json
import os
from pathlib import Path
import shutil
import subprocess
import tempfile
import unittest
from unittest.mock import patch

SPEC = importlib.util.spec_from_file_location(
    "release_assets", Path(__file__).with_name("release-assets.py")
)
ASSETS = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(ASSETS)
TAG = "v0.6.2"
PRIMARY = "A" * 40
SIGNER = "B" * 40
PUBLIC = "-----BEGIN PGP PUBLIC KEY BLOCK-----\nfixture\n-----END PGP PUBLIC KEY BLOCK-----\n"
RECORDS = (
    "pub:::::::::\nfpr:::::::::"
    + PRIMARY
    + ":\nsub:::::::::\nfpr:::::::::"
    + SIGNER
    + ":\n"
)


class ReleaseAssetsTests(unittest.TestCase):
    def setUp(self):
        self.temporary = tempfile.TemporaryDirectory(prefix="goneat-release-fixture-")
        self.addCleanup(self.temporary.cleanup)
        self.root = Path(self.temporary.name).resolve()
        self.release = self.root / "release"
        self.release.mkdir()
        self.names = ASSETS.archive_names(TAG)
        for name in self.names:
            (self.release / name).write_bytes(name.encode())
        for manifest, algorithm in zip(ASSETS.MANIFESTS, ("sha256", "sha512")):
            # Deliberately reversed order: verification must not rewrite CI bytes.
            (self.release / manifest).write_text(
                "".join(
                    f"{ASSETS.digest(self.release / name, algorithm)}  {name}\n"
                    for name in reversed(self.names)
                )
            )
        self.originals = self.names + list(ASSETS.MANIFESTS)
        # The committed pin, as release-verify-anchors.sh leaves it; the
        # decernor-backed pin reader itself is exercised with real keys in
        # test_release_assets_crypto.py.
        self.pins = self.root / "pins"
        self.pins.mkdir()
        self.pin_pgp = self.pins / "pin.asc"
        self.pin_pgp.write_text(PUBLIC)
        self.approved = self.pins / "minisign.pub"
        self.approved.write_text(
            "untrusted comment: fixture\n"
            + base64.b64encode(b"Ed" + b"k" * 40).decode()
            + "\n"
        )
        self.pin_home = self.root / "pin-gpg"
        self.pin_home.mkdir()
        self.operator_home = self.root / "operator-gpg"
        self.operator_home.mkdir()
        self.private = self.root / "fixture.key"
        self.private.write_text("fixture only")
        self.calls = []
        self.pin_refs = []
        self.crypto_failure = None
        self.signature_primary = PRIMARY
        self.signature_subkey = SIGNER
        self.operator_records = RECORDS
        environment = {
            "GONEAT_GPG_HOMEDIR": str(self.operator_home),
            "GONEAT_PGP_KEY_ID": PRIMARY,
            "GONEAT_MINISIGN_KEY": str(self.private),
        }
        self.addCleanup(patch.stopall)
        patch.dict(os.environ, environment, clear=True).start()
        patch.object(ASSETS, "tool").start()
        patch.object(ASSETS, "pinned_trust", side_effect=self.pinned).start()
        self.real_run = ASSETS.run
        patch.object(ASSETS, "run", side_effect=self.crypto).start()

    @contextmanager
    def pinned(self, ref):
        self.pin_refs.append(ref)
        yield ASSETS.Trust(
            home=self.pin_home,
            primary=PRIMARY,
            subkey=SIGNER,
            pgp=self.pin_pgp,
            minisign=self.approved,
            layout="current",
            commit="c" * 40,
        )

    def crypto(self, argv, **kwargs):
        if argv[0] == "bash":
            return self.real_run(argv, **kwargs)
        self.calls.append(argv)
        if self.crypto_failure and self.crypto_failure in " ".join(argv):
            raise RuntimeError("fixture crypto failure")
        output = ""
        if "--list-keys" in argv or "--list-secret-keys" in argv:
            output = self.operator_records
        elif "--verify" in argv:
            output = f"[GNUPG:] VALIDSIG {self.signature_subkey} 2026-10-07 1 0 4 0 22 8 00 {self.signature_primary}\n"
        elif "--detach-sign" in argv:
            Path(argv[argv.index("-o") + 1]).write_text("fixture pgp signature")
        elif "-S" in argv:
            Path(argv[argv.index("-x") + 1]).write_text("fixture minisign signature")
        return subprocess.CompletedProcess(argv, 0, stdout=output, stderr="")

    def signed(self):
        for name in ASSETS.SIGNATURES:
            (self.release / name).write_text("fixture signature")
        shutil.copyfile(self.pin_pgp, self.release / ASSETS.PUBLIC_PGP)
        shutil.copyfile(self.approved, self.release / ASSETS.PUBLIC_MINI)
        for name in ("release-notes.md", f"release-notes-{TAG}.md"):
            (self.release / name).write_text("Fixture release notes\n")

    def snapshot(self):
        return {p.name: p.read_bytes() for p in self.release.iterdir() if p.is_file()}

    def test_both_original_manifests_and_order_preserved(self):
        before = self.snapshot()
        self.assertEqual(
            set(ASSETS.verify_archives(self.release, TAG)), set(self.originals)
        )
        self.assertEqual(before, self.snapshot())

    def test_manifest_missing_duplicate_traversal_mismatch_and_extra_archive(self):
        for mutation in ("missing", "duplicate", "traversal", "mismatch", "extra"):
            with self.subTest(mutation=mutation):
                path = self.release / "SHA512SUMS"
                original = path.read_bytes()
                if mutation == "missing":
                    path.unlink()
                elif mutation == "duplicate":
                    path.write_bytes(original.splitlines(keepends=True)[0] * 5)
                elif mutation == "traversal":
                    path.write_text("0" * 128 + "  ../escape\n")
                elif mutation == "mismatch":
                    path.write_bytes(original.replace(original[:128], b"0" * 128, 1))
                else:
                    (self.release / "unexpected.zip").write_bytes(b"extra")
                with self.assertRaises((RuntimeError, OSError)):
                    ASSETS.verify_archives(self.release, TAG)
                path.write_bytes(original)
                (self.release / "unexpected.zip").unlink(missing_ok=True)

    def test_symlink_archive_refused(self):
        path = self.release / self.names[0]
        target = self.root / "archive"
        path.rename(target)
        path.symlink_to(target)
        with self.assertRaisesRegex(RuntimeError, "non-symlink"):
            ASSETS.verify_archives(self.release, TAG)

    def test_four_signature_checks_use_the_pin_at_the_tag(self):
        self.signed()
        before = self.snapshot()
        ASSETS.verify_release_signatures(self.release, TAG)
        self.assertEqual(self.pin_refs, [f"refs/tags/{TAG}"])
        pgp = [a for a in self.calls if "--verify" in a]
        mini = [a for a in self.calls if "-Vm" in a]
        self.assertEqual(len(pgp), 2)
        self.assertEqual(len(mini), 2)
        self.assertTrue(all(str(self.pin_home) in a for a in pgp))
        self.assertTrue(all(str(self.approved) in a for a in mini))
        self.assertFalse(
            any(str(self.operator_home) in " ".join(a) for a in self.calls)
        )
        self.assertEqual(before, self.snapshot())

    def test_every_missing_signature_key_or_manifest_fails(self):
        self.signed()
        for name in (
            *ASSETS.MANIFESTS,
            *ASSETS.SIGNATURES,
            ASSETS.PUBLIC_PGP,
            ASSETS.PUBLIC_MINI,
        ):
            with self.subTest(name=name):
                p = self.release / name
                original = p.read_bytes()
                p.unlink()
                with self.assertRaises(RuntimeError):
                    ASSETS.verify_release_signatures(self.release, TAG)
                p.write_bytes(original)

    def test_every_signature_failure_is_fatal(self):
        self.signed()
        for name in ASSETS.SIGNATURES:
            with self.subTest(name=name):
                self.crypto_failure = name
                with self.assertRaises(RuntimeError):
                    ASSETS.verify_release_signatures(self.release, TAG)

    def test_environment_cannot_replace_the_pin(self):
        # The former trust-mode, pin-ref and minisign public-key variables,
        # prefixed or not, have no effect: verification and staging use only
        # the pin committed at refs/tags/<tag>.
        self.signed()
        other = self.root / "other.pub"
        other.write_text(
            "untrusted comment: other\n"
            + base64.b64encode(b"Ed" + b"z" * 40).decode()
            + "\n"
        )
        overrides = {
            "GONEAT_RELEASE_TRUST": "env",
            "RELEASE_TRUST": "env",
            "GONEAT_RELEASE_PIN_REF": "refs/heads/elsewhere",
            "RELEASE_PIN_REF": "refs/heads/elsewhere",
            "GONEAT_MINISIGN_PUB": str(other),
            "MINISIGN_PUB": str(other),
            "SIGNING_ENV_PREFIX": "OTHER",
        }
        with patch.dict(os.environ, overrides):
            ASSETS.verify_release_signatures(self.release, TAG)
            mini = [a for a in self.calls if "-Vm" in a]
            self.assertTrue(mini and all(str(self.approved) in a for a in mini))
            self.assertFalse(any(str(other) in " ".join(a) for a in self.calls))
            fresh = self.root / "fresh"
            fresh.mkdir()
            for name in self.originals:
                shutil.copyfile(self.release / name, fresh / name)
            ASSETS.sign(fresh, TAG)
            self.assertEqual(
                (fresh / ASSETS.PUBLIC_MINI).read_bytes(), self.approved.read_bytes()
            )
        self.assertEqual(self.pin_refs, [f"refs/tags/{TAG}"] * 2)
        self.assertFalse(hasattr(ASSETS, "mini_key"))
        self.assertFalse(hasattr(ASSETS, "pin_mode"))

    def test_pin_commit_resolves_only_the_given_ref(self):
        with patch.dict(os.environ, {"GONEAT_RELEASE_PIN_REF": "refs/heads/elsewhere"}):
            with patch.object(ASSETS.subprocess, "run") as run:
                run.return_value = subprocess.CompletedProcess(
                    [], 0, stdout="c" * 40 + "\n"
                )
                ASSETS.pin_commit(f"refs/tags/{TAG}")
                self.assertIn(f"refs/tags/{TAG}^{{commit}}", run.call_args[0][0])

    def test_missing_minisign_does_not_skip_verification(self):
        self.signed()
        with patch.object(
            ASSETS,
            "tool",
            side_effect=lambda tool: ASSETS.require(tool != "minisign", "missing tool"),
        ):
            with self.assertRaisesRegex(RuntimeError, "missing tool"):
                ASSETS.verify_release_signatures(self.release, TAG)

    def test_sign_requires_each_prefixed_operator_input(self):
        for variable in (
            "GONEAT_GPG_HOMEDIR",
            "GONEAT_PGP_KEY_ID",
            "GONEAT_MINISIGN_KEY",
        ):
            unprefixed = variable.removeprefix("GONEAT_")
            with (
                self.subTest(variable=variable),
                patch.dict(
                    os.environ, {variable: "", unprefixed: os.environ[variable]}
                ),
            ):
                with self.assertRaisesRegex(RuntimeError, "required to sign"):
                    ASSETS.sign(self.release, TAG)
        self.assertEqual(self.calls, [])

    def test_operator_signing_inputs_must_be_outside_the_release(self):
        (self.release / "inside.key").write_text("fixture")
        (self.release / "gpg").mkdir()
        for variable, value in (
            ("GONEAT_MINISIGN_KEY", str(self.release / "inside.key")),
            ("GONEAT_GPG_HOMEDIR", str(self.release / "gpg")),
        ):
            with (
                self.subTest(variable=variable),
                patch.dict(os.environ, {variable: value}),
            ):
                with self.assertRaisesRegex(RuntimeError, "outside release"):
                    ASSETS.sign(self.release, TAG)

    def test_wrong_signer_primary_or_subkey_fails(self):
        self.signed()
        self.signature_primary = "C" * 40
        with self.assertRaisesRegex(RuntimeError, "unapproved PGP signer"):
            ASSETS.verify_release_signatures(self.release, TAG)
        self.signature_primary = PRIMARY
        self.signature_subkey = "D" * 40
        with self.assertRaisesRegex(RuntimeError, "unapproved PGP subkey"):
            ASSETS.verify_release_signatures(self.release, TAG)

    def test_bundled_keys_must_equal_the_pins_byte_for_byte(self):
        self.signed()
        cases = (
            (
                ASSETS.PUBLIC_PGP,
                "-----BEGIN PGP PRIVATE KEY BLOCK-----\n",
                "bundled PGP key",
            ),
            (ASSETS.PUBLIC_PGP, PUBLIC + "\n", "bundled PGP key"),
            (
                ASSETS.PUBLIC_MINI,
                "untrusted comment: other\n"
                + base64.b64encode(b"Ed" + b"z" * 40).decode()
                + "\n",
                "bundled minisign key",
            ),
            # Same key blob, different comment: still not the committed file.
            (
                ASSETS.PUBLIC_MINI,
                self.approved.read_text().replace("fixture", "renamed"),
                "bundled minisign key",
            ),
        )
        for name, content, reason in cases:
            with self.subTest(name=name, reason=reason):
                original = (self.release / name).read_bytes()
                (self.release / name).write_text(content)
                with self.assertRaisesRegex(RuntimeError, reason):
                    ASSETS.verify_release_signatures(self.release, TAG)
                (self.release / name).write_bytes(original)

    def test_operator_key_must_match_the_pin(self):
        with patch.dict(os.environ, {"GONEAT_PGP_KEY_ID": SIGNER + "!"}):
            fresh = self.root / "forced"
            fresh.mkdir()
            for name in self.originals:
                shutil.copyfile(self.release / name, fresh / name)
            ASSETS.sign(fresh, TAG)
        with patch.dict(os.environ, {"GONEAT_PGP_KEY_ID": "C" * 40 + "!"}):
            with self.assertRaisesRegex(RuntimeError, "committed signing subkey"):
                ASSETS.sign(self.release, TAG)
        self.operator_records = RECORDS.replace(PRIMARY, "C" * 40)
        with self.assertRaisesRegex(RuntimeError, "operator PGP identity differs"):
            ASSETS.sign(self.release, TAG)
        with self.assertRaisesRegex(RuntimeError, "exactly one"):
            ASSETS.primary_fingerprint(RECORDS + RECORDS)

    def test_sign_stages_pins_and_checks_all_outputs_without_changing_originals(self):
        before = self.snapshot()
        ASSETS.sign(self.release, TAG)
        self.assertEqual(
            before, {name: (self.release / name).read_bytes() for name in before}
        )
        self.assertTrue(
            all((self.release / name).is_file() for name in ASSETS.SIGNATURES)
        )
        self.assertEqual(
            (self.release / ASSETS.PUBLIC_PGP).read_bytes(), self.pin_pgp.read_bytes()
        )
        self.assertEqual(
            (self.release / ASSETS.PUBLIC_MINI).read_bytes(), self.approved.read_bytes()
        )
        signing = [a for a in self.calls if "--detach-sign" in a]
        self.assertTrue(
            signing
            and all(a[a.index("--local-user") + 1] == SIGNER + "!" for a in signing)
        )
        self.assertFalse(any("--export" in a for a in self.calls))
        self.assertEqual(
            len([a for a in self.calls if "--verify" in a or "-Vm" in a]), 4
        )
        self.assertFalse(list(self.root.glob(".goneat-sign-*")))

    def test_sign_failures_publish_nothing(self):
        for failure in ("SHA512SUMS.asc", "SHA512SUMS.minisig", "--list-secret-keys"):
            with self.subTest(failure=failure):
                before = self.snapshot()
                self.crypto_failure = failure
                with self.assertRaises(RuntimeError):
                    ASSETS.sign(self.release, TAG)
                self.assertEqual(before, self.snapshot())
                self.assertFalse(list(self.root.glob(".goneat-sign-*")))

    def test_sign_rejects_missing_manifest_ci_and_existing_output(self):
        with patch.dict(os.environ, {"CI": "true"}):
            with self.assertRaisesRegex(RuntimeError, "disabled in CI"):
                ASSETS.sign(self.release, TAG)
        (self.release / "SHA512SUMS").unlink()
        with self.assertRaises(RuntimeError):
            ASSETS.sign(self.release, TAG)
        self.assertEqual(self.calls, [])

    def remote(self):
        remote = self.root / "remote"
        remote.mkdir()
        for name in self.originals:
            shutil.copyfile(self.release / name, remote / name)
        state = {
            "body": "",
            "draft": False,
            "ids": {name: index + 100 for index, name in enumerate(self.originals)},
            "writes": [],
        }

        def gh(*argv):
            if argv[0] == "api":
                assets = [
                    {
                        "name": name,
                        "id": state["ids"][name],
                        "size": (remote / name).stat().st_size,
                        "digest": "sha256:" + ASSETS.digest(remote / name),
                        "updated_at": "unchanged",
                    }
                    for name in state["ids"]
                ]
                # gh api --paginate --slurp over the release list: pages of
                # releases, drafts included; another tag is never selected.
                self.assertIn("--slurp", argv)
                release = {
                    "tag_name": TAG,
                    "assets": assets,
                    "body": state["body"],
                    "draft": state["draft"],
                }
                return json.dumps(
                    [[{"tag_name": "v0.0.0-other", "assets": []}], [release]]
                )
            self.assertEqual(argv[0], "release")
            self.assertNotIn("--clobber", argv)
            if argv[1] == "download":
                name = argv[argv.index("--pattern") + 1]
                shutil.copyfile(
                    remote / name, Path(argv[argv.index("--dir") + 1]) / name
                )
            elif argv[1] == "upload":
                state["writes"].append(argv)
                for name in argv[5:]:
                    source = Path(name)
                    self.assertNotIn(source.name, self.originals)
                    self.assertNotIn(source.name, state["ids"])
                    shutil.copyfile(source, remote / source.name)
                    state["ids"][source.name] = max(state["ids"].values()) + 1
            elif argv[1] == "edit":
                state["writes"].append(argv)
                if "--notes-file" in argv:
                    state["body"] = Path(
                        argv[argv.index("--notes-file") + 1]
                    ).read_text()
                if "--draft=false" in argv:
                    state["draft"] = False
            else:
                self.fail("unexpected gh operation")
            return ""

        return remote, state, gh

    def test_download_fetches_and_verifies_original_manifests_without_reordering(self):
        _, _, gh = self.remote()
        target = self.root / "download"
        with patch.object(ASSETS, "gh", side_effect=gh):
            ASSETS.download(target, TAG)
        self.assertEqual(
            {name: (target / name).read_bytes() for name in self.originals},
            self.snapshot(),
        )
        with patch.object(
            ASSETS, "gh", side_effect=AssertionError("no network expected")
        ):
            with self.assertRaisesRegex(RuntimeError, "never overwritten"):
                ASSETS.download(target, TAG)

    def test_download_missing_or_bad_manifest_does_not_promote(self):
        remote, _, gh = self.remote()
        for mutation in ("bad", "missing"):
            with self.subTest(mutation=mutation):
                path = remote / "SHA512SUMS"
                original = (self.release / "SHA512SUMS").read_bytes()
                if mutation == "bad":
                    path.write_text("invalid")
                else:
                    path.unlink()
                target = self.root / ("download-" + mutation)
                with patch.object(ASSETS, "gh", side_effect=gh):
                    with self.assertRaises((RuntimeError, OSError)):
                        ASSETS.download(target, TAG)
                self.assertFalse(target.exists())
                self.assertFalse(list(self.root.glob(".goneat-download-*")))
                path.write_bytes(original)

    def test_upload_preserves_seven_ids_and_bytes_and_repeat_is_noop(self):
        self.signed()
        remote, state, gh = self.remote()
        before = {name: (remote / name).read_bytes() for name in self.originals}
        ids = dict(state["ids"])
        with patch.object(ASSETS, "gh", side_effect=gh):
            ASSETS.upload(self.release, TAG)
        self.assertEqual(ids, {name: state["ids"][name] for name in ids})
        self.assertEqual(
            before, {name: (remote / name).read_bytes() for name in before}
        )
        self.assertEqual(len(state["writes"]), 2)
        state["writes"].clear()
        with patch.object(ASSETS, "gh", side_effect=gh):
            ASSETS.upload(self.release, TAG)
        self.assertEqual(state["writes"], [])

    def test_upload_publishes_a_draft_only_after_verification(self):
        self.signed()
        _, state, gh = self.remote()
        state["draft"] = True
        with patch.object(ASSETS, "gh", side_effect=gh):
            ASSETS.upload(self.release, TAG)
        self.assertFalse(state["draft"])
        publish = [w for w in state["writes"] if "--draft=false" in w]
        self.assertEqual(len(publish), 1)
        self.assertIs(
            state["writes"][-1], publish[0], "publish is the last remote write"
        )
        self.assertIn("--latest", publish[0])

    def test_failed_preflight_leaves_the_draft_unpublished(self):
        self.signed()
        remote, state, gh = self.remote()
        state["draft"] = True
        (remote / "SHA256SUMS").write_bytes(b"different")
        with patch.object(ASSETS, "gh", side_effect=gh):
            with self.assertRaisesRegex(RuntimeError, "remote bytes differ"):
                ASSETS.upload(self.release, TAG)
        self.assertTrue(state["draft"])
        self.assertEqual(state["writes"], [])

    def test_upload_all_preflight_failures_precede_every_remote_write(self):
        self.signed()
        remote, state, gh = self.remote()
        notes = f"release-notes-{TAG}.md"
        shutil.copyfile(self.release / notes, remote / notes)
        state["ids"][notes] = 500
        for name in (self.names[0], "SHA256SUMS", notes):
            with self.subTest(name=name):
                original = (remote / name).read_bytes()
                (remote / name).write_bytes(b"different")
                with patch.object(ASSETS, "gh", side_effect=gh):
                    with self.assertRaisesRegex(RuntimeError, "remote bytes differ"):
                        ASSETS.upload(self.release, TAG)
                self.assertEqual(state["writes"], [])
                (remote / name).write_bytes(original)
        self.crypto_failure = "SHA512SUMS.minisig"
        with patch.object(
            ASSETS, "gh", side_effect=AssertionError("no remote expected")
        ):
            with self.assertRaises(RuntimeError):
                ASSETS.upload(self.release, TAG)

    def test_upload_identity_change_is_detected_before_write(self):
        self.signed()
        _, state, gh = self.remote()
        reads = 0

        def changing(*argv):
            nonlocal reads
            if argv[0] == "api":
                reads += 1
                if reads == 2:
                    state["ids"]["SHA256SUMS"] += 1
            return gh(*argv)

        with patch.object(ASSETS, "gh", side_effect=changing):
            with self.assertRaisesRegex(RuntimeError, "identities changed"):
                ASSETS.upload(self.release, TAG)
        self.assertEqual(state["writes"], [])


if __name__ == "__main__":
    unittest.main()

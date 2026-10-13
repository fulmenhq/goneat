#!/usr/bin/env python3
"""Download, verify, sign and supplement a release without replacing CI assets."""

import argparse
import base64
from contextlib import contextmanager
import hashlib
import json
import os
from pathlib import Path
import re
import shutil
import subprocess
import sys
import tempfile

REPOSITORY = "fulmenhq/goneat"
SCRIPTS = Path(__file__).resolve().parent
MANIFESTS = ("SHA256SUMS", "SHA512SUMS")
PUBLIC_PGP = "fulmenhq-release-signing-key.asc"
PUBLIC_MINI = "fulmenhq-release-minisign.pub"
SIGNATURES = tuple(
    name + suffix for name in MANIFESTS for suffix in (".asc", ".minisig")
)
# Committed public trust root. Verification reads these as git objects from
# the release tag (or GONEAT_RELEASE_PIN_REF), never from the working tree.
PIN_ANCHORS = "keys/expected-fingerprints.txt"
PIN_PGP = "docs/security/release-signing-keys.asc"
PIN_MINI = "docs/security/release-minisign.pub"


def require(condition, message):
    if not condition:
        raise RuntimeError(message)


def run(argv, **kwargs):
    result = subprocess.run(argv, check=False, **kwargs)
    require(
        result.returncode == 0,
        f"{Path(argv[0]).name} failed (exit {result.returncode})",
    )
    return result


def capture(argv):
    return run(argv, capture_output=True, text=True).stdout


def tool(name):
    require(shutil.which(name), f"required tool not found: {name}")


def regular(path):
    require(
        path.is_file() and not path.is_symlink(),
        f"regular non-symlink file required: {path.name}",
    )


def digest(path, algorithm="sha256"):
    regular(path)
    value = hashlib.new(algorithm)
    with path.open("rb") as stream:
        for data in iter(lambda: stream.read(1024 * 1024), b""):
            value.update(data)
    return value.hexdigest()


def archive_names(tag):
    require(
        re.fullmatch(r"v[0-9]+\.[0-9]+\.[0-9]+(?:-[0-9A-Za-z.-]+)?", tag),
        "expected release tag vX.Y.Z",
    )
    targets = capture(
        [
            "bash",
            "--noprofile",
            "--norc",
            "-c",
            'source "$1"; printf "%s\\n" "${RELEASE_TARGETS[@]}"',
            "release-platforms",
            str(SCRIPTS / "release-platforms.sh"),
        ]
    ).splitlines()
    require(
        len(targets) == 5 and len(set(targets)) == 5, "invalid release platform matrix"
    )
    require(
        all(
            re.fullmatch(r"(?:darwin|linux|windows)/(?:amd64|arm64)", t)
            for t in targets
        ),
        "invalid release target",
    )
    return [
        f"goneat_{tag}_{t.replace('/', '_')}.{'zip' if t.startswith('windows/') else 'tar.gz'}"
        for t in targets
    ]


def verify_archives(directory, tag):
    names = set(archive_names(tag))
    require(
        directory.is_dir() and not directory.is_symlink(),
        "release directory must be a non-symlink directory",
    )
    archives = {
        f.name for f in directory.iterdir() if f.name.endswith((".zip", ".tar.gz"))
    }
    require(archives == names, "expected exactly the five release archives")
    for manifest, algorithm in zip(MANIFESTS, ("sha256", "sha512")):
        path = directory / manifest
        regular(path)
        lines = path.read_text().splitlines()
        require(len(lines) == len(names), f"incorrect {manifest} entry count")
        seen = set()
        for line in lines:
            match = re.fullmatch(r"([0-9a-fA-F]+) [ *](\S+)", line)
            require(match, f"invalid {manifest} entry")
            expected, name = match.groups()
            require(
                name in names and name not in seen,
                f"unexpected or duplicate {manifest} name",
            )
            require(
                len(expected) == hashlib.new(algorithm).digest_size * 2,
                f"invalid {manifest} digest length",
            )
            require(
                digest(directory / name, algorithm) == expected.lower(),
                f"{manifest} mismatch: {name}",
            )
            seen.add(name)
    return sorted(names) + list(MANIFESTS)


def setting(name):
    prefix = os.environ.get("SIGNING_ENV_PREFIX", "GONEAT")
    return os.environ.get(prefix + "_" + name) or os.environ.get(name, "")


def outside(path, directory, label):
    value = path.resolve(strict=True)
    require(
        value != directory and directory not in value.parents,
        f"{label} must be independently approved outside release directory",
    )
    return value


def primary_fingerprint(records):
    rows = [line.split(":") for line in records.splitlines()]
    require(
        not any(row[0] in ("sec", "ssb") for row in rows),
        "secret key material is not a release asset",
    )
    require(
        sum(row[0] == "pub" for row in rows) == 1,
        "expected exactly one public primary key",
    )
    for index, row in enumerate(rows):
        if row[0] == "pub":
            require(
                index + 1 < len(rows) and rows[index + 1][0] == "fpr",
                "missing primary fingerprint",
            )
            return rows[index + 1][9].upper()
    raise RuntimeError("public primary fingerprint not found")


def mini_key(path):
    regular(path)
    lines = path.read_text().splitlines()
    require(
        len(lines) == 2 and lines[0].startswith("untrusted comment:"),
        "invalid minisign public key",
    )
    key = base64.b64decode(lines[1], validate=True)
    require(len(key) == 42 and key[:2] == b"Ed", "invalid minisign public key record")
    return key


def prefixed(name):
    # Trust selectors are read only under the signing prefix, never unprefixed.
    return os.environ.get(
        os.environ.get("SIGNING_ENV_PREFIX", "GONEAT") + "_" + name, ""
    )


def pin_mode():
    mode = prefixed("RELEASE_TRUST") or "pin"
    require(mode in ("pin", "env"), "RELEASE_TRUST must be pin or env")
    return mode


def committed(commit, path, destination):
    listing = capture(["git", "-C", str(SCRIPTS.parent), "ls-tree", commit, "--", path])
    fields = listing.split()
    require(
        len(fields) == 4 and fields[0] in ("100644", "100755") and fields[1] == "blob",
        f"committed regular file required in pin ref: {path} "
        "(tags before the pin first shipped need GONEAT_RELEASE_PIN_REF)",
    )
    data = run(
        ["git", "-C", str(SCRIPTS.parent), "cat-file", "blob", fields[2]],
        capture_output=True,
    ).stdout
    require(data, f"committed pin file is empty: {path}")
    destination.write_bytes(data)
    return data


def pin_commit(tag):
    ref = prefixed("RELEASE_PIN_REF") or f"refs/tags/{tag}"
    result = subprocess.run(
        [
            "git",
            "-C",
            str(SCRIPTS.parent),
            "rev-parse",
            "--verify",
            "--quiet",
            ref + "^{commit}",
        ],
        capture_output=True,
        text=True,
        check=False,
    )
    require(
        result.returncode == 0,
        f"pin ref not found: {ref} (fetch the release tag, or set GONEAT_RELEASE_PIN_REF)",
    )
    return result.stdout.strip()


@contextmanager
def pinned_trust(tag):
    """Trust inputs built only from the public pin committed at the pin ref."""
    for name in ("git", "gpg", "minisign"):
        tool(name)
    commit = pin_commit(tag)
    # GPG agent Unix socket paths are bounded; macOS TMPDIR can already be long.
    with tempfile.TemporaryDirectory(prefix="goneat-pin-", dir="/tmp") as tmp:
        root = Path(tmp)
        home = root / "gpg"
        home.mkdir(mode=0o700)
        try:
            anchors = committed(commit, PIN_ANCHORS, root / "anchors")
            match = re.fullmatch(
                rb"gpg ([0-9A-F]{40})\nminisign ([0-9a-f]{64})\n", anchors
            )
            require(
                match,
                "fingerprint anchors must be exactly a gpg line then a minisign line",
            )
            anchor_primary = match.group(1).decode()
            committed(commit, PIN_MINI, root / "minisign.pub")
            require(
                hashlib.sha256(mini_key(root / "minisign.pub")).hexdigest()
                == match.group(2).decode(),
                "pinned minisign key differs from its anchor",
            )
            pgp = committed(commit, PIN_PGP, root / "pgp.asc")
            require(b"PRIVATE KEY" not in pgp.upper(), "pin contains private key armor")
            run(
                [
                    "gpg",
                    "--batch",
                    "--quiet",
                    "--homedir",
                    str(home),
                    "--import",
                    str(root / "pgp.asc"),
                ],
                capture_output=True,
            )
            records = capture(
                [
                    "gpg",
                    "--batch",
                    "--homedir",
                    str(home),
                    "--with-colons",
                    "--with-subkey-fingerprint",
                    "--list-keys",
                ]
            )
            require(
                not capture(
                    [
                        "gpg",
                        "--batch",
                        "--homedir",
                        str(home),
                        "--with-colons",
                        "--list-secret-keys",
                    ]
                ).strip(),
                "pin contains secret key material",
            )
            primary = primary_fingerprint(records)
            require(primary == anchor_primary, "pinned PGP key differs from its anchor")
            rows = [line.split(":") for line in records.splitlines()]
            signing = [
                rows[index + 1][9].upper()
                for index, row in enumerate(rows)
                if row[0] == "sub"
                and "s" in row[11]
                and index + 1 < len(rows)
                and rows[index + 1][0] == "fpr"
            ]
            require(len(signing) == 1, "pin must hold exactly one signing subkey")
            yield home, primary, primary, signing[0], root / "minisign.pub"
        finally:
            if shutil.which("gpgconf"):
                subprocess.run(
                    ["gpgconf", "--homedir", str(home), "--kill", "all"],
                    check=False,
                    capture_output=True,
                )


@contextmanager
def release_trust(directory, tag):
    if pin_mode() == "env":
        yield trust_inputs(directory)
    else:
        with pinned_trust(tag) as trust:
            yield trust


def trust_inputs(directory):
    for name in ("gpg", "minisign"):
        tool(name)
    homedir = setting("GPG_HOMEDIR")
    identity = setting("PGP_KEY_ID")
    pub = setting("MINISIGN_PUB")
    require(
        homedir and identity and pub,
        "GPG_HOMEDIR, PGP_KEY_ID and MINISIGN_PUB are mandatory approved trust inputs",
    )
    home = outside(Path(homedir), directory, "GPG_HOMEDIR")
    require(home.is_dir(), "approved GPG homedir not found")
    public = outside(Path(pub), directory, "MINISIGN_PUB")
    mini_key(public)
    records = capture(
        [
            "gpg",
            "--batch",
            "--no-auto-key-retrieve",
            "--homedir",
            str(home),
            "--with-colons",
            "--fingerprint",
            "--fingerprint",
            "--list-keys",
            identity.rstrip("!"),
        ]
    )
    primary = primary_fingerprint(records)
    # An explicitly selected full subkey fingerprint must sign the manifests.
    selected = identity.rstrip("!").upper()
    exact_subkey = (
        selected
        if re.fullmatch(r"[0-9A-F]{40,64}", selected)
        and (selected != primary or identity.endswith("!"))
        else None
    )
    return home, identity, primary, exact_subkey, public


def verify_public_material(directory, trust):
    home, _, primary, _, approved_mini = trust
    pgp = directory / PUBLIC_PGP
    mini = directory / PUBLIC_MINI
    regular(pgp)
    require(
        b"PRIVATE KEY" not in pgp.read_bytes().upper(),
        "private key armor is not a release asset",
    )
    packets = capture(
        ["gpg", "--batch", "--homedir", str(home), "--list-packets", str(pgp)]
    )
    require(
        ":secret key packet:" not in packets
        and ":secret sub key packet:" not in packets,
        "secret key packets are not release assets",
    )
    records = capture(
        [
            "gpg",
            "--batch",
            "--homedir",
            str(home),
            "--with-colons",
            "--fingerprint",
            "--fingerprint",
            "--show-keys",
            str(pgp),
        ]
    )
    require(
        primary_fingerprint(records) == primary,
        "bundled PGP key does not match approved identity",
    )
    require(
        mini_key(mini) == mini_key(approved_mini),
        "bundled minisign key does not match approved key",
    )
    return {
        row.split(":")[9].upper()
        for row in records.splitlines()
        if row.startswith("fpr:")
    }


def verify_signatures(directory, trust):
    directory = directory.resolve(strict=True)
    home, _, primary, exact_subkey, approved_mini = trust
    for name in (*MANIFESTS, *SIGNATURES, PUBLIC_PGP, PUBLIC_MINI):
        regular(directory / name)
    bundled_fingerprints = verify_public_material(directory, trust)
    for name in MANIFESTS:
        status = capture(
            [
                "gpg",
                "--batch",
                "--no-auto-key-retrieve",
                "--homedir",
                str(home),
                "--status-fd",
                "1",
                "--verify",
                str(directory / (name + ".asc")),
                str(directory / name),
            ]
        )
        valid = [
            line.split()
            for line in status.splitlines()
            if line.startswith("[GNUPG:] VALIDSIG ")
        ]
        require(len(valid) == 1, f"exactly one valid PGP signature required for {name}")
        require(
            not re.search(
                r"^\[GNUPG:\] (EXPKEYSIG|EXPSIG|REVKEYSIG|KEYREVOKED|BADSIG|ERRSIG) ",
                status,
                re.M,
            ),
            f"expired, revoked or bad PGP signature for {name}",
        )
        record = valid[0]
        require(len(record) >= 11, "incomplete GPG signature status")
        signer = record[2].upper()
        signing_primary = record[11].upper() if len(record) > 11 else signer
        require(signing_primary == primary, f"unapproved PGP signer for {name}")
        require(
            signer in bundled_fingerprints,
            f"bundled PGP key lacks verified signing key for {name}",
        )
        require(
            not exact_subkey or signer == exact_subkey,
            f"unapproved PGP subkey for {name}",
        )
        run(
            [
                "minisign",
                "-Vm",
                str(directory / name),
                "-x",
                str(directory / (name + ".minisig")),
                "-p",
                str(approved_mini),
            ],
            capture_output=True,
        )
    print(
        "All four checksum manifest signatures verified with independently approved trust inputs"
    )


def verify_release_signatures(directory, tag):
    with release_trust(directory.resolve(strict=True), tag) as trust:
        verify_signatures(directory, trust)
    print(
        "Trust source: committed release-key pin"
        if pin_mode() == "pin"
        else "Trust source: operator environment (GONEAT_RELEASE_TRUST=env)"
    )


def gh(*args):
    tool("gh")
    return capture(["gh", *args])


def download_file(tag, name, directory):
    require(not (directory / name).exists(), "download destination already occupied")
    gh(
        "release",
        "download",
        tag,
        "--repo",
        REPOSITORY,
        "--pattern",
        name,
        "--dir",
        str(directory),
    )
    regular(directory / name)


def promote_files(staging, directory, names):
    # No overwrite: a concurrently created destination causes a hard failure.
    for name in names:
        os.link(staging / name, directory / name)


def download(directory, tag):
    names = archive_names(tag) + list(MANIFESTS)
    require(not directory.is_symlink(), "symlink download destination refused")
    require(
        not directory.exists() or (directory.is_dir() and not any(directory.iterdir())),
        "download requires a new or empty directory; existing assets are never overwritten",
    )
    directory.parent.mkdir(parents=True, exist_ok=True)
    with tempfile.TemporaryDirectory(
        prefix=".goneat-download-", dir=directory.parent
    ) as tmp:
        staging = Path(tmp)
        for name in names:
            download_file(tag, name, staging)
        verify_archives(staging, tag)
        directory.mkdir(exist_ok=True)
        require(
            not directory.is_symlink() and not any(directory.iterdir()),
            "download destination changed before promotion",
        )
        promote_files(staging, directory, names)
    print(
        "Downloaded five archives and both original CI manifests; both checksum algorithms verified"
    )


def sign(directory, tag):
    require(os.environ.get("CI", "").lower() != "true", "signing is disabled in CI")
    directory = directory.resolve(strict=True)
    originals = verify_archives(directory, tag)
    trust = trust_inputs(directory)
    home, identity, primary, _, approved_mini = trust
    if pin_mode() == "pin":
        with pinned_trust(tag) as pin:
            require(
                primary == pin[2],
                "operator PGP identity differs from the committed pin",
            )
            require(
                mini_key(approved_mini) == mini_key(pin[4]),
                "operator minisign key differs from the committed pin",
            )
    key = setting("MINISIGN_KEY")
    require(key, "MINISIGN_KEY is required")
    private_mini = outside(Path(key), directory, "MINISIGN_KEY")
    regular(private_mini)
    capture(
        [
            "gpg",
            "--batch",
            "--homedir",
            str(home),
            "--list-secret-keys",
            identity.rstrip("!"),
        ]
    )
    outputs = list(SIGNATURES) + [PUBLIC_PGP, PUBLIC_MINI]
    require(
        not any(
            (directory / name).exists() or (directory / name).is_symlink()
            for name in outputs
        ),
        "existing signing outputs refused; verify them or use a separate fresh download directory",
    )
    before = {name: digest(directory / name) for name in originals}
    with tempfile.TemporaryDirectory(
        prefix=".goneat-sign-", dir=directory.parent
    ) as tmp:
        staging = Path(tmp)
        for name in MANIFESTS:
            shutil.copyfile(directory / name, staging / name)
        for name in MANIFESTS:
            run(
                [
                    "minisign",
                    "-S",
                    "-s",
                    str(private_mini),
                    "-m",
                    str(staging / name),
                    "-x",
                    str(staging / (name + ".minisig")),
                    "-t",
                    f"goneat {tag} {name}",
                ]
            )
            run(
                [
                    "gpg",
                    "--batch",
                    "--armor",
                    "--homedir",
                    str(home),
                    "--local-user",
                    identity,
                    "--detach-sign",
                    "-o",
                    str(staging / (name + ".asc")),
                    str(staging / name),
                ]
            )
        (staging / PUBLIC_PGP).write_text(
            capture(
                [
                    "gpg",
                    "--batch",
                    "--homedir",
                    str(home),
                    "--armor",
                    "--export",
                    identity.rstrip("!"),
                ]
            )
        )
        shutil.copyfile(approved_mini, staging / PUBLIC_MINI)
        verify_signatures(staging, trust)
        if pin_mode() == "pin":
            with pinned_trust(tag) as pin:
                verify_signatures(staging, pin)
        require(
            before == {name: digest(directory / name) for name in originals},
            "original release bytes changed during signing",
        )
        promote_files(staging, directory, outputs)
    print(
        "Signed both original manifests; all four signatures verified before publishing local outputs"
    )


def remote_assets(tag):
    release = json.loads(gh("api", f"repos/{REPOSITORY}/releases/tags/{tag}"))
    require(release.get("tag_name") == tag, "remote release tag mismatch")
    assets = release["assets"]
    require(
        len({a["name"] for a in assets}) == len(assets),
        "duplicate remote release asset names",
    )
    return release, {asset["name"]: asset for asset in assets}


def original_identity(assets, originals):
    require(
        all(name in assets for name in originals),
        "original CI assets missing from remote release",
    )
    return {
        name: {
            field: assets[name].get(field)
            for field in ("id", "size", "digest", "updated_at")
        }
        for name in originals
    }


def compare_download(tag, directory, names, temporary):
    for name in names:
        download_file(tag, name, temporary)
        require(
            digest(temporary / name) == digest(directory / name),
            f"remote bytes differ: {name}",
        )


def upload(directory, tag):
    originals = verify_archives(directory, tag)
    verify_release_signatures(directory, tag)
    notes = f"release-notes-{tag}.md"
    supplements = [*SIGNATURES, PUBLIC_PGP, PUBLIC_MINI, notes]
    for name in supplements + ["release-notes.md"]:
        regular(directory / name)
    require(
        (directory / notes).read_bytes()
        == (directory / "release-notes.md").read_bytes(),
        "versioned and body release notes differ",
    )
    # Read and compare every existing asset before the first remote mutation.
    release, assets = remote_assets(tag)
    original_ids = original_identity(assets, originals)
    local = {
        name: digest(directory / name)
        for name in originals + supplements + ["release-notes.md"]
    }
    with tempfile.TemporaryDirectory(prefix="goneat-upload-") as tmp:
        temporary = Path(tmp)
        compare_download(tag, directory, originals, temporary)
        existing = [name for name in supplements if name in assets]
        compare_download(tag, directory, existing, temporary)
        _, current = remote_assets(tag)
        require(
            original_identity(current, originals) == original_ids,
            "remote original asset identities changed during preflight",
        )
        require(
            {name: current[name]["id"] for name in existing if name in current}
            == {name: assets[name]["id"] for name in existing},
            "remote supplement identities changed during preflight",
        )
        require(
            not any(name in current for name in supplements if name not in assets),
            "remote supplement appeared during preflight",
        )
        require(
            local == {name: digest(directory / name) for name in local},
            "local assets changed during preflight",
        )
        missing = [name for name in supplements if name not in assets]
        if missing:
            gh(
                "release",
                "upload",
                tag,
                "--repo",
                REPOSITORY,
                *(str(directory / name) for name in missing),
            )
        body = (directory / "release-notes.md").read_text()
        if (release.get("body") or "") != body:
            gh(
                "release",
                "edit",
                tag,
                "--repo",
                REPOSITORY,
                "--notes-file",
                str(directory / "release-notes.md"),
            )
        _, after = remote_assets(tag)
        require(
            original_identity(after, originals) == original_ids,
            "remote original assets changed after supplementation",
        )
        require(
            all(name in after for name in supplements),
            "uploaded supplement inventory incomplete",
        )
        require(
            all(after[name]["id"] == assets[name]["id"] for name in existing),
            "existing supplement asset IDs changed",
        )
        with tempfile.TemporaryDirectory(prefix="verify-", dir=temporary) as final:
            compare_download(tag, directory, originals + supplements, Path(final))
    print(
        "Supplemented signatures, public keys and notes; original seven asset IDs and bytes preserved"
    )


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument(
        "operation",
        choices=(
            "download",
            "verify",
            "verify-signatures",
            "verify-pin",
            "sign",
            "upload",
        ),
    )
    parser.add_argument("tag")
    parser.add_argument("directory", type=Path, nargs="?", default=Path("dist/release"))
    args = parser.parse_args()
    archive_names(args.tag)
    directory = args.directory.absolute()
    require(not directory.is_symlink(), "symlink release directory refused")
    if args.operation == "verify-pin":
        # The tag argument names the pin ref; GONEAT_RELEASE_PIN_REF overrides it.
        with pinned_trust(args.tag) as trust:
            _, primary, _, subkey, mini = trust
            print(
                f"Committed pin is consistent: PGP primary {primary}, signing subkey {subkey}, "
                f"minisign key {hashlib.sha256(mini_key(mini)).hexdigest()}"
            )
    elif args.operation == "verify-signatures":
        verify_archives(directory, args.tag)
        verify_release_signatures(directory, args.tag)
    elif args.operation == "verify":
        verify_archives(directory, args.tag)
        print(
            "Both original checksum manifests match exactly the five release archives"
        )
    else:
        {"download": download, "sign": sign, "upload": upload}[args.operation](
            directory, args.tag
        )


if __name__ == "__main__":
    try:
        main()
    except (RuntimeError, OSError, ValueError, KeyError) as error:
        print(f"error: {error}", file=sys.stderr)
        sys.exit(1)

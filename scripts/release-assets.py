#!/usr/bin/env python3
"""Download, verify, sign and supplement a release without replacing CI assets."""

import argparse
from collections import namedtuple
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
# Committed public trust root (3leaps/crucible PDR-0008). Staging and
# verification read these as git objects from the release tag, never from the
# working tree or the environment; release-verify-anchors.sh checks them with
# decernor. PUBLIC_PGP and PUBLIC_MINI are the published asset names; their
# bytes are the committed pins.
PIN_ANCHORS = "keys/expected-fingerprints.txt"
PIN_ANCHORS_NDJSON = "keys/expected-fingerprints.ndjson"
PIN_PGP = "docs/security/release-signing-keys.asc"
PIN_MINI = "docs/security/goneat-minisign.pub"
PIN_TAGGER = "config/release/tagger-identity.txt"
Trust = namedtuple("Trust", "home primary subkey pgp minisign layout commit")


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


def operator(name):
    # Signing inputs are read only under the GONEAT_ prefix, never unprefixed.
    return os.environ.get("GONEAT_" + name, "")


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


def pin_commit(ref):
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
    require(result.returncode == 0, f"pin ref not found: {ref} (fetch the release tag)")
    return result.stdout.strip()


def release_tag_of(ref):
    """The release tag a pin ref names, as a list for argv (empty otherwise)."""
    name = ref.removeprefix("refs/tags/")
    return (
        [name]
        if re.fullmatch(r"v[0-9]+\.[0-9]+\.[0-9]+(?:-[0-9A-Za-z.-]+)?", name)
        else []
    )


@contextmanager
def pinned_trust(ref):
    """Trust inputs built only from the public pin committed at ref.

    For a release the ref is always refs/tags/<tag>. The pins and anchors are
    read as git objects and verified by decernor (release-verify-anchors.sh);
    this module never decodes or hashes a key itself.
    """
    for name in ("git", "gpg", "bash"):
        tool(name)
    commit = pin_commit(ref)
    # GPG agent Unix socket paths are bounded; macOS TMPDIR can already be long.
    with tempfile.TemporaryDirectory(prefix="goneat-pin-", dir="/tmp") as tmp:
        root = Path(tmp)
        pins = root / "pins"
        pins.mkdir()
        home = root / "gpg"
        home.mkdir(mode=0o700)
        try:
            checked = subprocess.run(
                [
                    "bash",
                    str(SCRIPTS / "release-verify-anchors.sh"),
                    commit,
                    str(pins),
                    *release_tag_of(ref),
                ],
                capture_output=True,
                text=True,
                check=False,
            )
            require(
                checked.returncode == 0,
                "committed pin check failed: "
                + " ".join(
                    line.removeprefix("error: ")
                    for line in checked.stderr.splitlines()
                    if line.startswith("error: ")
                ),
            )
            match = re.fullmatch(
                r"gpg ([0-9A-F]{40})\nminisign [0-9a-f]{64}\n",
                (pins / "anchors.txt").read_text(),
            )
            require(
                match, "fingerprint anchors must be a gpg line then a minisign line"
            )
            anchor_primary = match.group(1)
            run(
                [
                    "gpg",
                    "--batch",
                    "--quiet",
                    "--homedir",
                    str(home),
                    "--import",
                    str(pins / "pin.asc"),
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
            require(
                primary_fingerprint(records) == anchor_primary,
                "pinned PGP key differs from its anchor",
            )
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
            yield Trust(
                home=home,
                primary=anchor_primary,
                subkey=signing[0],
                pgp=pins / "pin.asc",
                minisign=pins / "minisign.pub",
                layout=(pins / "layout").read_text().strip(),
                commit=commit,
            )
        finally:
            if shutil.which("gpgconf"):
                subprocess.run(
                    ["gpgconf", "--homedir", str(home), "--kill", "all"],
                    check=False,
                    capture_output=True,
                )


def verify_public_material(directory, trust):
    # The published keys are the committed pins, byte for byte.
    for name, pin in ((PUBLIC_PGP, trust.pgp), (PUBLIC_MINI, trust.minisign)):
        regular(directory / name)
        require(
            (directory / name).read_bytes() == pin.read_bytes(),
            f"bundled {'PGP' if name == PUBLIC_PGP else 'minisign'} key "
            "does not match the committed pin",
        )


def verify_signatures(directory, trust):
    directory = directory.resolve(strict=True)
    tool("minisign")
    for name in (*MANIFESTS, *SIGNATURES, PUBLIC_PGP, PUBLIC_MINI):
        regular(directory / name)
    verify_public_material(directory, trust)
    for name in MANIFESTS:
        status = capture(
            [
                "gpg",
                "--batch",
                "--no-auto-key-retrieve",
                "--homedir",
                str(trust.home),
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
        require(signing_primary == trust.primary, f"unapproved PGP signer for {name}")
        require(signer == trust.subkey, f"unapproved PGP subkey for {name}")
        run(
            [
                "minisign",
                "-Vm",
                str(directory / name),
                "-x",
                str(directory / (name + ".minisig")),
                "-p",
                str(trust.minisign),
            ],
            capture_output=True,
        )
    print("All four checksum manifest signatures verified against the committed pin")


def verify_release_signatures(directory, tag):
    directory.resolve(strict=True)
    with pinned_trust(f"refs/tags/{tag}") as trust:
        verify_signatures(directory, trust)
        print(
            f"Trust source: release-key pin committed at {tag} "
            f"({trust.commit}, {trust.layout} layout)"
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
    for name in ("gpg", "minisign"):
        tool(name)
    homedir = operator("GPG_HOMEDIR")
    identity = operator("PGP_KEY_ID")
    key = operator("MINISIGN_KEY")
    require(
        homedir and identity and key,
        "GONEAT_GPG_HOMEDIR, GONEAT_PGP_KEY_ID and GONEAT_MINISIGN_KEY are required to sign",
    )
    home = outside(Path(homedir), directory, "GONEAT_GPG_HOMEDIR")
    require(home.is_dir(), "approved GPG homedir not found")
    private_mini = outside(Path(key), directory, "GONEAT_MINISIGN_KEY")
    regular(private_mini)
    outputs = list(SIGNATURES) + [PUBLIC_PGP, PUBLIC_MINI]
    require(
        not any(
            (directory / name).exists() or (directory / name).is_symlink()
            for name in outputs
        ),
        "existing signing outputs refused; verify them or use a separate fresh download directory",
    )
    with pinned_trust(f"refs/tags/{tag}") as pin:
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
        require(
            primary_fingerprint(records) == pin.primary,
            "operator PGP identity differs from the committed pin",
        )
        selected = identity.rstrip("!").upper()
        require(
            selected == pin.primary
            or selected == pin.subkey
            or not re.fullmatch(r"[0-9A-F]{40}", selected),
            "operator PGP subkey differs from the committed signing subkey",
        )
        signer = pin.subkey + "!"
        capture(
            ["gpg", "--batch", "--homedir", str(home), "--list-secret-keys", signer]
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
                        signer,
                        "--detach-sign",
                        "-o",
                        str(staging / (name + ".asc")),
                        str(staging / name),
                    ]
                )
            # The published public keys are the pins committed at the tag.
            shutil.copyfile(pin.pgp, staging / PUBLIC_PGP)
            shutil.copyfile(pin.minisign, staging / PUBLIC_MINI)
            verify_signatures(staging, pin)
            require(
                before == {name: digest(directory / name) for name in originals},
                "original release bytes changed during signing",
            )
            promote_files(staging, directory, outputs)
    print(
        "Signed both original manifests; all four signatures verified against the committed pin before publishing local outputs"
    )


def remote_assets(tag):
    # The release stays a draft until upload publishes it. releases/tags/<tag>
    # does not return drafts, so list the releases and select by tag.
    releases = json.loads(
        gh("api", "--paginate", "--slurp", f"repos/{REPOSITORY}/releases?per_page=100")
    )
    matches = [r for page in releases for r in page if r.get("tag_name") == tag]
    require(
        len(matches) == 1,
        f"expected exactly one release for {tag}, found {len(matches)}",
    )
    release = matches[0]
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
        # Publish only now: every asset and signature has been verified in place.
        if release.get("draft"):
            prerelease = "-" in tag
            gh(
                "release",
                "edit",
                tag,
                "--repo",
                REPOSITORY,
                "--draft=false",
                *(("--prerelease", "--latest=false") if prerelease else ("--latest",)),
            )
            require(
                not remote_assets(tag)[0].get("draft"),
                "release is still a draft after publishing",
            )
            print(f"Published release {tag}")
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
    parser.add_argument("tag", help="release tag vX.Y.Z (verify-pin: a commit)")
    parser.add_argument("directory", type=Path, nargs="?", default=Path("dist/release"))
    args = parser.parse_args()
    if args.operation == "verify-pin":
        # Checks the pin committed at a commit (default HEAD in the Makefile);
        # it signs, stages and publishes nothing.
        with pinned_trust(args.tag) as trust:
            print(
                f"Committed pin at {trust.commit} is consistent ({trust.layout} layout): "
                f"PGP primary {trust.primary}, signing subkey {trust.subkey}"
            )
        return
    archive_names(args.tag)
    directory = args.directory.absolute()
    require(not directory.is_symlink(), "symlink release directory refused")
    if args.operation == "verify-signatures":
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

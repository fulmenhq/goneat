#!/usr/bin/env python3
"""Exercise the distributed candidate bytes on a native runner, without scanners."""

import argparse
import hashlib
import json
import os
from pathlib import Path
import platform
import struct
import subprocess
import tarfile
import tempfile
import zipfile


def require(condition, message):
    if not condition:
        raise RuntimeError(message)


def run(binary, cwd, args, success=True):
    result = subprocess.run(
        [str(binary), *args], cwd=cwd, capture_output=True, text=True, timeout=60
    )
    require(
        (result.returncode == 0) == success,
        f"{args}: unexpected exit {result.returncode}\n{result.stdout}\n{result.stderr}",
    )
    return result.stdout


def check_binary(data, target):
    os_name, arch = target.split("/")
    if os_name == "windows":
        require(data[:2] == b"MZ", "not a PE executable")
        offset = struct.unpack_from("<I", data, 0x3C)[0]
        require(data[offset : offset + 4] == b"PE\0\0", "missing PE signature")
        machine = struct.unpack_from("<H", data, offset + 4)[0]
        require(
            machine == {"amd64": 0x8664, "arm64": 0xAA64}[arch],
            "PE architecture mismatch",
        )
    elif os_name == "linux":
        require(data[:6] == b"\x7fELF\x02\x01", "not a little-endian ELF64 executable")
        machine = struct.unpack_from("<H", data, 18)[0]
        require(
            machine == {"amd64": 62, "arm64": 183}[arch], "ELF architecture mismatch"
        )
    else:
        require(data[:4] == b"\xcf\xfa\xed\xfe", "not a Mach-O64 executable")
        require(
            struct.unpack_from("<I", data, 4)[0] == 0x100000C,
            "Mach-O architecture mismatch",
        )


def verify_manifests(directory, version):
    targets = [
        "linux_amd64",
        "linux_arm64",
        "darwin_arm64",
        "windows_amd64",
        "windows_arm64",
    ]
    names = {
        f"goneat_{version}_{target}.{'zip' if target.startswith('windows') else 'tar.gz'}"
        for target in targets
    }
    archives = {
        p.name for p in directory.iterdir() if p.name.endswith((".tar.gz", ".zip"))
    }
    require(archives == names, f"archive inventory mismatch: {archives ^ names}")
    for algorithm in ("sha256", "sha512"):
        lines = (directory / f"{algorithm.upper()}SUMS").read_text().splitlines()
        require(len(lines) == len(names), f"incorrect {algorithm} manifest size")
        seen = set()
        for line in lines:
            digest, name = line.split()
            require(
                name in names and name not in seen,
                f"unexpected/duplicate manifest name: {name}",
            )
            seen.add(name)
            require(
                hashlib.new(algorithm, (directory / name).read_bytes()).hexdigest()
                == digest,
                f"{algorithm} mismatch for {name}",
            )


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--directory", type=Path, required=True)
    parser.add_argument("--target", required=True)
    parser.add_argument("--version", required=True)
    parser.add_argument("--source-sha", required=True)
    args = parser.parse_args()
    os_name, arch = args.target.split("/")
    host_os = {"Darwin": "darwin", "Linux": "linux", "Windows": "windows"}.get(
        platform.system()
    )
    host_arch = {
        "x86_64": "amd64",
        "amd64": "amd64",
        "aarch64": "arm64",
        "arm64": "arm64",
    }.get(platform.machine().lower())
    require(
        (host_os, host_arch) == (os_name, arch),
        f"runner identity mismatch: {host_os}/{host_arch}",
    )
    # Explicit Windows host hardware evidence as well as the goneat process below.
    if os_name == "windows":
        native_arch = os.environ.get("PROCESSOR_ARCHITEW6432") or os.environ.get(
            "PROCESSOR_ARCHITECTURE", ""
        )
        require(
            native_arch.lower() == {"amd64": "amd64", "arm64": "arm64"}[arch],
            "Windows host architecture mismatch",
        )
    directory = args.directory.resolve()
    provenance = json.loads((directory / "candidate.json").read_text())
    require(
        provenance
        == {
            "source_sha": args.source_sha,
            "version": args.version,
            "compiler": "go1.26.6",
        },
        "candidate provenance mismatch",
    )
    verify_manifests(directory, args.version)
    archive = (
        directory
        / f"goneat_{args.version}_{os_name}_{arch}.{'zip' if os_name == 'windows' else 'tar.gz'}"
    )
    binary_name = "goneat.exe" if os_name == "windows" else "goneat"
    # Read only the named binary rather than extracting arbitrary archive paths.
    if os_name == "windows":
        with zipfile.ZipFile(archive) as zipped:
            data = zipped.read(binary_name)
    else:
        with tarfile.open(archive) as tar:
            member = tar.getmember(binary_name)
            require(member.isfile(), "candidate executable is not a regular file")
            data = tar.extractfile(member).read()
    check_binary(data, args.target)
    with tempfile.TemporaryDirectory(prefix="goneat-native-") as temp:
        cwd = Path(temp)
        binary = cwd / binary_name
        binary.write_bytes(data)
        binary.chmod(0o755)
        info = json.loads(run(binary, cwd, ["version", "--extended", "--json"]))
        require(info["binaryVersion"] == args.version, "binary version mismatch")
        require(
            (info["platform"], info["arch"]) == (os_name, arch),
            "goneat process architecture mismatch",
        )
        require(info["goVersion"] == "go1.26.6", "compiler mismatch")
        # JSON's gitCommit describes the working directory; the text form reads
        # buildinfo.GitCommit. Run outside the source tree and check that value.
        extended = run(binary, cwd, ["version", "--extended"])
        require(
            f"Git commit: {args.source_sha[:8]}\n" in extended,
            "embedded source revision mismatch",
        )
        run(binary, cwd, ["--help"])
        # Spaces and non-ASCII names exercise actual file/path handling.
        fixture = cwd / "native paths café"
        fixture.mkdir()
        schema = fixture / "schema.json"
        schema.write_text(
            json.dumps(
                {
                    "$schema": "https://json-schema.org/draft-07/schema#",
                    "type": "object",
                    "required": ["name"],
                    "properties": {"name": {"type": "string"}},
                    "additionalProperties": False,
                }
            )
        )
        good = fixture / "valid.json"
        good.write_text('{"name":"native"}')
        bad = fixture / "invalid.json"
        bad.write_text('{"name":42}')
        run(binary, cwd, ["schema", "validate-schema", str(schema)])
        invalid_schema = fixture / "invalid-schema.json"
        invalid_schema.write_text(
            '{"$schema":"https://json-schema.org/draft-07/schema#","type":42}'
        )
        run(
            binary,
            cwd,
            ["schema", "validate-schema", str(invalid_schema)],
            success=False,
        )
        run(
            binary,
            cwd,
            ["validate", "data", "--schema-file", str(schema), "--data", str(good)],
        )
        run(
            binary,
            cwd,
            ["validate", "data", "--schema-file", str(schema), "--data", str(bad)],
            success=False,
        )
        run(
            binary,
            cwd,
            [
                "validate",
                "data",
                "--schema-file",
                str(schema),
                "--data",
                str(fixture / "missing.json"),
            ],
            success=False,
        )
        run(binary, cwd, ["not-a-command"], success=False)
        print(
            json.dumps(
                {
                    "target": args.target,
                    "runner": platform.machine(),
                    "binary_sha256": hashlib.sha256(data).hexdigest(),
                    "identity": info,
                    "core_contract": "pass",
                },
                indent=2,
            )
        )


if __name__ == "__main__":
    main()

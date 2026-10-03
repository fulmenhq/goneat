"""Deterministic negative controls for candidate inventory and executable identity."""

import hashlib
import importlib.util
from pathlib import Path
import struct
import sys
import tempfile
import unittest

sys.dont_write_bytecode = True
spec = importlib.util.spec_from_file_location(
    "native_contract", Path(__file__).with_name("native-contract.py")
)
contract = importlib.util.module_from_spec(spec)
spec.loader.exec_module(contract)


class CandidateChecks(unittest.TestCase):
    def test_windows_architecture_is_not_interchangeable(self):
        data = bytearray(128)
        data[:2] = b"MZ"
        struct.pack_into("<I", data, 0x3C, 64)
        data[64:68] = b"PE\0\0"
        struct.pack_into("<H", data, 68, 0xAA64)
        contract.check_binary(data, "windows/arm64")
        with self.assertRaisesRegex(RuntimeError, "architecture mismatch"):
            contract.check_binary(data, "windows/amd64")

    def test_original_manifests_reject_tampering_and_retired_archives(self):
        with tempfile.TemporaryDirectory() as temp:
            directory = Path(temp)
            names = []
            for target in [
                "linux_amd64",
                "linux_arm64",
                "darwin_arm64",
                "windows_amd64",
                "windows_arm64",
            ]:
                name = f"goneat_v1.2.3_{target}.{'zip' if target.startswith('windows') else 'tar.gz'}"
                (directory / name).write_bytes(target.encode())
                names.append(name)
            for algorithm in ["sha256", "sha512"]:
                (directory / f"{algorithm.upper()}SUMS").write_text(
                    "".join(
                        f"{hashlib.new(algorithm, (directory / name).read_bytes()).hexdigest()}  {name}\n"
                        for name in names
                    )
                )
            contract.verify_manifests(directory, "v1.2.3")
            (directory / names[0]).write_bytes(b"tampered")
            with self.assertRaisesRegex(RuntimeError, "mismatch"):
                contract.verify_manifests(directory, "v1.2.3")
            (directory / "goneat_v1.2.3_darwin_amd64.tar.gz").write_bytes(b"retired")
            with self.assertRaisesRegex(RuntimeError, "inventory mismatch"):
                contract.verify_manifests(directory, "v1.2.3")


if __name__ == "__main__":
    unittest.main()

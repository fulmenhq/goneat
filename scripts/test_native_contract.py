"""Deterministic negative controls for candidate inventory and executable identity."""

import base64
import contextlib
import hashlib
import importlib.util
import io
import json
from pathlib import Path
import struct
import subprocess
import sys
import tempfile
import unittest
from unittest import mock

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


class CaptureChecks(unittest.TestCase):
    def capture(self, stdout=b"", stderr=b"", returncode=0, success=True):
        result = subprocess.CompletedProcess(["fixture"], returncode, stdout, stderr)
        output = io.StringIO()
        with mock.patch.object(
            contract.subprocess, "run", return_value=result
        ) as child:
            with contextlib.redirect_stdout(output):
                text = contract.run(Path("fixture"), Path("."), ["--help"], success)
        child.assert_called_once_with(
            ["fixture", "--help"], cwd=Path("."), capture_output=True, timeout=60
        )
        return text, json.loads(output.getvalue())

    def assert_stream(self, receipt, stream, expected):
        self.assertTrue(receipt[stream]["available"])
        self.assertEqual(receipt[stream]["bytes"], len(expected))
        self.assertEqual(
            receipt[stream]["sha256"], hashlib.sha256(expected).hexdigest()
        )
        self.assertEqual(base64.b64decode(receipt[stream]["base64"]), expected)

    def test_real_utf8_both_streams_zero_and_expected_nonzero(self):
        stdout = "❌ native café stdout\n".encode("utf-8")
        stderr = "❌ native café stderr\n".encode("utf-8")
        for code in (0, 7):
            with self.subTest(code=code), tempfile.TemporaryDirectory() as temp:
                output = io.StringIO()
                script = (
                    "import sys; "
                    f"sys.stdout.buffer.write({stdout!r}); "
                    f"sys.stderr.buffer.write({stderr!r}); sys.exit({code})"
                )
                with contextlib.redirect_stdout(output):
                    text = contract.run(
                        Path(sys.executable), Path(temp), ["-c", script], code == 0
                    )
                receipt = json.loads(output.getvalue())
                self.assertEqual(text, stdout.decode("utf-8"))
                self.assertEqual(receipt["capture_outcome"], "complete")
                self.assertEqual(receipt["returncode"], code)
                self.assertEqual(receipt["native_child"][0], sys.executable)
                self.assert_stream(receipt, "stdout", stdout)
                self.assert_stream(receipt, "stderr", stderr)

    def test_binary_capture_does_not_depend_on_cp1252_locale(self):
        data = "❌ café".encode("utf-8")
        text, receipt = self.capture(data, data)
        self.assertEqual(text, data.decode("utf-8"))
        self.assertEqual(receipt["encoding"], "utf-8")
        self.assert_stream(receipt, "stdout", data)
        self.assert_stream(receipt, "stderr", data)

    def test_invalid_utf8_in_either_stream_cannot_pass(self):
        for stream in ("stdout", "stderr"):
            for code in (0, 7):
                with self.subTest(stream=stream, code=code):
                    streams = {"stdout": b"valid", "stderr": b"valid"}
                    streams[stream] = b"\xff\xfe"
                    output = io.StringIO()
                    result = subprocess.CompletedProcess(["fixture"], code, **streams)
                    with mock.patch.object(
                        contract.subprocess, "run", return_value=result
                    ):
                        with contextlib.redirect_stdout(output):
                            with self.assertRaisesRegex(
                                RuntimeError, stream + " is not UTF-8"
                            ) as caught:
                                contract.run(
                                    Path("fixture"), Path("."), ["--help"], code == 0
                                )
                    self.assertIsInstance(
                        caught.exception.__cause__, UnicodeDecodeError
                    )
                    receipt = json.loads(output.getvalue())
                    self.assertEqual(receipt["capture_outcome"], "decode_error")
                    self.assertEqual(receipt["failed_stream"], stream)
                    self.assert_stream(receipt, stream, streams[stream])
                    self.assertNotIn("core_contract", output.getvalue())

    def test_missing_or_nonbyte_stream_is_not_empty(self):
        # Synthetic unavailable-reader controls, not a physical Windows fault.
        for stream in ("stdout", "stderr"):
            for missing in (None, "", bytearray()):
                with self.subTest(stream=stream, missing=missing):
                    streams = {"stdout": b"", "stderr": b""}
                    streams[stream] = missing
                    result = subprocess.CompletedProcess(["fixture"], 0, **streams)
                    output = io.StringIO()
                    with mock.patch.object(
                        contract.subprocess, "run", return_value=result
                    ):
                        with contextlib.redirect_stdout(output):
                            with self.assertRaisesRegex(
                                RuntimeError, "capture is unavailable"
                            ):
                                contract.run(Path("fixture"), Path("."), ["--help"])
                    receipt = json.loads(output.getvalue())
                    self.assertEqual(receipt["capture_outcome"], "unavailable_stream")
                    self.assertEqual(receipt[stream], {"available": False})
                    self.assertNotIn("core_contract", output.getvalue())

    def test_start_reader_io_and_timeout_errors_propagate_without_retry(self):
        for error in (
            FileNotFoundError("fixture start"),
            IOError("fixture reader"),
            subprocess.TimeoutExpired(
                ["fixture"], 60, output=b"partial", stderr=b"diagnostic"
            ),
        ):
            with self.subTest(error=type(error).__name__):
                output = io.StringIO()
                with mock.patch.object(
                    contract.subprocess, "run", side_effect=error
                ) as child:
                    with contextlib.redirect_stdout(output):
                        with self.assertRaises(type(error)) as caught:
                            contract.run(Path("fixture"), Path("."), ["--help"])
                self.assertIs(caught.exception, error)
                child.assert_called_once()
                self.assertEqual(child.call_args.kwargs["timeout"], 60)
                receipt = json.loads(output.getvalue())
                self.assertEqual(receipt["capture_outcome"], "execution_error")
                if isinstance(error, subprocess.TimeoutExpired):
                    self.assert_stream(receipt, "stdout", b"partial")
                    self.assert_stream(receipt, "stderr", b"diagnostic")
                self.assertNotIn("core_contract", output.getvalue())

    def test_unexpected_exit_keeps_lossless_stream_receipt(self):
        for code, expected_success in ((0, False), (7, True)):
            with self.subTest(code=code):
                result = subprocess.CompletedProcess(
                    ["fixture"], code, b"fixture output", b"fixture error"
                )
                output = io.StringIO()
                with mock.patch.object(contract.subprocess, "run", return_value=result):
                    with contextlib.redirect_stdout(output):
                        with self.assertRaisesRegex(RuntimeError, "unexpected exit"):
                            contract.run(
                                Path("fixture"), Path("."), ["--help"], expected_success
                            )
                receipt = json.loads(output.getvalue())
                self.assert_stream(receipt, "stdout", b"fixture output")
                self.assert_stream(receipt, "stderr", b"fixture error")
                self.assertNotIn("core_contract", output.getvalue())


if __name__ == "__main__":
    unittest.main()

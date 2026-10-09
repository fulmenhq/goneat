"""Deterministic, offline controls for the temporary selected-module restriction."""

from datetime import timedelta
import contextlib
import importlib.util
import io
import json
from pathlib import Path
import subprocess
import sys
import unittest
from unittest import mock

sys.dont_write_bytecode = True
spec = importlib.util.spec_from_file_location(
    "cooling_selection", Path(__file__).with_name("check-cooling-selection.py")
)
guard = importlib.util.module_from_spec(spec)
spec.loader.exec_module(guard)


class CoolingSelectionChecks(unittest.TestCase):
    def setUp(self):
        self.repo = Path("fixture")
        self.now = guard.EXPIRES - timedelta(days=1)
        self.selected = {
            "Path": guard.MODULE,
            "Version": guard.VERSION,
            "Indirect": True,
        }
        self.root = {"Require": [dict(self.selected)]}

    def result(self, value, code=0, stderr=b""):
        raw = json.dumps(value).encode() if not isinstance(value, bytes) else value
        return subprocess.CompletedProcess(["fixture"], code, raw, stderr)

    def exercise(self, selected=None, root=None):
        with mock.patch.object(
            guard.subprocess,
            "run",
            side_effect=[
                self.result(self.selected if selected is None else selected),
                self.result(self.root if root is None else root),
            ],
        ) as child:
            self.assertTrue(guard.check(self.repo, self.now))
            self.assertEqual(child.call_count, 2)
            first, second = child.call_args_list
            self.assertEqual(
                first.args[0],
                ["go", "list", "-mod=readonly", "-m", "-json", guard.MODULE],
            )
            self.assertEqual(second.args[0], ["go", "mod", "edit", "-json"])
            for call in child.call_args_list:
                self.assertEqual(call.kwargs["cwd"], self.repo)
                self.assertTrue(call.kwargs["capture_output"])
                self.assertNotIn("text", call.kwargs)
                self.assertEqual(call.kwargs["timeout"], 60)
                self.assertEqual(call.kwargs["env"]["GOWORK"], "off")
                self.assertEqual(call.kwargs["env"]["GOFLAGS"], "")
                self.assertEqual(call.kwargs["env"]["GOPROXY"], "off")

    def test_exact_selected_version_and_indirect_root_pass(self):
        self.exercise()

    def test_unknown_producer_fields_do_not_change_selection(self):
        self.exercise(dict(self.selected, Future={"opaque": 123}))

    def test_other_selected_versions_fail(self):
        for version in ("v0.59.0", "v0.61.0", "", None):
            with self.subTest(version=version), self.assertRaises(RuntimeError):
                self.exercise(dict(self.selected, Version=version))

    def test_missing_or_wrong_module_fails(self):
        for value in ({}, dict(self.selected, Path="other/module")):
            with self.subTest(value=value), self.assertRaises(RuntimeError):
                self.exercise(value)

    def test_direct_or_replaced_or_error_selection_fails(self):
        for field, value in (
            ("Indirect", False),
            ("Indirect", 1),
            ("Indirect", "true"),
            ("Replace", {"Path": "local"}),
            ("Replace", None),
            ("Error", {"Err": "failure"}),
        ):
            with (
                self.subTest(field=field, value=value),
                self.assertRaises(RuntimeError),
            ):
                self.exercise(dict(self.selected, **{field: value}))

    def test_root_requirement_must_be_unique_versioned_and_indirect(self):
        for requirements in (
            [],
            [self.selected, self.selected],
            [dict(self.selected, Version="v0.61.0")],
            [dict(self.selected, Indirect=False)],
            [dict(self.selected, Indirect=1)],
            ["invalid"],
            None,
        ):
            with (
                self.subTest(requirements=requirements),
                self.assertRaises(RuntimeError),
            ):
                self.exercise(root={"Require": requirements})

    def test_invalid_json_and_utf8_fail(self):
        for raw in (
            b"",
            b"garbage",
            b"[]",
            b"null",
            b"{} {}",
            b'{"Version":"v0.60.0","Version":"v0.61.0"}',
            b"\xff",
        ):
            with self.subTest(raw=raw), self.assertRaises(RuntimeError):
                self.exercise(raw)

    def test_execution_and_stderr_failures_are_not_passes(self):
        for outcome in (
            self.result(self.selected, code=1),
            self.result(self.selected, stderr=b"\xff"),
            OSError("missing go"),
            subprocess.TimeoutExpired(["go"], 60),
        ):
            with (
                self.subTest(outcome=outcome),
                mock.patch.object(guard.subprocess, "run", side_effect=[outcome]),
                self.assertRaises(RuntimeError),
            ):
                guard.check(self.repo, self.now)

    def test_root_command_failure_fails(self):
        with (
            mock.patch.object(
                guard.subprocess,
                "run",
                side_effect=[self.result(self.selected), self.result({}, code=1)],
            ),
            self.assertRaises(RuntimeError),
        ):
            guard.check(self.repo, self.now)

    def test_expiry_boundary_matches_strict_after_checker(self):
        self.now = guard.EXPIRES
        self.exercise()
        with mock.patch.object(guard.subprocess, "run") as child:
            self.assertFalse(
                guard.check(self.repo, guard.EXPIRES + timedelta(microseconds=1))
            )
            child.assert_not_called()

    def test_naive_time_is_not_an_expiry_bypass(self):
        with self.assertRaises(RuntimeError):
            guard.check(self.repo, guard.EXPIRES.replace(tzinfo=None))

    def test_cli_failure_is_nonzero(self):
        output = io.StringIO()
        with (
            mock.patch.object(guard, "check", side_effect=RuntimeError("blocked")),
            contextlib.redirect_stderr(output),
        ):
            self.assertEqual(guard.main(), 1)
        self.assertIn("check failed", output.getvalue())

    def test_required_ci_and_make_test_wiring(self):
        repo = Path(__file__).resolve().parent.parent
        for name in ("ci.yml", "native-contracts.yml"):
            workflow = (repo / ".github" / "workflows" / name).read_text()
            step = (
                "      - name: Verify temporary cooling selection\n"
                "        env:\n"
                "          GOPROXY: 'off'\n"
                "          GOFLAGS: ''\n"
                "        run: python3 scripts/check-cooling-selection.py\n"
            )
            self.assertEqual(workflow.count(step), 1)
            before, after = workflow.split(step)
            self.assertIn("actions/checkout@v4", before)
            if name == "ci.yml":
                self.assertIn("GOTOOLCHAIN: go1.26.9", before)
                self.assertIn("      - name: Build\n        run: make build\n", before)
                self.assertIn("      - name: Upload goneat binary", after)
            else:
                self.assertIn("go-version: '1.26.9'", before)
                self.assertIn("      - name: Build and package", before)
                self.assertIn("        run: make release-build\n", before)
                self.assertIn("      - name: Record producer provenance", after)
            # The network restriction belongs only to this step, not to builds.
            self.assertNotIn("GOPROXY:", before)
        makefile = (repo / "Makefile").read_text()
        scripts = makefile.split("test-scripts:", 1)[1].split(".PHONY:", 1)[0]
        self.assertIn(
            "\t@GOPROXY=off GOFLAGS= python3 scripts/test_cooling_selection.py\n",
            scripts,
        )


if __name__ == "__main__":
    unittest.main(verbosity=2)

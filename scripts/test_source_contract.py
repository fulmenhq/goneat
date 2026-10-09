"""Negative controls for required native source-SBOM test receipts."""

import importlib.util
import json
from pathlib import Path
import sys
import unittest

sys.dont_write_bytecode = True
spec = importlib.util.spec_from_file_location(
    "source_contract", Path(__file__).with_name("source-contract.py")
)
contract = importlib.util.module_from_spec(spec)
spec.loader.exec_module(contract)


class SourceContractChecks(unittest.TestCase):
    target = "windows/arm64"

    def events(self):
        identity = {"os": "windows", "arch": "arm64", "compiler": "go1.26.9"}
        events = [
            {
                "Package": contract.PACKAGE,
                "Action": "output",
                "Test": "TestSourceRuntimeIdentity",
                "Output": "SOURCE_TEST_PROCESS=" + json.dumps(identity),
            }
        ]
        events.extend(
            {"Package": contract.PACKAGE, "Action": "pass", "Test": name}
            for name in sorted(contract.required_tests(self.target))
        )
        events.append({"Package": contract.PACKAGE, "Action": "pass"})
        return events

    def validate(self, events, code=0):
        return contract.validate_events(
            "\n".join(json.dumps(event) for event in events), self.target, code
        )

    def test_required_children_cannot_hide_under_parent_pass(self):
        for child in [
            "TestSourcePublicationCleanupBeforeOutput/cleanup-failure",
            "TestSourceOrderedIgnoreNegations/dead-child",
            "TestSourceRootGoEvidenceProtection/broad-configured-conflict",
            "TestSourceGoWorkspaceScope/external-workspace",
            "TestSourceProvenanceRecordsRootGoProtection/spdx-json",
            "TestSourceCollectorRefusesMutation",
        ]:
            for action in ["skip", "fail", "missing"]:
                with self.subTest(child=child, action=action):
                    events = self.events()
                    if action == "missing":
                        events = [
                            event for event in events if event.get("Test") != child
                        ]
                    else:
                        for event in events:
                            if event.get("Test") == child:
                                event["Action"] = action
                    with self.assertRaisesRegex(RuntimeError, "required source tests"):
                        self.validate(events)

    def test_platform_manifest_requires_acl_and_symlink_proof(self):
        for target in ["windows/amd64", "windows/arm64"]:
            required = contract.required_tests(target)
            for name in [
                "TestSourceCaptureUnreadableACL",
                "TestSourceCapturePrivacyMutation",
                "TestSourceRootHandleIdentitySurvivesPathReuse",
                "TestSourceCaptureOwnerMutation",
                "TestSourceCaptureOwnerMutation/.",
                "TestSourceCaptureOwnerMutation/nested",
                "TestSourceCaptureOwnerMutation/nested/file",
                "TestSourceChildOwnerRejectsClosedHandle",
                "TestSourceChildDirectoryOwnerIdentity",
                "TestSourceChildOwnerErrorStages",
                "TestSourceReplacementDenialClassification/generic-error",
                "TestSourceCaptureRejectsSymlinksEvenExcluded",
            ]:
                self.assertIn(name, required)
            self.assertNotIn("TestSourceCaptureErrorsCleanup/unreadable-file", required)
            self.assertNotIn("TestSourceInvokerFailurePreservesDestination", required)

    def test_process_identity_and_complete_stream_are_required(self):
        events = self.events()
        self.validate(events)
        for invalid in [events[:-1], events[1:]]:
            with self.assertRaises(RuntimeError):
                self.validate(invalid)
        events[0]["Output"] = events[0]["Output"].replace("arm64", "amd64")
        with self.assertRaisesRegex(RuntimeError, "identity mismatch"):
            self.validate(events)
        with self.assertRaisesRegex(RuntimeError, "process failed"):
            self.validate(self.events(), 1)
        with self.assertRaisesRegex(RuntimeError, "invalid go test JSON"):
            contract.validate_events('{"Action":', self.target, 0)


if __name__ == "__main__":
    unittest.main()

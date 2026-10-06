"""Require native source-SBOM helper tests; actual collector proof is separate."""

import argparse
import json
import os
from pathlib import Path
import re
import subprocess
import sys

PACKAGE = "github.com/fulmenhq/goneat/pkg/sbom"
TARGETS = {
    "linux/amd64",
    "linux/arm64",
    "darwin/arm64",
    "windows/amd64",
    "windows/arm64",
}


def required_tests(target):
    if target not in TARGETS:
        raise RuntimeError(f"unsupported native source contract target: {target}")
    names = {
        "TestSourceRuntimeIdentity",
        "TestSourceArtifactIdentity",
        "TestSourceArtifactRejectsDirectory",
        "TestSourceCapturePrivateComplete",
        "TestSourceCaptureErrorsCleanup",
        "TestSourceCaptureRejectsSymlinksEvenExcluded",
        "TestSourceCaptureBoundariesAndCancellation",
        "TestSourceCapturePolicyPreflight",
        "TestSourceCapturePrivacyMutation",
        "TestSourceCleanupRejectsReplacedRoot",
        "TestSourceRootHandleIdentitySurvivesPathReuse",
        "TestSourceManifestIdentityReconciliation",
        "TestSourceCaptureDetectsMutationDuringHandleRead",
        "TestSourceArgumentBudgets",
        "TestSourcePatternGrammar",
        "TestSourceSelectionNamedOnly",
        "TestSourceIgnoreDiagnostics",
        "TestSourceLiteralExcludes",
        "TestSourcePublicationCleanupBeforeOutput",
        "TestSourcePublicationStdoutCleanupFailure",
        "TestSourcePublicationCancelledStillCleans",
        "TestSourceJSONPreservesUnknownValues",
        "TestSourceJSONRejectsAmbiguousDocuments",
        "TestSourceSchemasOffline",
        "TestSourceSchemasRejectUnbundledReference",
        "TestSourceProvenancePreservesInventory",
        "TestSourceProvenanceRejectsResidueAndInvalidReferences",
        "TestSourceProvenancePreservesNumericEvidence",
        "TestSourceProvenanceResidualEncodedPaths",
        "TestSourceReferencesEvidenceAndAnnotations",
        "TestSourceReferencesRejectsUnresolvedEvidenceTool",
    }
    children = {
        "TestSourcePatternGrammar": [
            "bin/**/bin/stale",
            "bin/**/nested/bin/stale",
            "**/bin/**/nested/bin/stale",
            "/bin/**/bin/stale",
            "/bin/**/nested/bin/stale",
            "bin//nested/bin/stale",
            "bin//bin",
            "bin//bin#01",
            "/bin//nested/bin/stale",
            "*.tmp/a/thing.tmp",
            "/thing.tmp/a/thing.tmp",
            "a/?.[ch]/a/x.c",
            "a/?.[ch]/a/x.go",
            r"\#artifact/a/#artifact",
            r"\!artifact/!artifact",
            r"a/\*.tmp/a/*.tmp",
            r"a/\*.tmp/a/x.tmp",
            r"a/name\_/a/name_",
            r"\_name/_name",
        ],
        "TestSourceSelectionNamedOnly": [
            "default",
            "named",
            "directory",
            "no-ignore",
            "no-ignore-named",
        ],
        "TestSourceArtifactIdentity": [
            "unchanged",
            "content",
            "replacement",
            "parent-replacement",
            "deleted",
        ],
        "TestSourceCaptureErrorsCleanup": [
            "entry-limit",
            "byte-limit",
            "bad-policy",
            "missing-force",
        ],
        "TestSourceCaptureDetectsMutationDuringHandleRead": [
            "replace=false",
            "replace=true",
        ],
        "TestSourcePublicationCleanupBeforeOutput": ["success", "cleanup-failure"],
        "TestSourceProvenancePreservesInventory": ["cyclonedx-json", "spdx-json"],
        "TestSourceProvenanceRejectsResidueAndInvalidReferences": [
            "unknown-location",
            "prefix-lookalike",
            "opaque-id",
            "dangling",
            "duplicate-id",
        ],
    }
    children["TestSourceReferencesEvidenceAndAnnotations"] = [
        f"{version}/{reference}"
        for version in ["1.6", "1.7"]
        for reference in [
            "pkg-1",
            "missing",
            "urn:cdx:00000000-0000-0000-0000-000000000001/1#external-component",
        ]
    ]
    if target.startswith("windows/"):
        names.add("TestSourceCaptureUnreadableACL")
        names.add("TestSourceCaptureOwnerMutation")
        children["TestSourceCaptureOwnerMutation"] = [".", "nested", "nested/file"]
        names.add("TestSourceChildOwnerRejectsClosedHandle")
        names.add("TestSourceChildDirectoryOwnerIdentity")
        names.add("TestSourceReplacementDenialClassification")
        children["TestSourceReplacementDenialClassification"] = [
            "access-denied",
            "sharing-violation",
            "wrapped-denial",
            "generic-permission",
            "missing-file",
            "generic-error",
        ]
    else:
        names.add("TestSourceCaptureRejectsSpecialFile")
        names.add("TestSourceInvokerFailurePreservesDestination")
        children["TestSourceCaptureErrorsCleanup"].append("unreadable-file")
        children["TestSourceInvokerFailurePreservesDestination"] = [
            "collector-error",
            "malformed-json",
            "duplicate-json-key",
            "invalid-schema",
            "dangling-reference",
            "snapshot-mutation",
            "snapshot-path-replaced",
        ]
    for parent, cases in children.items():
        names.update(f"{parent}/{case}" for case in cases)
    return names


def validate_events(text, target, returncode):
    if returncode != 0:
        raise RuntimeError(f"source contract test process failed: {returncode}")
    required = required_tests(target)
    terminal = {}
    package_actions = []
    identities = []
    for line in text.splitlines():
        try:
            event = json.loads(line)
        except (ValueError, TypeError) as error:
            raise RuntimeError("incomplete or invalid go test JSON") from error
        if not isinstance(event, dict) or event.get("Package") != PACKAGE:
            raise RuntimeError("unexpected go test JSON event/package")
        action, test = event.get("Action"), event.get("Test")
        if action in {"pass", "skip", "fail"}:
            if test:
                terminal.setdefault(test, []).append(action)
            else:
                package_actions.append(action)
        output = event.get("Output", "")
        if test == "TestSourceRuntimeIdentity" and "SOURCE_TEST_PROCESS=" in output:
            marker = output.split("SOURCE_TEST_PROCESS=", 1)[1].strip()
            try:
                identities.append(json.loads(marker))
            except ValueError as error:
                raise RuntimeError("invalid test-process identity") from error
    missing = sorted(name for name in required if terminal.get(name) != ["pass"])
    if missing:
        raise RuntimeError(f"required source tests missing/skipped/failed: {missing}")
    allowed_skips = (
        {"TestSourceCaptureErrorsCleanup/unreadable-file"}
        if target.startswith("windows/")
        else set()
    )
    unexpected = sorted(
        name
        for name, actions in terminal.items()
        if actions != ["pass"] and not (name in allowed_skips and actions == ["skip"])
    )
    if unexpected:
        raise RuntimeError(f"unexpected skipped/failed source tests: {unexpected}")
    if package_actions != ["pass"]:
        raise RuntimeError("missing or unsuccessful package completion event")
    expected_os, expected_arch = target.split("/")
    expected = {"os": expected_os, "arch": expected_arch, "compiler": "go1.26.6"}
    if identities != [expected]:
        raise RuntimeError(
            f"actual test-process identity mismatch: {identities}; expected {expected}"
        )
    return {
        "process": expected,
        "required_passed": sorted(required),
        "skipped": sorted(
            name for name, actions in terminal.items() if "skip" in actions
        ),
    }


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--target", required=True, choices=sorted(TARGETS))
    parser.add_argument("--source-sha")
    parser.add_argument("--receipt", type=Path)
    args = parser.parse_args()
    root = Path(__file__).resolve().parent.parent
    sha = subprocess.check_output(
        ["git", "rev-parse", "HEAD"], cwd=root, text=True
    ).strip()
    dirty = bool(
        subprocess.check_output(
            ["git", "status", "--porcelain"], cwd=root, text=True
        ).strip()
    )
    if args.source_sha and (sha != args.source_sha or dirty):
        raise RuntimeError(
            "native source contracts require the exact clean candidate source SHA"
        )
    roots = sorted({name.split("/", 1)[0] for name in required_tests(args.target)})
    pattern = "^(" + "|".join(re.escape(name) for name in roots) + ")$"
    environment = dict(
        os.environ, GOTOOLCHAIN="go1.26.6", GONEAT_OFFLINE_SCHEMA_VALIDATION="true"
    )
    command = ["go", "test", "-json", "-count=1", "-run", pattern, "./pkg/sbom"]
    completed = subprocess.run(
        command, cwd=root, env=environment, text=True, capture_output=True, timeout=600
    )
    sys.stdout.write(completed.stdout)
    sys.stderr.write(completed.stderr)
    receipt = validate_events(completed.stdout, args.target, completed.returncode)
    receipt.update(
        {
            "source_sha": sha,
            "dirty": dirty,
            "target": args.target,
            "command": command,
            "proof": "native source helper contracts; actual collector evidence is separate",
        }
    )
    if args.target.startswith("windows/"):
        receipt["integration_gap"] = (
            "POSIX collector failure injector is not Windows collector-process proof"
        )
    encoded = json.dumps(receipt, indent=2) + "\n"
    if args.receipt:
        args.receipt.parent.mkdir(parents=True, exist_ok=True)
        args.receipt.write_text(encoded, encoding="utf-8")
    print(encoded)


if __name__ == "__main__":
    try:
        main()
    except (RuntimeError, subprocess.SubprocessError) as error:
        print(f"source contract failed: {error}", file=sys.stderr)
        sys.exit(1)

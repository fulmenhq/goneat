"""Enforce the selected x/net version during the temporary cooling exception."""

from datetime import datetime, timezone
import json
import os
from pathlib import Path
import subprocess
import sys

MODULE = "golang.org/x/net"
VERSION = "v0.60.0"
EXPIRES = datetime(2026, 10, 16, tzinfo=timezone.utc)


def unique_object(pairs):
    result = {}
    for key, value in pairs:
        if key in result:
            raise RuntimeError("duplicate JSON field in Go evidence")
        result[key] = value
    return result


def go_json(repo, args):
    env = dict(os.environ, GOWORK="off", GOFLAGS="", GOPROXY="off")
    try:
        result = subprocess.run(
            ["go", *args], cwd=repo, env=env, capture_output=True, timeout=60
        )
    except (OSError, subprocess.TimeoutExpired) as error:
        raise RuntimeError("Go evidence command failed") from error
    if result.returncode != 0:
        raise RuntimeError("Go evidence command returned nonzero")
    try:
        result.stderr.decode("utf-8", errors="strict")
        value = json.loads(
            result.stdout.decode("utf-8", errors="strict"),
            object_pairs_hook=unique_object,
        )
    except (UnicodeError, ValueError) as error:
        raise RuntimeError("invalid Go JSON evidence") from error
    if not isinstance(value, dict):
        raise RuntimeError("Go evidence must be one JSON object")
    return value


def check(repo, now):
    if now.tzinfo is None or now.utcoffset() is None:
        raise RuntimeError("UTC-aware time required")
    # The cooling checker expires only strictly after this UTC instant.
    if now > EXPIRES:
        return False
    selected = go_json(repo, ["list", "-mod=readonly", "-m", "-json", MODULE])
    if (
        selected.get("Path") != MODULE
        or selected.get("Version") != VERSION
        or selected.get("Indirect") is not True
        or "Replace" in selected
        or "Error" in selected
    ):
        raise RuntimeError(
            "cooling exception requires unreplaced indirect x/net v0.60.0"
        )
    root = go_json(repo, ["mod", "edit", "-json"])
    requirements = root.get("Require")
    if not isinstance(requirements, list) or not all(
        isinstance(row, dict) for row in requirements
    ):
        raise RuntimeError("invalid root requirement evidence")
    matches = [row for row in requirements if row.get("Path") == MODULE]
    if (
        len(matches) != 1
        or matches[0].get("Version") != VERSION
        or matches[0].get("Indirect") is not True
    ):
        raise RuntimeError("root x/net requirement must remain v0.60.0 and indirect")
    return True


def main():
    try:
        active = check(
            Path(__file__).resolve().parent.parent, datetime.now(timezone.utc)
        )
    except RuntimeError as error:
        print(f"Cooling selection check failed: {error}", file=sys.stderr)
        return 1
    if active:
        print("Cooling selection verified: indirect golang.org/x/net v0.60.0")
    else:
        print("Temporary cooling selection restriction expired at 2026-10-16T00:00:00Z")
    return 0


if __name__ == "__main__":
    sys.exit(main())

#!/usr/bin/env python3
"""Check the conditions the push gate reads before it scans the tree.

The dates check needs the full commit history. The tools check needs
shellcheck and yamllint at the minimum versions in .goneat/tools.yaml.
"""

import re
import subprocess
from pathlib import Path


def repo_root() -> Path:
    out = subprocess.check_output(["git", "rev-parse", "--show-toplevel"], text=True)
    return Path(out.strip())


def tool_minimum(text: str, name: str) -> str:
    match = re.search(
        rf"\n  {re.escape(name)}:\n(.*?)(\n  [A-Za-z0-9_-]+:\n|\Z)",
        text,
        re.S,
    )
    if not match:
        raise SystemExit(f"missing {name} in .goneat/tools.yaml")
    version = re.search(r'minimum_version:\s*"([^"]+)"', match.group(1))
    if not version:
        raise SystemExit(f"missing minimum_version for {name}")
    return version.group(1)


def parts(value: str) -> tuple[int, ...]:
    return tuple(int(piece) for piece in value.strip().lstrip("v").split("."))


def require_at_least(command: list[str], pattern: str, minimum: str) -> None:
    try:
        output = subprocess.check_output(command, text=True, stderr=subprocess.STDOUT)
    except (OSError, subprocess.CalledProcessError) as err:
        raise SystemExit(f"{command[0]} is not available: {err}") from err
    found = re.search(pattern, output, re.M)
    if not found:
        raise SystemExit(f"could not read {command[0]} version from:\n{output}")
    current = found.group(1)
    if parts(current) < parts(minimum):
        raise SystemExit(
            f"{command[0]} {current} is older than the foundation minimum {minimum}"
        )


def main() -> None:
    root = repo_root()
    shallow = subprocess.check_output(
        ["git", "rev-parse", "--is-shallow-repository"],
        cwd=root,
        text=True,
    ).strip()
    if shallow == "true":
        raise SystemExit(
            "Impossible-chronology check needs the full commit history (repository is shallow)"
        )
    tools = (root / ".goneat" / "tools.yaml").read_text()
    require_at_least(
        ["yamllint", "--version"],
        r"yamllint\s+(\d+\.\d+\.\d+)",
        tool_minimum(tools, "yamllint"),
    )
    require_at_least(
        ["shellcheck", "--version"],
        r"^version:\s+(\d+\.\d+\.\d+)",
        tool_minimum(tools, "shellcheck"),
    )


if __name__ == "__main__":
    main()

# `goneat guardian`

> **⚠️ EXPERIMENTAL**: This feature is experimental until v0.3.x. It may have breaking changes, incomplete documentation, or unexpected behavior. Use in production repositories at your own risk.

Guardian enforces high-risk repository operations with explicit human approval. It evaluates configured policies for each scope/operation pair (for example `git.push`) and, when required, launches a secure local approval server before the command proceeds.

## `goneat guardian check`

```bash
goneat guardian check git push --branch main --remote origin
```

- Returns exit code `0` when no approval is required.
- Returns a non-zero exit code with context when approval is required (hooks treat this as a block).
- Supports optional context flags:
  - `--branch`: active branch name
  - `--remote`: remote name or URL
  - `--user`: actor requesting the operation

## `goneat guardian approve`

```bash
# Wrap the operation (similar to sudo) so it executes only after approval
goneat guardian approve git push -- git push origin main
```

- The command after `--` is executed automatically once approval succeeds.
- Guardian starts a local browser approval flow, displaying the URL and project branding in the terminal.
- Approval sessions expire automatically based on policy duration (shorter of policy `expires` and `browser_approval.timeout_seconds`).
- If the session expires before approval, the command fails with `guardian approval expired`.
- Use `--branch`, `--remote`, `--user`, or `--reason` to provide additional context shown to reviewers.
- Successful approval issues a single-use guardian grant (stored in `~/.goneat/guardian/grants/`) that hooks consume automatically. Re-running the command after the grant is consumed requires a new approval.

## `goneat guardian setup`

```bash
goneat guardian setup
```

- Ensures the guardian configuration file exists (usually `~/.goneat/guardian/config.yaml`).
- Initializes default policy scopes, branding placeholders, and security settings.

## Configuration Scope

Guardian policies are **user-level only** and apply globally across all repositories on the machine. There is currently no support for repository-specific guardian policies.

- **Configuration Location**: `~/.goneat/guardian/config.yaml` (user home directory)
- **Scope**: Policies apply to all repositories where goneat is used
- **Repository Control**: Repository maintainers can configure git hooks (`.goneat/hooks.yaml`) to call guardian for specific operations, but cannot define custom guardian policies for their repository

This design ensures consistent security policies across all repositories while allowing per-repository hook customization for assessment workflows.

## Commands Pending Implementation

The following subcommands are planned but currently return informative errors:

- `goneat guardian grant` — grant-token workflows
- `goneat guardian status` — active grant/status inspection

## Hooks Integration Workflow

When hooks include guardian checks, a blocked operation presents instructions similar to:

```text
❌ Operation blocked by guardian
🔐 Approval required for: git push

Wrap your git push with guardian approval to proceed:
  goneat guardian approve git push -- git push origin main
Once approved, the push runs automatically under guardian supervision.
```

Re-run the original git command through `guardian approve` to resume the workflow after approval completes.

## Browser Approval Experience

- Local server binds to `127.0.0.1` on a random high port and uses cryptographic nonces.
- Project name and optional custom message from `guardian.security.branding` appear prominently in the approval page (`<h1>` header) and terminal instructions.
- The terminal can optionally hide the URL display when `browser_approval.show_url_in_terminal` is disabled.

## Troubleshooting

- **Approval expired**: restart the command with `guardian approve`; approvals time out automatically.
- **Browser did not open**: copy the URL printed in the terminal and open it manually (auto-open respects `GONEAT_GUARDIAN_AUTO_OPEN` and config settings).
- **Hooks blocked unexpectedly**: run `goneat guardian check <scope> <operation> --branch <branch>` to inspect the policy result and confirm branch/remote matching.

## Related Resources

- [Hooks command guide](hooks.md)
- Guardian configuration defaults: `~/.goneat/guardian/config.yaml`

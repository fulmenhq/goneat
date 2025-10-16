# Crucible SSOT Documentation

> **⚠️ DO NOT EDIT FILES IN THIS DIRECTORY MANUALLY**

This directory contains **Single Source of Truth (SSOT)** documentation synced from the [Crucible](https://github.com/fulmenhq/crucible) repository. Files here are **read-only** in language-specific repositories and are automatically synchronized during releases.

## 🚫 What NOT to Do

- ❌ **Do not edit** any files in this directory
- ❌ **Do not format** files in this directory with local tools
- ❌ **Do not create** new files in this directory
- ❌ **Do not delete** files from this directory
- ❌ **Do not commit** changes to synced files

**Any manual changes will be overwritten** the next time `make sync` runs.

## ✅ What to Do Instead

### If you need to update documentation in this directory:

1. **Propose changes to the Crucible team** via approved messaging channels (Slack, GitHub Discussions, or issues)
2. **Submit a pull request to Crucible** at https://github.com/fulmenhq/crucible
3. **Wait for the next sync** - changes will automatically propagate to all language repositories

### If you need library-specific documentation:

Create files in the **appropriate local directories**:

- **Go**: Use `docs/development/` for gofulmen-specific docs
- **Python**: Use `docs/development/` for pyfulmen-specific docs
- **TypeScript**: Use `docs/development/` for tsfulmen-specific docs

See the [Fulmen Helper Library Standard](architecture/fulmen-helper-library-standard.md) for guidance on local vs. ecosystem documentation.

## 📁 Directory Structure

```
docs/
├── README.md                    ⚠️ This file (synced from Crucible)
├── architecture/                ⚠️ Ecosystem architecture (synced)
│   ├── decisions/              ⚠️ Ecosystem ADRs (synced)
│   └── ...
├── standards/                   ⚠️ Cross-language standards (synced)
│   ├── coding/                 ⚠️ Language-specific coding standards (synced)
│   ├── library/                ⚠️ Library module standards (synced)
│   └── ...
└── guides/                      ⚠️ Integration guides (synced)
```

## 🔄 How Syncing Works

The Crucible repository maintains authoritative versions of:

- **Documentation** (`docs/`) - Architecture, standards, guides
- **Schemas** (`schemas/`) - JSON Schema definitions
- **Configuration** (`config/`) - Default configs, catalogs, taxonomy

During releases or when maintainers run `make sync`, these assets are copied to language-specific repositories:

```bash
# In language repos (gofulmen, pyfulmen, tsfulmen):
crucible/docs/       → lang/<language>/docs/
crucible/schemas/    → lang/<language>/schemas/
crucible/config/     → lang/<language>/config/
```

This ensures **consistency across all Fulmen helper libraries**.

## 📋 ADR System (Two-Tier)

This directory contains **Tier 1: Ecosystem ADRs** that apply across all languages:

- **Location**: `docs/architecture/decisions/ADR-XXXX-*.md`
- **Scope**: Cross-language architectural decisions
- **Sync**: Automatically synced to all language repositories
- **Changes**: Must be proposed in Crucible repository

For **Tier 2: Local ADRs** specific to a single library:

- **Location**: `docs/development/adr/ADR-XXXX-*.md` (not in this directory!)
- **Scope**: Library-specific decisions
- **Sync**: Not synced; maintained independently
- **Changes**: Can be made directly in the language repository

See [ADR-0001: Two-Tier ADR System](architecture/decisions/ADR-0001-two-tier-adr-system.md) for details.

## 🛠️ Developer Workflow

### Viewing Documentation

```bash
# Browse synced documentation (read-only)
cd docs/
ls architecture/decisions/    # View ecosystem ADRs
cat standards/coding/go.md     # Read Go coding standards
```

### Proposing Changes

1. **Identify the change**: What needs to be updated?
2. **Contact Crucible maintainers**: Slack, GitHub Discussions, or open an issue
3. **Submit PR to Crucible**: https://github.com/fulmenhq/crucible/pulls
4. **Wait for review and merge**: Crucible team reviews and approves
5. **Sync propagates**: Next release or sync will update all language repos

### Common Mistakes to Avoid

❌ **Mistake**: "I fixed a typo in `docs/standards/coding/python.md` in pyfulmen"
✅ **Solution**: Revert local change, submit PR to Crucible instead

❌ **Mistake**: "I ran Prettier on `docs/` and it reformatted everything"
✅ **Solution**: Revert changes, configure your formatter to exclude synced directories

❌ **Mistake**: "I added a new ADR in `docs/architecture/decisions/`"
✅ **Solution**: Move to `docs/development/adr/` for local ADRs, or propose ecosystem ADR to Crucible

## 📚 Related Documentation

- [Fulmen Ecosystem Guide](architecture/fulmen-ecosystem-guide.md) - Overview of the ecosystem
- [Sync Model](architecture/sync-model.md) - How SSOT syncing works
- [Sync Consumers Guide](guides/sync-consumers-guide.md) - Consuming synced assets in language repos
- [ADR-0001: Two-Tier ADR System](architecture/decisions/ADR-0001-two-tier-adr-system.md) - ADR structure

## 🔗 Quick Links

- **Crucible Repository**: https://github.com/fulmenhq/crucible
- **Report Issues**: https://github.com/fulmenhq/crucible/issues
- **Standards**: [docs/standards/](standards/)
- **Architecture**: [docs/architecture/](architecture/)

## ℹ️ Questions?

Contact the Fulmen maintainers via approved channels:

- **Crucible Issues**: https://github.com/fulmenhq/crucible/issues
- **Team Communication**: Check with your library maintainer for Slack/Discord details

---

**Remember**: This documentation is **synced from Crucible**. Propose changes upstream, not here.

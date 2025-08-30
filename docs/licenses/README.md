# License Inventory

This directory contains the license inventory for Goneat and all its dependencies.

## Overview

Goneat is committed to license compliance and transparency. This inventory tracks all licenses used by Goneat and its dependencies.

## Structure

- `inventory.md` - Complete license inventory with classifications
- `third-party/` - License texts for third-party dependencies
- `scripts/` - Scripts for generating and updating the inventory

## License Classifications

### Compatible Licenses ✅

These licenses are compatible with Goneat's Apache 2.0 license:

- **Apache License 2.0** - Full compatibility
- **MIT License** - Full compatibility
- **BSD 2-Clause** - Full compatibility
- **BSD 3-Clause** - Full compatibility
- **ISC License** - Full compatibility

### Restricted Licenses ⚠️

These licenses require additional consideration:

- **GPL 2.0/3.0** - Copyleft, may contaminate Apache 2.0
- **LGPL 2.1/3.0** - Lesser copyleft, may contaminate
- **CDDL** - Copyleft, may contaminate
- **MPL 2.0** - Copyleft, may contaminate

### Forbidden Licenses ❌

These licenses are incompatible and cannot be used:

- **GPL 1.0** - Outdated, incompatible
- **Proprietary** - Not open source

## Generation

The license inventory is generated using:

```bash
# Generate full inventory
make license-inventory

# Update third-party licenses
make update-licenses
```

## Compliance

- All dependencies are reviewed for license compatibility
- License texts are preserved and included in distributions
- Attribution requirements are met
- No viral licenses are used in core functionality

## Contact

For license questions, contact the maintainers or create an issue.

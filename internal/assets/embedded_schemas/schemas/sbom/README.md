# SBOM validation schemas

These upstream files are retained verbatim for offline validation. Edit neither
these schemas nor their embedded copies when changing goneat's mapping logic.

## CycloneDX

Source: [CycloneDX specification, tag 1.7](https://github.com/CycloneDX/specification/tree/1.7/schema).

- `bom-1.6.schema.json`
- `bom-1.7.schema.json`
- `spdx.schema.json` (CycloneDX's SPDX license identifiers)
- `jsf-0.82.schema.json`
- `cryptography-defs.schema.json`

Copyright and license: see `CycloneDX-LICENSE.txt` (Apache-2.0), from the
[upstream license](https://github.com/CycloneDX/specification/blob/1.7/LICENSE).

## SPDX

`spdx-2.3.schema.json` comes from
[SPDX specification, tag v2.3](https://github.com/spdx/spdx-spec/blob/v2.3/schemas/spdx-schema.json).
Copyright and license: see `SPDX-LICENSE.txt` (CC-BY-3.0), from the
[upstream license](https://github.com/spdx/spdx-spec/blob/v2.3/LICENSE).

The schema dependency closure is bundled. SBOM validation must resolve these
references locally and must not fetch schemas from the network.

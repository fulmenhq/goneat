# Goneat License Inventory

Generated on: 2025-08-27
Goneat Version: v0.1.0

## Summary

- **Total Dependencies**: 18
- **Compatible Licenses**: 18 ✅
- **Restricted Licenses**: 0 ⚠️
- **Forbidden Licenses**: 0 ❌

## Compatible Licenses ✅

All dependencies use licenses compatible with Goneat's Apache 2.0 license.

### Apache License 2.0 (6 dependencies)

| Package                            | License URL                                                                                                   |
| ---------------------------------- | ------------------------------------------------------------------------------------------------------------- |
| github.com/fulmenhq/goneat         | [LICENSE](https://github.com/fulmenhq/goneat/blob/HEAD/LICENSE)                                               |
| github.com/spf13/afero             | [LICENSE.txt](https://github.com/spf13/afero/blob/v1.12.0/LICENSE.txt)                                        |
| github.com/spf13/cobra             | [LICENSE.txt](https://github.com/spf13/cobra/blob/v1.9.1/LICENSE.txt)                                         |
| github.com/xeipuuv/gojsonpointer   | [LICENSE-APACHE-2.0.txt](https://github.com/xeipuuv/gojsonpointer/blob/4e3ac2762d5f/LICENSE-APACHE-2.0.txt)   |
| github.com/xeipuuv/gojsonreference | [LICENSE-APACHE-2.0.txt](https://github.com/xeipuuv/gojsonreference/blob/bd5ef7bd5415/LICENSE-APACHE-2.0.txt) |
| github.com/xeipuuv/gojsonschema    | [LICENSE-APACHE-2.0.txt](https://github.com/xeipuuv/gojsonschema/blob/v1.2.0/LICENSE-APACHE-2.0.txt)          |

### MIT License (8 dependencies)

| Package                             | License URL                                                              |
| ----------------------------------- | ------------------------------------------------------------------------ |
| github.com/go-viper/mapstructure/v2 | [LICENSE](https://github.com/go-viper/mapstructure/blob/v2.2.1/LICENSE)  |
| github.com/pelletier/go-toml/v2     | [LICENSE](https://github.com/pelletier/go-toml/blob/v2.2.3/LICENSE)      |
| github.com/sagikazarmark/locafero   | [LICENSE](https://github.com/sagikazarmark/locafero/blob/v0.7.0/LICENSE) |
| github.com/sourcegraph/conc         | [LICENSE](https://github.com/sourcegraph/conc/blob/v0.3.0/LICENSE)       |
| github.com/spf13/cast               | [LICENSE](https://github.com/spf13/cast/blob/v1.7.1/LICENSE)             |
| github.com/spf13/viper              | [LICENSE](https://github.com/spf13/viper/blob/v1.20.1/LICENSE)           |
| github.com/subosito/gotenv          | [LICENSE](https://github.com/subosito/gotenv/blob/v1.6.0/LICENSE)        |
| gopkg.in/yaml.v3                    | [LICENSE](https://github.com/go-yaml/yaml/blob/v3.0.1/LICENSE)           |

### BSD 3-Clause License (3 dependencies)

| Package                      | License URL                                                         |
| ---------------------------- | ------------------------------------------------------------------- |
| github.com/fsnotify/fsnotify | [LICENSE](https://github.com/fsnotify/fsnotify/blob/v1.8.0/LICENSE) |
| github.com/spf13/pflag       | [LICENSE](https://github.com/spf13/pflag/blob/v1.0.6/LICENSE)       |
| golang.org/x/sys/unix        | [LICENSE](https://cs.opensource.google/go/x/sys/+/v0.29.0:LICENSE)  |
| golang.org/x/text            | [LICENSE](https://cs.opensource.google/go/x/text/+/v0.21.0:LICENSE) |

## Third-Party License Texts

License texts for third-party dependencies are available in the `third-party/` directory.

## Generation

This inventory was generated using:

```bash
go-licenses csv . > docs/licenses/inventory.csv
```

To regenerate:

```bash
cd goneat
go-licenses csv . > docs/licenses/inventory.csv
# Then manually update this markdown file
```

## Compliance Notes

- All dependencies are compatible with Apache 2.0
- No GPL/LGPL/AGPL dependencies (copyleft families) that would contaminate the license
- All license URLs are verified and accessible
- Goneat itself is licensed under Apache 2.0

## Future Dependencies

When adding new dependencies:

1. Check license compatibility with Apache 2.0
2. Add to this inventory
3. Include license text in `third-party/` directory
4. Update this document

## Contact

For license questions, contact the maintainers.

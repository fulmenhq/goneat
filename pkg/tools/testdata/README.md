# Test Fixtures for pkg/tools

This directory contains test fixtures for artifact installation tests.

## Directory Structure

```
testdata/
├── artifacts/         # Test archive files
├── binaries/         # Source binaries for creating archives
└── README.md         # This file
```

## Artifacts

### Valid Archives
- `valid-tool-1.0.0-darwin-amd64.tar.gz`: Small tar.gz archive containing dummy-tool binary
- `valid-tool-1.0.0-windows-amd64.zip`: ZIP archive containing dummy-tool binary
- Both archives contain the same `dummy-tool` executable for testing

### Malicious/Invalid Archives
- `path-traversal.tar.gz`: Malicious archive attempting path traversal (contains `../../../passwd`)
- `corrupted.tar.gz`: Invalid gzip file (not actually compressed)
- `empty-archive.tar.gz`: Valid archive with no binary inside

## Checksums

All checksums are in `checksums.txt` in SHA256 format:
```
15f41bd50c00f8404e339785ecb5073d4789434140b8a9bb6a3b6e8bdb5c260f  valid-tool-1.0.0-darwin-amd64.tar.gz
978a87eb9de2a69174a3115fa51c1aff65eed820ce99a4995620f227a679a1fb  valid-tool-1.0.0-windows-amd64.zip
```

## Dummy Tool Binary

The `binaries/dummy-tool` is a simple bash script that outputs:
```
dummy-tool version 1.0.0
```

This is used to test extraction and execution without requiring real tool binaries.

## Regenerating Fixtures

If you need to regenerate the fixtures:

```bash
cd pkg/tools/testdata

# Create valid tar.gz archive
tar czf artifacts/valid-tool-1.0.0-darwin-amd64.tar.gz -C binaries dummy-tool

# Create valid zip archive
zip -j artifacts/valid-tool-1.0.0-windows-amd64.zip binaries/dummy-tool

# Compute checksums
shasum -a 256 artifacts/valid-tool-1.0.0-*.{tar.gz,zip} > artifacts/checksums.txt

# Create corrupted archive
echo "corrupted gzip data" > artifacts/corrupted.tar.gz

# Create empty archive
tar czf artifacts/empty-archive.tar.gz -T /dev/null
```

## Usage in Tests

These fixtures are used in `pkg/tools/installer_test.go` to test:

1. **SHA256 Verification**: Using known checksums from `checksums.txt`
2. **Archive Extraction**: Valid archives should extract successfully
3. **Security**: Path traversal attempts should be detected and blocked
4. **Error Handling**: Corrupted and empty archives should fail gracefully
5. **Cross-Platform**: Both tar.gz and zip formats

## Security Notes

⚠️ **Path Traversal Archive**: The `path-traversal.tar.gz` file is intentionally malicious and should NEVER be extracted without proper security checks. It's used to verify that the installer correctly blocks path traversal attacks.

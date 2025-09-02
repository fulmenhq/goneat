# Comprehensive File Normalization Integration Tests

This directory contains integration test files demonstrating all EOF finalizer and file normalization features.

## Test Scenarios

### 1. EOF Newline Enforcement

**File**: `missing-eof.txt`

- **Issue**: File doesn't end with newline
- **Test**: `./dist/goneat format --check --finalize-eof missing-eof.txt`
- **Expected**: "needs formatting" (exit code 1)
- **Apply**: `./dist/goneat format --finalize-eof missing-eof.txt`
- **Result**: File gains trailing `\n`

### 2. Multiple Newline Collapse

**File**: `multiple-eof.txt`

- **Issue**: File ends with `\n\n\n` (multiple newlines)
- **Test**: `./dist/goneat format --check --finalize-eof multiple-eof.txt`
- **Expected**: "needs formatting" (exit code 1)
- **Apply**: `./dist/goneat format --finalize-eof multiple-eof.txt`
- **Result**: Multiple newlines collapsed to single `\n`

### 3. Trailing Whitespace Removal

**File**: `trailing-spaces.txt`

- **Issue**: Lines contain trailing spaces/tabs
- **Test**: `./dist/goneat format --check --finalize-trim-trailing-spaces trailing-spaces.txt`
- **Expected**: "needs formatting" (exit code 1)
- **Apply**: `./dist/goneat format --finalize-trim-trailing-spaces trailing-spaces.txt`
- **Result**: All trailing whitespace removed

### 4. UTF-8 BOM Removal

**File**: `with-bom.txt`

- **Issue**: File starts with UTF-8 BOM (`\xef\xbb\xbf`)
- **Test**: `./dist/goneat format --check --finalize-remove-bom with-bom.txt`
- **Expected**: "needs formatting" (exit code 1)
- **Apply**: `./dist/goneat format --finalize-remove-bom with-bom.txt`
- **Result**: BOM bytes removed, clean UTF-8

### 5. Already Formatted (Control)

**File**: `proper-format.txt`

- **Status**: Already properly formatted
- **Test**: `./dist/goneat format --check --finalize-eof --finalize-trim-trailing-spaces proper-format.txt`
- **Expected**: "already formatted" (exit code 0)
- **Apply**: No changes made to file

## Comprehensive Testing

### All Features Combined

```bash
# Test all files with comprehensive normalization
./dist/goneat format --check \
  --finalize-eof \
  --finalize-trim-trailing-spaces \
  --finalize-line-endings=lf \
  --finalize-remove-bom \
  *.txt

# Apply comprehensive formatting
./dist/goneat format \
  --finalize-eof \
  --finalize-trim-trailing-spaces \
  --finalize-line-endings=lf \
  --finalize-remove-bom \
  *.txt
```

### Idempotency Verification

```bash
# Run formatting multiple times - should produce identical results
for i in {1..3}; do
  ./dist/goneat format --finalize-eof --finalize-trim-trailing-spaces *.txt
done
```

## Expected Results Summary

| Feature      | Check Result      | Apply Result      | Idempotent |
| ------------ | ----------------- | ----------------- | ---------- |
| EOF Newline  | needs formatting  | ✅ adds `\n`      | ✅         |
| Multiple EOF | needs formatting  | ✅ collapses to 1 | ✅         |
| Trailing WS  | needs formatting  | ✅ removes spaces | ✅         |
| UTF-8 BOM    | needs formatting  | ✅ removes BOM    | ✅         |
| Already Good | already formatted | ✅ no changes     | ✅         |

## File Contents (Before/After Examples)

### missing-eof.txt

**Before**: `Missing final newline` (no `\n`)
**After**: `Missing final newline\n` (with `\n`)

### multiple-eof.txt

**Before**: `Multiple trailing newlines\n\n\n` (3 newlines)
**After**: `Multiple trailing newlines\n` (1 newline)

### trailing-spaces.txt

**Before**: `Line with trailing spaces   \n` (spaces after "spaces")
**After**: `Line with trailing spaces\n` (spaces removed)

### with-bom.txt

**Before**: `\xef\xbb\xbfUTF-8 with BOM\n` (starts with BOM)
**After**: `UTF-8 with BOM\n` (BOM removed)

## Command Line Reference

```bash
# Individual features
--finalize-eof                    # Ensure single trailing newline
--finalize-trim-trailing-spaces   # Remove trailing whitespace
--finalize-line-endings=lf        # Normalize to LF line endings
--finalize-remove-bom             # Remove UTF-8 BOM

# Combined usage
--finalize-eof \
--finalize-trim-trailing-spaces \
--finalize-line-endings=lf \
--finalize-remove-bom

# Check mode (don't modify)
--check

# Apply mode (modify files)
# (no additional flags needed)
```

## Integration with CI/CD

These test files can be used to verify that the normalization features work correctly in automated environments:

```yaml
# .github/workflows/test.yml
- name: Test file normalization
  run: |
    # Test that improperly formatted files are detected
    if ./dist/goneat format --check --finalize-eof test-fixtures/*.txt; then
      echo "ERROR: Should have detected formatting issues"
      exit 1
    fi

    # Apply formatting
    ./dist/goneat format --finalize-eof test-fixtures/*.txt

    # Verify files are now properly formatted
    ./dist/goneat format --check --finalize-eof test-fixtures/*.txt
```

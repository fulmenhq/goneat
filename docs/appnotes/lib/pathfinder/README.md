# Pathfinder Library Documentation

This directory contains documentation for the Pathfinder library implementation.

## Planned Documents

- `audit-trail-usage.md` - Audit trail implementation guide
- `loader-patterns.md` - File loader patterns and best practices
- `security-considerations.md` - Security features and considerations
- `api-reference.md` - Complete API reference

## Overview

Pathfinder is a secure file system abstraction library designed for:

- **Secure file operations** with audit trails
- **Multiple loader backends** (local, remote, cloud)
- **Compliance-ready** audit logging
- **Deterministic testing** capabilities

## UUID Generation Strategy

### Dual-Track Approach

The Pathfinder audit system implements a custom dual-track UUID generation strategy optimized for both production security and test reproducibility.

#### Production Mode (Default)

- **Algorithm**: `crypto/rand` for cryptographically secure random generation
- **Entropy**: 128 bits (equivalent to UUIDv4's 122 bits of randomness)
- **Format**: UUID-like string (8-4-4-4-12) without RFC 4122 markers
- **Security**: Unpredictable, collision-resistant identifiers
- **Use Case**: Production audit trails requiring secure, unique IDs

#### Test/Replay Mode (Deterministic)

- **Algorithm**: SHA-256 hash of content + seed (custom UUIDv5-like)
- **Format**: UUID-like string (8-4-4-4-12) for consistency
- **Collision Resistance**: 2^128 (superior to UUIDv5's SHA-1 at 2^80)
- **Namespace Isolation**: Seed provides test scenario separation
- **Use Case**: Test idempotency, audit replay, debugging

### Why Not Standard UUIDv5?

We chose a custom implementation over standard UUIDv5 for several reasons:

1. **Security**: SHA-256 vs SHA-1
   - SHA-1 is cryptographically deprecated
   - SHA-256 provides 2^128 collision resistance vs 2^80 for SHA-1
   - Future-proof against advancing cryptographic attacks

2. **No External Dependencies**
   - Keeps goneat lightweight
   - Reduces supply chain attack surface
   - Simpler deployment and maintenance

3. **Test Control**
   - Complete control over deterministic behavior
   - Clear separation between production and test modes
   - Seed-based namespace isolation for test scenarios

4. **Intentionally Non-Standard**
   - Custom format indicates special audit trail usage
   - Not meant for interchange with external UUID systems
   - Optimized for our specific audit requirements

### Implementation Details

```go
// Production Mode
bytes := make([]byte, 16)
rand.Read(bytes)  // crypto/rand for security
// Format: xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx

// Test Mode (Deterministic)
content := fmt.Sprintf("%s|%s|%s|%s|%d",
    operation, path, loader, timestamp, code)
hash := sha256.Sum256([]byte(seed + "|" + content))
// Same UUID-like format from hash
```

### RFC 4122 Considerations

We intentionally do NOT set RFC 4122 version/variant bits because:

- This is a custom format, not standard UUIDv4 or UUIDv5
- Clear indication that these are audit-specific identifiers
- Avoids confusion with standard UUID implementations

Future enhancement could add RFC 4122 compliance:

```go
// For UUIDv4 compatibility (not currently implemented)
bytes[6] = (bytes[6] & 0x0f) | 0x40  // Version 4
bytes[8] = (bytes[8] & 0x3f) | 0x80  // Variant 10
```

### Testing

Comprehensive test coverage validates:

- **Uniqueness**: 10,000+ random UUIDs without collision
- **Idempotency**: Identical inputs produce identical UUIDs
- **Format**: Consistent 8-4-4-4-12 UUID-like structure
- **Isolation**: Different seeds produce different UUIDs
- **Performance**: Sub-millisecond generation in both modes

See `pkg/pathfinder/audit_uuid_test.go` for complete test suite.

## Status

🚧 **In Development** - Documentation will be added as features are implemented.

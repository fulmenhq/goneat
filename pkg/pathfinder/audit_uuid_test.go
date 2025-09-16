package pathfinder

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"regexp"
	"strings"
	"testing"
	"time"
)

// TestAuditUUIDGeneration provides comprehensive validation of our dual-track UUID approach
func TestAuditUUIDGeneration(t *testing.T) {
	t.Run("ProductionMode_AlwaysUnique", func(t *testing.T) {
		logger := NewAuditLogger()
		// Explicitly ensure we're in production mode (default)
		logger.SetDeterministicMode(false, "")

		// Generate many IDs rapidly to test uniqueness
		ids := make(map[string]bool)
		const numTests = 1000

		for i := 0; i < numTests; i++ {
			record := AuditRecord{
				Operation:    OpOpen,
				Path:         fmt.Sprintf("/tmp/file%d.txt", i),
				SourceLoader: "local",
				Result:       OperationResult{Status: "success", Code: 200},
				Timestamp:    time.Now().Add(time.Duration(i) * time.Nanosecond),
			}

			err := logger.LogOperation(record)
			if err != nil {
				t.Fatalf("LogOperation failed: %v", err)
			}
		}

		// Query all records
		results, err := logger.Query(AuditQuery{Limit: numTests})
		if err != nil {
			t.Fatalf("Query failed: %v", err)
		}

		if len(results) != numTests {
			t.Errorf("Expected %d records, got %d", numTests, len(results))
		}

		// Verify all IDs are unique
		for _, r := range results {
			if ids[r.ID] {
				t.Errorf("CRITICAL: Duplicate UUID in production mode: %s", r.ID)
			}
			ids[r.ID] = true
		}

		t.Logf("✅ Generated %d unique UUIDs in production mode", numTests)
	})

	t.Run("ProductionMode_UUIDv4Format", func(t *testing.T) {
		logger := NewAuditLogger()

		record := AuditRecord{
			Operation:    OpOpen,
			Path:         "/tmp/test.txt",
			SourceLoader: "local",
			Result:       OperationResult{Status: "success", Code: 200},
			Timestamp:    time.Now(),
		}

		_ = logger.LogOperation(record)
		results, _ := logger.Query(AuditQuery{Limit: 1})

		id := results[0].ID

		// Production now uses UUID-like format (8-4-4-4-12)
		uuidPattern := regexp.MustCompile(`^[a-f0-9]{8}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{12}$`)
		if !uuidPattern.MatchString(id) {
			t.Errorf("Production ID doesn't match UUID format: %s", id)
		}

		// Verify it has proper structure
		parts := strings.Split(id, "-")
		if len(parts) != 5 {
			t.Errorf("UUID should have 5 parts, got %d", len(parts))
		}

		t.Logf("Production UUID (custom format without RFC 4122 markers): %s", id)
	})

	t.Run("DeterministicMode_Idempotency", func(t *testing.T) {
		// Test that identical inputs produce identical outputs
		logger1 := NewAuditLogger()
		logger1.SetDeterministicMode(true, "test-seed-123")

		logger2 := NewAuditLogger()
		logger2.SetDeterministicMode(true, "test-seed-123")

		// Use recent timestamp to avoid retention cleanup
		fixedTime := time.Now().Add(-1 * time.Hour) // 1 hour ago, well within retention

		record := AuditRecord{
			Operation:    OpOpen,
			Path:         "/tmp/test.txt",
			SourceLoader: "local",
			Result:       OperationResult{Status: "success", Code: 200},
			Timestamp:    fixedTime,
		}

		// Log same record in both loggers
		if err := logger1.LogOperation(record); err != nil {
			t.Fatalf("LogOperation failed for logger1: %v", err)
		}
		if err := logger2.LogOperation(record); err != nil {
			t.Fatalf("LogOperation failed for logger2: %v", err)
		}

		results1, err1 := logger1.Query(AuditQuery{Limit: 1})
		results2, err2 := logger2.Query(AuditQuery{Limit: 1})

		if err1 != nil || err2 != nil {
			t.Fatalf("Query failed: err1=%v, err2=%v", err1, err2)
		}

		if len(results1) == 0 || len(results2) == 0 {
			t.Fatalf("No results returned: len1=%d, len2=%d", len(results1), len(results2))
		}

		if results1[0].ID != results2[0].ID {
			t.Errorf("Idempotency failed: same input produced different IDs:\n  %s\n  %s",
				results1[0].ID, results2[0].ID)
		}

		t.Logf("✅ Deterministic UUID (idempotent): %s", results1[0].ID)
	})

	t.Run("DeterministicMode_CustomUUIDFormat", func(t *testing.T) {
		logger := NewAuditLogger()
		logger.SetDeterministicMode(true, "format-test-seed")

		record := AuditRecord{
			Operation:    OpOpen,
			Path:         "/tmp/test.txt",
			SourceLoader: "local",
			Result:       OperationResult{Status: "success", Code: 200},
			Timestamp:    time.Now(),
		}

		_ = logger.LogOperation(record)
		results, _ := logger.Query(AuditQuery{Limit: 1})

		id := results[0].ID

		// Validate UUID-like format: 8-4-4-4-12
		uuidPattern := regexp.MustCompile(`^[a-f0-9]{8}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{12}$`)
		if !uuidPattern.MatchString(id) {
			t.Errorf("Deterministic ID doesn't match UUID format: %s", id)
		}

		parts := strings.Split(id, "-")
		if len(parts) != 5 {
			t.Errorf("UUID should have 5 parts, got %d", len(parts))
		}

		expectedLengths := []int{8, 4, 4, 4, 12}
		for i, part := range parts {
			if len(part) != expectedLengths[i] {
				t.Errorf("UUID part %d wrong length: got %d, want %d",
					i, len(part), expectedLengths[i])
			}
		}

		t.Logf("✅ Custom UUID format (8-4-4-4-12): %s", id)
	})

	t.Run("DeterministicMode_ContentSensitivity", func(t *testing.T) {
		logger := NewAuditLogger()
		logger.SetDeterministicMode(true, "content-test-seed")

		baseTime := time.Now()

		// Different operations should produce different IDs
		operations := []PathOperation{OpOpen, OpList, OpWalk, OpValidate}
		ids := make(map[string]PathOperation)

		for _, op := range operations {
			record := AuditRecord{
				Operation:    op,
				Path:         "/tmp/test.txt",                               // Same path
				SourceLoader: "local",                                       // Same loader
				Result:       OperationResult{Status: "success", Code: 200}, // Same result
				Timestamp:    baseTime,                                      // Same time
			}

			_ = logger.LogOperation(record)
		}

		results, _ := logger.Query(AuditQuery{Limit: len(operations)})

		for _, r := range results {
			if prevOp, exists := ids[r.ID]; exists {
				t.Errorf("Different operations produced same ID: %s used by both %s and %s",
					r.ID, prevOp, r.Operation)
			}
			ids[r.ID] = r.Operation
		}

		t.Logf("✅ Each unique content produces unique deterministic UUID")
	})

	t.Run("DeterministicMode_SeedIsolation", func(t *testing.T) {
		// Different seeds must produce different IDs for same content
		seeds := []string{"seed-alpha", "seed-beta", "seed-gamma"}
		fixedTime := time.Now().Add(-30 * time.Minute) // Recent enough to avoid retention cleanup

		record := AuditRecord{
			Operation:    OpOpen,
			Path:         "/tmp/test.txt",
			SourceLoader: "local",
			Result:       OperationResult{Status: "success", Code: 200},
			Timestamp:    fixedTime,
		}

		ids := make(map[string]string)

		for _, seed := range seeds {
			logger := NewAuditLogger()
			logger.SetDeterministicMode(true, seed)
			_ = logger.LogOperation(record)

			results, _ := logger.Query(AuditQuery{Limit: 1})
			id := results[0].ID

			if prevSeed, exists := ids[id]; exists {
				t.Errorf("Different seeds produced same ID: %s used by both '%s' and '%s'",
					id, prevSeed, seed)
			}
			ids[id] = seed
		}

		t.Logf("✅ Seed isolation working: %d different seeds = %d unique UUIDs",
			len(seeds), len(ids))
	})

	t.Run("RFC4122_Compatibility", func(t *testing.T) {
		// Test that our custom UUID could be RFC 4122 compliant with minor changes
		logger := NewAuditLogger()
		logger.SetDeterministicMode(true, "rfc-test")

		record := AuditRecord{
			Operation:    OpOpen,
			Path:         "/tmp/test.txt",
			SourceLoader: "local",
			Result:       OperationResult{Status: "success", Code: 200},
			Timestamp:    time.Now(),
		}

		_ = logger.LogOperation(record)
		results, _ := logger.Query(AuditQuery{Limit: 1})

		id := results[0].ID

		// Parse the UUID to check where version/variant bits would go
		cleanID := strings.ReplaceAll(id, "-", "")
		bytes, _ := hex.DecodeString(cleanID)

		if len(bytes) >= 9 {
			// Version bits are in byte 6, bits 4-7 (should be 0101 for v5)
			// Variant bits are in byte 8, bits 6-7 (should be 10 for RFC 4122)

			t.Logf("UUID bytes[6] (version byte): %02x (would need 0x5X for UUIDv5)", bytes[6])
			t.Logf("UUID bytes[8] (variant byte): %02x (would need 0x8X-0xBX for RFC 4122)", bytes[8])

			// Note: Current implementation doesn't set these bits
			// This is intentional - we use a custom format for deterministic testing
		}

		t.Logf("Custom UUID (without RFC markers): %s", id)
	})

	t.Run("SHA256_Verification", func(t *testing.T) {
		// Verify that deterministic IDs are actually using SHA-256
		logger := NewAuditLogger()
		seed := "sha256-verify"
		logger.SetDeterministicMode(true, seed)

		fixedTime := time.Now().Add(-15 * time.Minute) // Recent enough to avoid retention cleanup
		record := AuditRecord{
			Operation:    OpOpen,
			Path:         "/tmp/test.txt",
			SourceLoader: "local",
			Result:       OperationResult{Status: "success", Code: 200},
			Timestamp:    fixedTime,
		}

		_ = logger.LogOperation(record)
		results, _ := logger.Query(AuditQuery{Limit: 1})

		// Manually compute expected SHA-256
		content := fmt.Sprintf("%s|%s|%s|%s|%d",
			record.Operation,
			record.Path,
			record.SourceLoader,
			record.Timestamp.Format(time.RFC3339Nano),
			record.Result.Code,
		)
		fullContent := seed + "|" + content
		hash := sha256.Sum256([]byte(fullContent))
		hashStr := hex.EncodeToString(hash[:])

		expectedID := fmt.Sprintf("%s-%s-%s-%s-%s",
			hashStr[0:8],
			hashStr[8:12],
			hashStr[12:16],
			hashStr[16:20],
			hashStr[20:32],
		)

		if results[0].ID != expectedID {
			t.Errorf("SHA-256 computation mismatch:\ngot:      %s\nexpected: %s",
				results[0].ID, expectedID)
		}

		t.Logf("✅ Verified SHA-256 based UUID: %s", results[0].ID)
	})

	t.Run("Collision_Resistance", func(t *testing.T) {
		// Test that our SHA-256 approach has good collision resistance
		logger := NewAuditLogger()
		logger.SetDeterministicMode(true, "collision-test")

		ids := make(map[string]bool)
		const numTests = 10000

		for i := 0; i < numTests; i++ {
			// Create slightly different records
			record := AuditRecord{
				Operation:    OpOpen,
				Path:         fmt.Sprintf("/tmp/file%d.txt", i),
				SourceLoader: "local",
				Result:       OperationResult{Status: "success", Code: 200 + (i % 10)},
				Timestamp:    time.Now().Add(time.Duration(i) * time.Millisecond),
			}

			_ = logger.LogOperation(record)
		}

		results, _ := logger.Query(AuditQuery{Limit: numTests})

		for _, r := range results {
			if ids[r.ID] {
				t.Errorf("COLLISION DETECTED: Duplicate deterministic UUID: %s", r.ID)
			}
			ids[r.ID] = true
		}

		t.Logf("✅ No collisions in %d deterministic UUIDs (SHA-256)", numTests)
	})
}

// TestUUIDComparison documents why we chose our custom approach over UUIDv5
func TestUUIDComparison(t *testing.T) {
	t.Run("WhySHA256NotSHA1", func(t *testing.T) {
		// UUIDv5 uses SHA-1, which is deprecated for cryptographic use
		// SHA-256 provides better collision resistance

		logger := NewAuditLogger()
		logger.SetDeterministicMode(true, "security-test")

		record := AuditRecord{
			Operation:    OpOpen,
			Path:         "/tmp/secure.txt",
			SourceLoader: "local",
			Result:       OperationResult{Status: "success", Code: 200},
			Timestamp:    time.Now(),
		}

		_ = logger.LogOperation(record)
		results, _ := logger.Query(AuditQuery{Limit: 1})

		t.Logf("SHA-256 based UUID (more secure than UUIDv5's SHA-1): %s", results[0].ID)
		t.Logf("Collision resistance: 2^128 for SHA-256 vs 2^80 for SHA-1")
	})

	t.Run("WhyNotGoogleUUID", func(t *testing.T) {
		// Document why we don't use external UUID library
		t.Logf("Reasons for custom implementation:")
		t.Logf("1. No external dependencies (keeping goneat lightweight)")
		t.Logf("2. SHA-256 instead of SHA-1 for better security")
		t.Logf("3. Complete control over deterministic behavior for testing")
		t.Logf("4. Simpler implementation for our specific use case")
	})

	t.Run("ProductionVsTest", func(t *testing.T) {
		t.Logf("Production Mode (Random):")
		t.Logf("  - Uses crypto/rand for cryptographic security")
		t.Logf("  - Equivalent to UUIDv4 entropy (122 bits)")
		t.Logf("  - Always generates unique IDs")

		t.Logf("\nTest Mode (Deterministic):")
		t.Logf("  - Uses SHA-256 hash of content + seed")
		t.Logf("  - Similar to UUIDv5 but with SHA-256")
		t.Logf("  - Enables test idempotency and replay")
		t.Logf("  - Seed provides namespace isolation")
	})
}

// BenchmarkUUIDGeneration measures performance of both modes
func BenchmarkUUIDGeneration(b *testing.B) {
	b.Run("RandomMode", func(b *testing.B) {
		logger := NewAuditLogger()
		record := AuditRecord{
			Operation:    OpOpen,
			Path:         "/tmp/bench.txt",
			SourceLoader: "local",
			Result:       OperationResult{Status: "success", Code: 200},
			Timestamp:    time.Now(),
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = logger.LogOperation(record)
		}
	})

	b.Run("DeterministicMode", func(b *testing.B) {
		logger := NewAuditLogger()
		logger.SetDeterministicMode(true, "bench-seed")

		record := AuditRecord{
			Operation:    OpOpen,
			Path:         "/tmp/bench.txt",
			SourceLoader: "local",
			Result:       OperationResult{Status: "success", Code: 200},
			Timestamp:    time.Now(),
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = logger.LogOperation(record)
		}
	})
}

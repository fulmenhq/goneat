package pathfinder

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

// AuditLoggerImpl provides a complete audit trail implementation
type AuditLoggerImpl struct {
	mu             sync.RWMutex
	records        []AuditRecord
	complianceMode ComplianceMode
	retentionDays  int
	maxRecords     int
	exportFormats  []ExportFormat
	deterministic  bool   // Use deterministic ID generation for testing/replay
	seed           string // Seed for deterministic generation
}

// NewAuditLogger creates a new audit logger with default settings
//
// UUID Generation Strategy (Dual-Track Approach):
//
// Production Mode (Default):
//   - Uses crypto/rand for cryptographically secure random IDs
//   - Equivalent entropy to UUIDv4 (122 bits of randomness)
//   - Currently returns 32-char hex string (128 bits)
//   - TODO: Add RFC 4122 version/variant bits for standard UUIDv4 format
//
// Test/Replay Mode (Deterministic):
//   - Custom UUIDv5-like implementation using SHA-256 instead of SHA-1
//   - SHA-256 chosen over SHA-1 for better collision resistance (2^128 vs 2^80)
//   - Formatted as UUID-like string (8-4-4-4-12) for consistency
//   - Does NOT set RFC 4122 version/variant bits (intentionally custom)
//   - Seed provides namespace isolation similar to UUIDv5 namespaces
//
// Why Not Standard UUIDv5?
//  1. SHA-1 is cryptographically deprecated; SHA-256 is more secure
//  2. No external dependencies keeps goneat lightweight
//  3. Complete control over deterministic behavior for testing
//  4. Clear separation between production and test modes
//
// This dual approach ensures:
//   - Security and unpredictability in production (crypto/rand)
//   - Test idempotency and replay capability (deterministic SHA-256)
//   - Audit trail consistency across test runs
func NewAuditLogger() *AuditLoggerImpl {
	return &AuditLoggerImpl{
		records:        make([]AuditRecord, 0),
		complianceMode: ComplianceNone,
		retentionDays:  90,
		maxRecords:     10000,
		exportFormats:  []ExportFormat{ExportJSON},
		deterministic:  false, // Default to random generation for security
		seed:           "",
	}
}

// LogOperation records an audit event
func (a *AuditLoggerImpl) LogOperation(record AuditRecord) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	// Generate ID if not provided
	if record.ID == "" {
		record.ID = a.generateID(&record)
	}

	// Set timestamp if not provided
	if record.Timestamp.IsZero() {
		record.Timestamp = time.Now()
	}

	// Apply compliance-specific rules
	if err := a.applyComplianceRules(&record); err != nil {
		return fmt.Errorf("compliance rule violation: %w", err)
	}

	// Add to records
	a.records = append(a.records, record)

	// Maintain retention policy
	a.cleanupOldRecords()

	// Enforce max records limit
	if len(a.records) > a.maxRecords {
		// Remove oldest records
		keep := len(a.records) - a.maxRecords
		a.records = a.records[keep:]
	}

	return nil
}

// Query retrieves audit records based on constraints
func (a *AuditLoggerImpl) Query(query AuditQuery) ([]AuditRecord, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	var results []AuditRecord

	for _, record := range a.records {
		if a.matchesQuery(record, query) {
			results = append(results, record)
		}
	}

	// Sort by timestamp (newest first)
	sort.Slice(results, func(i, j int) bool {
		return results[i].Timestamp.After(results[j].Timestamp)
	})

	// Apply pagination
	if query.Limit > 0 && len(results) > query.Limit {
		offset := query.Offset
		if offset < 0 {
			offset = 0
		}
		end := offset + query.Limit
		if end > len(results) {
			end = len(results)
		}
		if offset >= len(results) {
			return []AuditRecord{}, nil
		}
		results = results[offset:end]
	}

	return results, nil
}

// Export exports audit records in the specified format
func (a *AuditLoggerImpl) Export(format ExportFormat) ([]byte, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	switch format {
	case ExportJSON:
		return a.exportJSON()
	case ExportCSV:
		return a.exportCSV()
	case ExportSyslog:
		return a.exportSyslog()
	default:
		return nil, fmt.Errorf("unsupported export format: %s", format)
	}
}

// SetComplianceMode sets the compliance mode for the audit logger
func (a *AuditLoggerImpl) SetComplianceMode(mode ComplianceMode) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	// Validate compliance mode
	switch mode {
	case ComplianceNone, ComplianceHIPAA, ComplianceSOC2, CompliancePCIDSS, ComplianceGDPR:
		a.complianceMode = mode
		return nil
	default:
		return fmt.Errorf("unsupported compliance mode: %s", mode)
	}
}

// Configure updates audit logger configuration (satisfies AuditLogger interface)
func (a *AuditLoggerImpl) Configure(config AuditConfig) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if err := a.SetComplianceMode(config.ComplianceMode); err != nil {
		return err
	}

	a.retentionDays = config.RetentionDays
	if a.retentionDays <= 0 {
		a.retentionDays = 90
	}

	a.exportFormats = config.ExportFormats
	if len(a.exportFormats) == 0 {
		a.exportFormats = []ExportFormat{ExportJSON}
	}

	return nil
}

// matchesQuery checks if a record matches the query constraints
func (a *AuditLoggerImpl) matchesQuery(record AuditRecord, query AuditQuery) bool {
	if query.StartTime != nil && record.Timestamp.Before(*query.StartTime) {
		return false
	}
	if query.EndTime != nil && record.Timestamp.After(*query.EndTime) {
		return false
	}
	if query.Operation != nil && record.Operation != *query.Operation {
		return false
	}
	if query.Path != nil && !strings.Contains(record.Path, *query.Path) {
		return false
	}
	if query.SourceLoader != nil && record.SourceLoader != *query.SourceLoader {
		return false
	}
	if query.Result != nil && record.Result.Status != *query.Result {
		return false
	}
	return true
}

// applyComplianceRules applies compliance-specific validation and enrichment
func (a *AuditLoggerImpl) applyComplianceRules(record *AuditRecord) error {
	switch a.complianceMode {
	case ComplianceHIPAA:
		return a.applyHIPAARules(record)
	case ComplianceSOC2:
		return a.applySOC2Rules(record)
	case CompliancePCIDSS:
		return a.applyPCIDSSRules(record)
	case ComplianceGDPR:
		return a.applyGDPRRules(record)
	default:
		return nil
	}
}

// applyHIPAARules applies HIPAA-specific audit rules
func (a *AuditLoggerImpl) applyHIPAARules(record *AuditRecord) error {
	// HIPAA requires tracking of all access to protected health information
	if strings.Contains(record.Path, "phi") || strings.Contains(record.Path, "health") {
		record.SecurityFlags = append(record.SecurityFlags, "HIPAA_PHI_ACCESS")
	}
	return nil
}

// applySOC2Rules applies SOC2-specific audit rules
func (a *AuditLoggerImpl) applySOC2Rules(record *AuditRecord) error {
	// SOC2 requires detailed access logging for security monitoring
	if record.Result.Status == "denied" {
		record.SecurityFlags = append(record.SecurityFlags, "SOC2_ACCESS_DENIED")
	}
	return nil
}

// applyPCIDSSRules applies PCI-DSS-specific audit rules
func (a *AuditLoggerImpl) applyPCIDSSRules(record *AuditRecord) error {
	// PCI-DSS requires tracking of all access to cardholder data
	if strings.Contains(record.Path, "card") || strings.Contains(record.Path, "payment") {
		record.SecurityFlags = append(record.SecurityFlags, "PCI_CHD_ACCESS")
	}
	return nil
}

// applyGDPRRules applies GDPR-specific audit rules
func (a *AuditLoggerImpl) applyGDPRRules(record *AuditRecord) error {
	// GDPR requires tracking of personal data access
	if strings.Contains(record.Path, "personal") || strings.Contains(record.Path, "user") {
		record.SecurityFlags = append(record.SecurityFlags, "GDPR_PERSONAL_DATA")
	}
	return nil
}

// cleanupOldRecords removes records older than retention period
func (a *AuditLoggerImpl) cleanupOldRecords() {
	cutoff := time.Now().AddDate(0, 0, -a.retentionDays)
	var kept []AuditRecord

	for _, record := range a.records {
		if record.Timestamp.After(cutoff) {
			kept = append(kept, record)
		}
	}

	a.records = kept
}

// exportJSON exports records as JSON
func (a *AuditLoggerImpl) exportJSON() ([]byte, error) {
	return json.MarshalIndent(a.records, "", "  ")
}

// exportCSV exports records as CSV
func (a *AuditLoggerImpl) exportCSV() ([]byte, error) {
	if len(a.records) == 0 {
		return []byte{}, nil
	}

	var buf strings.Builder
	writer := csv.NewWriter(&buf)

	// Write header
	header := []string{
		"id", "timestamp", "operation", "path", "source_loader", "constraint",
		"result_status", "result_code", "result_message", "duration_ms",
		"user_context", "security_flags", "error_type", "error_message", "error_code",
	}
	if err := writer.Write(header); err != nil {
		return nil, err
	}

	// Write records
	for _, record := range a.records {
		row := []string{
			record.ID,
			record.Timestamp.Format(time.RFC3339),
			string(record.Operation),
			record.Path,
			record.SourceLoader,
			record.Constraint,
			record.Result.Status,
			fmt.Sprintf("%d", record.Result.Code),
			record.Result.Message,
			fmt.Sprintf("%d", record.Duration.Milliseconds()),
			fmt.Sprintf("%v", record.UserContext),
			strings.Join(record.SecurityFlags, ";"),
			"",
			"",
			"",
		}

		if record.ErrorDetails != nil {
			row[12] = record.ErrorDetails.Type
			row[13] = record.ErrorDetails.Message
			row[14] = record.ErrorDetails.Code
		}

		if err := writer.Write(row); err != nil {
			return nil, err
		}
	}

	writer.Flush()
	if err := writer.Error(); err != nil {
		return nil, err
	}

	return []byte(buf.String()), nil
}

// exportSyslog exports records in syslog format
func (a *AuditLoggerImpl) exportSyslog() ([]byte, error) {
	var buf strings.Builder

	for _, record := range a.records {
		line := fmt.Sprintf("<%d>%s %s pathfinder[%s]: operation=%s path=%s result=%s",
			6, // INFO level
			record.Timestamp.Format(time.RFC3339),
			"pathfinder",
			record.ID,
			record.Operation,
			record.Path,
			record.Result.Status,
		)

		if record.Result.Code != 0 {
			line += fmt.Sprintf(" code=%d", record.Result.Code)
		}

		if record.Result.Message != "" {
			line += fmt.Sprintf(" message=%q", record.Result.Message)
		}

		if record.SourceLoader != "" {
			line += fmt.Sprintf(" loader=%s", record.SourceLoader)
		}

		if len(record.SecurityFlags) > 0 {
			line += fmt.Sprintf(" flags=%s", strings.Join(record.SecurityFlags, ","))
		}

		buf.WriteString(line + "\n")
	}

	return []byte(buf.String()), nil
}

// GetStats returns audit statistics
func (a *AuditLoggerImpl) GetStats() AuditStats {
	a.mu.RLock()
	defer a.mu.RUnlock()

	stats := AuditStats{
		TotalRecords:   len(a.records),
		ComplianceMode: a.complianceMode,
	}

	if len(a.records) > 0 {
		stats.OldestRecord = a.records[0].Timestamp
		stats.NewestRecord = a.records[len(a.records)-1].Timestamp
	}

	// Count operations
	opCounts := make(map[PathOperation]int)
	for _, record := range a.records {
		opCounts[record.Operation]++
	}
	stats.OperationCounts = opCounts

	return stats
}

// AuditStats provides audit statistics
type AuditStats struct {
	TotalRecords    int                   `json:"total_records"`
	OldestRecord    time.Time             `json:"oldest_record"`
	NewestRecord    time.Time             `json:"newest_record"`
	ComplianceMode  ComplianceMode        `json:"compliance_mode"`
	OperationCounts map[PathOperation]int `json:"operation_counts"`
}

// generateID creates a unique identifier using our dual-track approach
//
// Production: Generates cryptographically secure random UUIDs (similar to UUIDv4)
// Test Mode: Generates deterministic UUIDs using SHA-256 (similar to UUIDv5 but more secure)
func (a *AuditLoggerImpl) generateID(record *AuditRecord) string {
	if a.deterministic && a.seed != "" {
		// Deterministic mode for testing/replay scenarios
		return a.generateDeterministicID(record)
	}

	// Production mode: cryptographically secure random UUID
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		// Fallback to timestamp-based ID if random fails
		// This should never happen with crypto/rand, but we handle it gracefully
		return fmt.Sprintf("%d-%d", time.Now().UnixNano(), time.Now().Unix())
	}

	// Format as UUID-like string for consistency with deterministic mode
	// Note: We intentionally don't set RFC 4122 version/variant bits
	// This is a custom format optimized for our audit use case
	// TODO: Consider adding RFC 4122 markers for standard compliance:
	//   bytes[6] = (bytes[6] & 0x0f) | 0x40  // Version 4
	//   bytes[8] = (bytes[8] & 0x3f) | 0x80  // Variant 10
	return fmt.Sprintf("%x-%x-%x-%x-%x",
		bytes[0:4], bytes[4:6], bytes[6:8], bytes[8:10], bytes[10:16])
}

// generateDeterministicID creates a deterministic ID based on record content
//
// This is our custom UUIDv5-like implementation that uses SHA-256 instead of SHA-1
// for improved security and collision resistance.
//
// Key differences from standard UUIDv5:
//   - Uses SHA-256 instead of SHA-1 (better collision resistance)
//   - Does NOT set RFC 4122 version/variant bits (intentionally custom)
//   - Seed acts as namespace for isolation between test scenarios
//
// The deterministic nature enables:
//   - Test idempotency (same input always produces same UUID)
//   - Audit trail replay for debugging
//   - Predictable IDs in test environments
//
// Content hashed includes: operation|path|source_loader|timestamp|result_code
// This ensures unique IDs for different operations while maintaining consistency
// for identical operations.
func (a *AuditLoggerImpl) generateDeterministicID(record *AuditRecord) string {
	// Create a unique string from record content for deterministic generation
	content := fmt.Sprintf("%s|%s|%s|%s|%d",
		record.Operation,
		record.Path,
		record.SourceLoader,
		record.Timestamp.Format(time.RFC3339Nano),
		record.Result.Code,
	)

	// Include seed for namespace isolation (similar to UUIDv5 namespaces)
	// The seed ensures different test scenarios generate different UUIDs
	// even for identical content
	content = a.seed + "|" + content

	// Generate SHA-256 hash (256 bits, much stronger than SHA-1's 160 bits)
	hash := sha256.Sum256([]byte(content))
	hashStr := hex.EncodeToString(hash[:])

	// Format as UUID-like string: xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx
	// We use the first 32 hex chars (128 bits) from the SHA-256 hash
	// Note: We intentionally don't set RFC 4122 bits to indicate this is
	// a custom format, not a standard UUIDv5
	return fmt.Sprintf("%s-%s-%s-%s-%s",
		hashStr[0:8],   // 32 bits
		hashStr[8:12],  // 16 bits
		hashStr[12:16], // 16 bits
		hashStr[16:20], // 16 bits
		hashStr[20:32], // 48 bits
	) // Total: 128 bits (standard UUID size)
}

// SetDeterministicMode enables deterministic ID generation for testing/replay
func (a *AuditLoggerImpl) SetDeterministicMode(enabled bool, seed string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.deterministic = enabled
	a.seed = seed
}

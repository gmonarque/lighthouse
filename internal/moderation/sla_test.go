package moderation

import (
	"testing"
	"time"
)

func TestSLAEnforcer_New(t *testing.T) {
	policy := DefaultModerationPolicy()

	enforcer := NewSLAEnforcer(nil, policy)
	if enforcer == nil {
		t.Fatal("Expected enforcer to be created")
	}
}

func TestReport_IsPending(t *testing.T) {
	report := NewReport(ReportKindReport, CategoryDMCA, "pubkey")

	if !report.IsPending() {
		t.Error("Expected new report to be pending")
	}

	report.Acknowledge()
	if report.IsPending() {
		t.Error("Expected acknowledged report to not be pending")
	}
}

func TestReport_NeedsAcknowledgment(t *testing.T) {
	// Fresh report
	report := NewReport(ReportKindReport, CategoryDMCA, "pubkey")
	if report.NeedsAcknowledgment() {
		t.Error("Fresh report should not need acknowledgment")
	}

	// Old report
	oldReport := &Report{
		Kind:           ReportKindReport,
		Category:       CategoryDMCA,
		ReporterPubkey: "pubkey",
		Status:         StatusPending,
		CreatedAt:      time.Now().Add(-25 * time.Hour),
	}
	if !oldReport.NeedsAcknowledgment() {
		t.Error("Old pending report should need acknowledgment")
	}

	// Acknowledged report
	oldReport.Acknowledge()
	if oldReport.NeedsAcknowledgment() {
		t.Error("Acknowledged report should not need acknowledgment")
	}
}

func TestReport_IsOpen(t *testing.T) {
	report := NewReport(ReportKindReport, CategorySpam, "pubkey")

	if !report.IsOpen() {
		t.Error("Expected new report to be open")
	}

	report.Resolve("Resolved", "moderator")
	if report.IsOpen() {
		t.Error("Expected resolved report to not be open")
	}
}

func TestReport_Lifecycle(t *testing.T) {
	report := NewReport(ReportKindReport, CategoryMalware, "reporter-key")
	report.SetTarget("event-123", "infohash-abc")
	report.SetEvidence("Evidence text", "scope")

	// Check initial state
	if report.Status != StatusPending {
		t.Errorf("Expected status 'pending', got '%s'", report.Status)
	}
	if report.TargetEventID != "event-123" {
		t.Errorf("Expected target event ID 'event-123', got '%s'", report.TargetEventID)
	}
	if report.TargetInfohash != "infohash-abc" {
		t.Errorf("Expected target infohash 'infohash-abc', got '%s'", report.TargetInfohash)
	}

	// Acknowledge
	report.Acknowledge()
	if report.Status != StatusAcknowledged {
		t.Errorf("Expected status 'acknowledged', got '%s'", report.Status)
	}
	if report.AcknowledgedAt == nil {
		t.Error("Expected AcknowledgedAt to be set")
	}

	// Start investigation
	report.StartInvestigation()
	if report.Status != StatusInvestigating {
		t.Errorf("Expected status 'investigating', got '%s'", report.Status)
	}

	// Resolve
	report.Resolve("Malware confirmed and removed", "moderator-key")
	if report.Status != StatusResolved {
		t.Errorf("Expected status 'resolved', got '%s'", report.Status)
	}
	if report.Resolution != "Malware confirmed and removed" {
		t.Errorf("Expected resolution text, got '%s'", report.Resolution)
	}
	if report.ResolvedBy != "moderator-key" {
		t.Errorf("Expected resolved by 'moderator-key', got '%s'", report.ResolvedBy)
	}
	if report.ResolvedAt == nil {
		t.Error("Expected ResolvedAt to be set")
	}
}

func TestReport_Reject(t *testing.T) {
	report := NewReport(ReportKindReport, CategoryOther, "reporter-key")
	report.Acknowledge()

	report.Reject("Insufficient evidence", "moderator-key")

	if report.Status != StatusRejected {
		t.Errorf("Expected status 'rejected', got '%s'", report.Status)
	}
	if report.Resolution != "Insufficient evidence" {
		t.Errorf("Expected rejection reason, got '%s'", report.Resolution)
	}
	if report.ResolvedAt == nil {
		t.Error("Expected ResolvedAt to be set")
	}
}

func TestReportStatus(t *testing.T) {
	tests := []struct {
		status   ReportStatus
		expected string
	}{
		{StatusPending, "pending"},
		{StatusAcknowledged, "acknowledged"},
		{StatusInvestigating, "investigating"},
		{StatusResolved, "resolved"},
		{StatusRejected, "rejected"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			if string(tt.status) != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, string(tt.status))
			}
		})
	}
}

func TestReportCategory(t *testing.T) {
	tests := []struct {
		category ReportCategory
		expected string
	}{
		{CategoryDMCA, "dmca"},
		{CategoryIllegal, "illegal"},
		{CategorySpam, "spam"},
		{CategoryMalware, "malware"},
		{CategoryFalseInfo, "false_info"},
		{CategoryDuplicate, "duplicate"},
		{CategoryOther, "other"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			if string(tt.category) != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, string(tt.category))
			}
		})
	}
}

func TestNewReport(t *testing.T) {
	report := NewReport(ReportKindReport, CategoryDMCA, "reporter-pubkey")

	if report.ReportID == "" {
		t.Error("Expected report ID to be generated")
	}

	if report.Kind != ReportKindReport {
		t.Errorf("Expected kind 'report', got '%s'", report.Kind)
	}

	if report.Category != CategoryDMCA {
		t.Errorf("Expected category 'dmca', got '%s'", report.Category)
	}

	if report.ReporterPubkey != "reporter-pubkey" {
		t.Errorf("Expected reporter pubkey 'reporter-pubkey', got '%s'", report.ReporterPubkey)
	}

	if report.Status != StatusPending {
		t.Errorf("Expected status 'pending', got '%s'", report.Status)
	}

	if report.CreatedAt.IsZero() {
		t.Error("Expected CreatedAt to be set")
	}
}

func TestReportToJSON(t *testing.T) {
	report := NewReport(ReportKindAppeal, CategoryDuplicate, "pubkey")
	report.SetTarget("event-id", "infohash")
	report.SetEvidence("Test evidence", "global")

	data, err := report.ToJSON()
	if err != nil {
		t.Fatalf("Failed to serialize: %v", err)
	}

	parsed, err := ReportFromJSON(data)
	if err != nil {
		t.Fatalf("Failed to deserialize: %v", err)
	}

	if parsed.Kind != report.Kind {
		t.Errorf("Kind mismatch: expected %s, got %s", report.Kind, parsed.Kind)
	}
	if parsed.Category != report.Category {
		t.Errorf("Category mismatch: expected %s, got %s", report.Category, parsed.Category)
	}
	if parsed.TargetEventID != report.TargetEventID {
		t.Errorf("TargetEventID mismatch: expected %s, got %s", report.TargetEventID, parsed.TargetEventID)
	}
}

func TestDefaultModerationPolicy(t *testing.T) {
	policy := DefaultModerationPolicy()

	if policy.AcknowledgmentDeadline != 24*time.Hour {
		t.Errorf("Expected 24h acknowledgment deadline, got %v", policy.AcknowledgmentDeadline)
	}

	if policy.ResolutionDeadline != 7*24*time.Hour {
		t.Errorf("Expected 7 day resolution deadline, got %v", policy.ResolutionDeadline)
	}

	if !policy.AutoAcknowledge {
		t.Error("Expected auto-acknowledge to be true by default")
	}

	if !policy.RequireEvidence {
		t.Error("Expected require evidence to be true by default")
	}
}

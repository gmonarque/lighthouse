// Package moderation handles reports and appeals
package moderation

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"time"
)

// ReportKind represents the type of moderation request
type ReportKind string

const (
	ReportKindReport ReportKind = "report"
	ReportKindAppeal ReportKind = "appeal"
)

// ReportStatus represents the status of a report
type ReportStatus string

const (
	StatusPending       ReportStatus = "pending"
	StatusAcknowledged  ReportStatus = "acknowledged"
	StatusInvestigating ReportStatus = "investigating"
	StatusResolved      ReportStatus = "resolved"
	StatusRejected      ReportStatus = "rejected"
)

// ReportCategory represents the category of a report
type ReportCategory string

const (
	CategoryDMCA        ReportCategory = "dmca"
	CategoryIllegal     ReportCategory = "illegal"
	CategorySpam        ReportCategory = "spam"
	CategoryMalware     ReportCategory = "malware"
	CategoryFalseInfo   ReportCategory = "false_info"
	CategoryDuplicate   ReportCategory = "duplicate"
	CategoryOther       ReportCategory = "other"
)

// Report represents a moderation report or appeal
type Report struct {
	ID              string         `json:"id"`
	ReportID        string         `json:"report_id"`
	Kind            ReportKind     `json:"kind"`
	TargetEventID   string         `json:"target_event_id,omitempty"`
	TargetInfohash  string         `json:"target_infohash,omitempty"`
	Category        ReportCategory `json:"category"`
	Evidence        string         `json:"evidence,omitempty"`
	Scope           string         `json:"scope,omitempty"`
	Jurisdiction    string         `json:"jurisdiction,omitempty"`
	ReporterPubkey  string         `json:"reporter_pubkey"`
	ReporterContact string         `json:"reporter_contact,omitempty"`
	Signature       string         `json:"signature,omitempty"`
	Status          ReportStatus   `json:"status"`
	Resolution      string         `json:"resolution,omitempty"`
	CreatedAt       time.Time      `json:"created_at"`
	AcknowledgedAt  *time.Time     `json:"acknowledged_at,omitempty"`
	ResolvedAt      *time.Time     `json:"resolved_at,omitempty"`
	ResolvedBy      string         `json:"resolved_by,omitempty"`
}

// NewReport creates a new report
func NewReport(kind ReportKind, category ReportCategory, reporterPubkey string) *Report {
	r := &Report{
		Kind:           kind,
		Category:       category,
		ReporterPubkey: reporterPubkey,
		Status:         StatusPending,
		CreatedAt:      time.Now().UTC(),
	}
	r.ReportID = r.generateID()
	return r
}

// generateID generates a unique report ID
func (r *Report) generateID() string {
	data := struct {
		Kind           ReportKind
		TargetInfohash string
		Reporter       string
		CreatedAt      int64
	}{
		Kind:           r.Kind,
		TargetInfohash: r.TargetInfohash,
		Reporter:       r.ReporterPubkey,
		CreatedAt:      r.CreatedAt.UnixNano(),
	}

	bytes, _ := json.Marshal(data)
	hash := sha256.Sum256(bytes)
	return hex.EncodeToString(hash[:12])
}

// SetTarget sets the report target
func (r *Report) SetTarget(eventID, infohash string) {
	r.TargetEventID = eventID
	r.TargetInfohash = infohash
}

// SetEvidence sets the evidence for the report
func (r *Report) SetEvidence(evidence, scope string) {
	r.Evidence = evidence
	r.Scope = scope
}

// Acknowledge marks the report as acknowledged
func (r *Report) Acknowledge() {
	r.Status = StatusAcknowledged
	now := time.Now().UTC()
	r.AcknowledgedAt = &now
}

// StartInvestigation marks the report as under investigation
func (r *Report) StartInvestigation() {
	r.Status = StatusInvestigating
}

// Resolve resolves the report
func (r *Report) Resolve(resolution, resolvedBy string) {
	r.Status = StatusResolved
	r.Resolution = resolution
	r.ResolvedBy = resolvedBy
	now := time.Now().UTC()
	r.ResolvedAt = &now
}

// Reject rejects the report
func (r *Report) Reject(reason, rejectedBy string) {
	r.Status = StatusRejected
	r.Resolution = reason
	r.ResolvedBy = rejectedBy
	now := time.Now().UTC()
	r.ResolvedAt = &now
}

// IsOpen returns true if the report is still open
func (r *Report) IsOpen() bool {
	return r.Status != StatusResolved && r.Status != StatusRejected
}

// IsPending returns true if the report hasn't been acknowledged
func (r *Report) IsPending() bool {
	return r.Status == StatusPending
}

// TimeSinceCreated returns the duration since the report was created
func (r *Report) TimeSinceCreated() time.Duration {
	return time.Since(r.CreatedAt)
}

// NeedsAcknowledgment returns true if the report needs acknowledgment (>24h)
func (r *Report) NeedsAcknowledgment() bool {
	return r.IsPending() && r.TimeSinceCreated() > 24*time.Hour
}

// ToJSON serializes the report to JSON
func (r *Report) ToJSON() ([]byte, error) {
	return json.Marshal(r)
}

// ReportFromJSON deserializes a report from JSON
func ReportFromJSON(data []byte) (*Report, error) {
	var r Report
	if err := json.Unmarshal(data, &r); err != nil {
		return nil, err
	}
	return &r, nil
}

// ReportFilter for querying reports
type ReportFilter struct {
	Kind           ReportKind
	Status         ReportStatus
	Category       ReportCategory
	TargetInfohash string
	ReporterPubkey string
	Since          *time.Time
	Until          *time.Time
	Limit          int
	Offset         int
}

// ReportStats contains report statistics
type ReportStats struct {
	TotalReports      int64 `json:"total_reports"`
	PendingReports    int64 `json:"pending_reports"`
	AcknowledgedCount int64 `json:"acknowledged_count"`
	ResolvedCount     int64 `json:"resolved_count"`
	RejectedCount     int64 `json:"rejected_count"`
	TotalAppeals      int64 `json:"total_appeals"`
	AvgResolutionTime string `json:"avg_resolution_time,omitempty"`
}

// ModerationPolicy defines the moderation policy
type ModerationPolicy struct {
	AcknowledgmentDeadline time.Duration `json:"acknowledgment_deadline"`
	ResolutionDeadline     time.Duration `json:"resolution_deadline"`
	AllowAnonymousReports  bool          `json:"allow_anonymous_reports"`
	RequireEvidence        bool          `json:"require_evidence"`
	AutoAcknowledge        bool          `json:"auto_acknowledge"`
	NotifyOnResolution     bool          `json:"notify_on_resolution"`
}

// DefaultModerationPolicy returns the default moderation policy
func DefaultModerationPolicy() *ModerationPolicy {
	return &ModerationPolicy{
		AcknowledgmentDeadline: 24 * time.Hour,
		ResolutionDeadline:     7 * 24 * time.Hour,
		AllowAnonymousReports:  false,
		RequireEvidence:        true,
		AutoAcknowledge:        true,
		NotifyOnResolution:     true,
	}
}

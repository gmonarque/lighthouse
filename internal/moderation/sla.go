// Package moderation handles reports and appeals
package moderation

import (
	"context"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

// SLAEnforcer monitors and enforces report SLAs
type SLAEnforcer struct {
	mu sync.RWMutex

	storage *Storage
	policy  *ModerationPolicy

	// Callbacks for SLA violations
	onViolation func(report *Report, violationType string)

	// State
	running bool
	ctx     context.Context
	cancel  context.CancelFunc

	// Check interval
	checkInterval time.Duration
}

// SLAViolation types
const (
	ViolationAcknowledgmentOverdue = "acknowledgment_overdue"
	ViolationResolutionOverdue     = "resolution_overdue"
)

// NewSLAEnforcer creates a new SLA enforcer
func NewSLAEnforcer(storage *Storage, policy *ModerationPolicy) *SLAEnforcer {
	if policy == nil {
		policy = DefaultModerationPolicy()
	}

	return &SLAEnforcer{
		storage:       storage,
		policy:        policy,
		checkInterval: 15 * time.Minute, // Check every 15 minutes
	}
}

// SetViolationCallback sets the callback for SLA violations
func (e *SLAEnforcer) SetViolationCallback(callback func(report *Report, violationType string)) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.onViolation = callback
}

// Start starts the SLA enforcement loop
func (e *SLAEnforcer) Start() error {
	e.mu.Lock()
	if e.running {
		e.mu.Unlock()
		return nil
	}
	e.running = true
	e.ctx, e.cancel = context.WithCancel(context.Background())
	e.mu.Unlock()

	log.Info().
		Dur("acknowledgment_deadline", e.policy.AcknowledgmentDeadline).
		Dur("resolution_deadline", e.policy.ResolutionDeadline).
		Dur("check_interval", e.checkInterval).
		Msg("Starting SLA enforcer")

	go e.enforcementLoop()
	return nil
}

// Stop stops the SLA enforcement loop
func (e *SLAEnforcer) Stop() {
	e.mu.Lock()
	defer e.mu.Unlock()

	if !e.running {
		return
	}

	e.running = false
	if e.cancel != nil {
		e.cancel()
	}

	log.Info().Msg("SLA enforcer stopped")
}

// enforcementLoop periodically checks for SLA violations
func (e *SLAEnforcer) enforcementLoop() {
	ticker := time.NewTicker(e.checkInterval)
	defer ticker.Stop()

	// Initial check
	e.checkViolations()

	for {
		select {
		case <-e.ctx.Done():
			return
		case <-ticker.C:
			e.checkViolations()
		}
	}
}

// checkViolations checks for all types of SLA violations
func (e *SLAEnforcer) checkViolations() {
	e.checkAcknowledgmentSLA()
	e.checkResolutionSLA()
}

// checkAcknowledgmentSLA checks for reports exceeding 24h acknowledgment SLA
func (e *SLAEnforcer) checkAcknowledgmentSLA() {
	reports, err := e.storage.GetNeedingAcknowledgment(e.policy.AcknowledgmentDeadline)
	if err != nil {
		log.Warn().Err(err).Msg("Failed to check acknowledgment SLA")
		return
	}

	for _, report := range reports {
		overdue := time.Since(report.CreatedAt) - e.policy.AcknowledgmentDeadline

		log.Warn().
			Str("report_id", report.ReportID).
			Str("category", string(report.Category)).
			Dur("overdue", overdue).
			Time("created_at", report.CreatedAt).
			Msg("Report exceeds 24h acknowledgment SLA")

		// Call violation callback if set
		e.mu.RLock()
		callback := e.onViolation
		e.mu.RUnlock()

		if callback != nil {
			callback(report, ViolationAcknowledgmentOverdue)
		}

		// Auto-acknowledge if policy allows
		if e.policy.AutoAcknowledge {
			report.Acknowledge()
			if err := e.storage.Save(report); err != nil {
				log.Error().Err(err).Str("report_id", report.ReportID).Msg("Failed to auto-acknowledge report")
			} else {
				log.Info().Str("report_id", report.ReportID).Msg("Auto-acknowledged overdue report")
			}
		}
	}

	if len(reports) > 0 {
		log.Warn().
			Int("count", len(reports)).
			Msg("Found reports violating acknowledgment SLA")
	}
}

// checkResolutionSLA checks for reports exceeding resolution deadline
func (e *SLAEnforcer) checkResolutionSLA() {
	// Get all open (acknowledged/investigating) reports
	open, err := e.storage.GetOpen()
	if err != nil {
		log.Warn().Err(err).Msg("Failed to check resolution SLA")
		return
	}

	violationCount := 0
	for _, report := range open {
		// Skip pending (handled by acknowledgment SLA)
		if report.Status == StatusPending {
			continue
		}

		// Check if exceeds resolution deadline
		if time.Since(report.CreatedAt) > e.policy.ResolutionDeadline {
			overdue := time.Since(report.CreatedAt) - e.policy.ResolutionDeadline
			violationCount++

			log.Warn().
				Str("report_id", report.ReportID).
				Str("status", string(report.Status)).
				Str("category", string(report.Category)).
				Dur("overdue", overdue).
				Msg("Report exceeds resolution SLA")

			// Call violation callback if set
			e.mu.RLock()
			callback := e.onViolation
			e.mu.RUnlock()

			if callback != nil {
				callback(report, ViolationResolutionOverdue)
			}
		}
	}

	if violationCount > 0 {
		log.Warn().
			Int("count", violationCount).
			Msg("Found reports violating resolution SLA")
	}
}

// GetSLAStatus returns the current SLA compliance status
func (e *SLAEnforcer) GetSLAStatus() (*SLAStatus, error) {
	status := &SLAStatus{
		AcknowledgmentDeadline: e.policy.AcknowledgmentDeadline,
		ResolutionDeadline:     e.policy.ResolutionDeadline,
	}

	// Get reports needing acknowledgment
	ackOverdue, err := e.storage.GetNeedingAcknowledgment(e.policy.AcknowledgmentDeadline)
	if err != nil {
		return nil, err
	}
	status.AcknowledgmentOverdue = len(ackOverdue)

	// Get pending reports within SLA
	pending, err := e.storage.GetPending()
	if err != nil {
		return nil, err
	}
	status.PendingWithinSLA = len(pending) - len(ackOverdue)
	if status.PendingWithinSLA < 0 {
		status.PendingWithinSLA = 0
	}

	// Get open reports for resolution SLA check
	open, err := e.storage.GetOpen()
	if err != nil {
		return nil, err
	}

	for _, report := range open {
		if report.Status != StatusPending {
			if time.Since(report.CreatedAt) > e.policy.ResolutionDeadline {
				status.ResolutionOverdue++
			} else {
				status.OpenWithinSLA++
			}
		}
	}

	// Calculate compliance percentage
	totalOpen := status.PendingWithinSLA + status.AcknowledgmentOverdue + status.OpenWithinSLA + status.ResolutionOverdue
	if totalOpen > 0 {
		compliant := status.PendingWithinSLA + status.OpenWithinSLA
		status.CompliancePercent = float64(compliant) / float64(totalOpen) * 100
	} else {
		status.CompliancePercent = 100.0 // No open reports = 100% compliance
	}

	return status, nil
}

// SLAStatus represents the current SLA compliance status
type SLAStatus struct {
	AcknowledgmentDeadline time.Duration `json:"acknowledgment_deadline"`
	ResolutionDeadline     time.Duration `json:"resolution_deadline"`
	PendingWithinSLA       int           `json:"pending_within_sla"`
	AcknowledgmentOverdue  int           `json:"acknowledgment_overdue"`
	OpenWithinSLA          int           `json:"open_within_sla"`
	ResolutionOverdue      int           `json:"resolution_overdue"`
	CompliancePercent      float64       `json:"compliance_percent"`
}

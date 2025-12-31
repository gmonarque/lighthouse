package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/gmonarque/lighthouse/internal/moderation"
	"github.com/go-chi/chi/v5"
)

// ReportResponse represents a report in API responses
type ReportResponse struct {
	ID             string `json:"id"`
	ReportID       string `json:"report_id"`
	Kind           string `json:"kind"`
	TargetEventID  string `json:"target_event_id,omitempty"`
	TargetInfohash string `json:"target_infohash,omitempty"`
	Category       string `json:"category"`
	Evidence       string `json:"evidence,omitempty"`
	Scope          string `json:"scope,omitempty"`
	Jurisdiction   string `json:"jurisdiction,omitempty"`
	ReporterPubkey string `json:"reporter_pubkey"`
	Status         string `json:"status"`
	Resolution     string `json:"resolution,omitempty"`
	CreatedAt      string `json:"created_at"`
	AcknowledgedAt string `json:"acknowledged_at,omitempty"`
	ResolvedAt     string `json:"resolved_at,omitempty"`
}

// reportStorage is the storage instance
var reportStorage *moderation.Storage

// SetReportStorage sets the report storage instance
func SetReportStorage(s *moderation.Storage) {
	reportStorage = s
}

// GetReports returns all reports with optional filtering
func GetReports(w http.ResponseWriter, r *http.Request) {
	if reportStorage == nil {
		respondJSON(w, http.StatusOK, map[string]interface{}{
			"reports": []ReportResponse{},
			"total":   0,
		})
		return
	}

	// Parse query params
	filter := &moderation.ReportFilter{}

	if status := r.URL.Query().Get("status"); status != "" {
		filter.Status = moderation.ReportStatus(status)
	}
	if category := r.URL.Query().Get("category"); category != "" {
		filter.Category = moderation.ReportCategory(category)
	}
	if infohash := r.URL.Query().Get("infohash"); infohash != "" {
		filter.TargetInfohash = infohash
	}

	if limit := r.URL.Query().Get("limit"); limit != "" {
		if l, err := strconv.Atoi(limit); err == nil {
			filter.Limit = l
		}
	} else {
		filter.Limit = 50
	}

	if offset := r.URL.Query().Get("offset"); offset != "" {
		if o, err := strconv.Atoi(offset); err == nil {
			filter.Offset = o
		}
	}

	reports, err := reportStorage.Query(filter)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to query reports")
		return
	}

	response := make([]ReportResponse, 0, len(reports))
	for _, rpt := range reports {
		rr := ReportResponse{
			ID:             rpt.ID,
			ReportID:       rpt.ReportID,
			Kind:           string(rpt.Kind),
			TargetEventID:  rpt.TargetEventID,
			TargetInfohash: rpt.TargetInfohash,
			Category:       string(rpt.Category),
			Evidence:       rpt.Evidence,
			Scope:          rpt.Scope,
			Jurisdiction:   rpt.Jurisdiction,
			ReporterPubkey: rpt.ReporterPubkey,
			Status:         string(rpt.Status),
			Resolution:     rpt.Resolution,
			CreatedAt:      rpt.CreatedAt.Format("2006-01-02T15:04:05Z"),
		}
		if rpt.AcknowledgedAt != nil {
			rr.AcknowledgedAt = rpt.AcknowledgedAt.Format("2006-01-02T15:04:05Z")
		}
		if rpt.ResolvedAt != nil {
			rr.ResolvedAt = rpt.ResolvedAt.Format("2006-01-02T15:04:05Z")
		}
		response = append(response, rr)
	}

	stats, _ := reportStorage.GetStats()

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"reports": response,
		"total":   len(response),
		"stats":   stats,
	})
}

// GetReport returns a specific report by ID
func GetReport(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		respondError(w, http.StatusBadRequest, "Report ID required")
		return
	}

	if reportStorage == nil {
		respondError(w, http.StatusNotFound, "Report not found")
		return
	}

	report, err := reportStorage.GetByID(id)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to get report")
		return
	}
	if report == nil {
		respondError(w, http.StatusNotFound, "Report not found")
		return
	}

	respondJSON(w, http.StatusOK, report)
}

// SubmitReportRequest is the request body for submitting a report
type SubmitReportRequest struct {
	Kind           string `json:"kind"` // "report" or "appeal"
	TargetEventID  string `json:"target_event_id,omitempty"`
	TargetInfohash string `json:"target_infohash,omitempty"`
	Category       string `json:"category"`
	Evidence       string `json:"evidence,omitempty"`
	Scope          string `json:"scope,omitempty"`
	Jurisdiction   string `json:"jurisdiction,omitempty"`
	ReporterPubkey string `json:"reporter_pubkey,omitempty"`
}

// SubmitReport submits a new report or appeal
func SubmitReport(w http.ResponseWriter, r *http.Request) {
	var req SubmitReportRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.Kind == "" {
		req.Kind = "report"
	}

	if req.Category == "" {
		respondError(w, http.StatusBadRequest, "Category required")
		return
	}

	if req.TargetEventID == "" && req.TargetInfohash == "" {
		respondError(w, http.StatusBadRequest, "Either target_event_id or target_infohash required")
		return
	}

	if reportStorage == nil {
		respondError(w, http.StatusInternalServerError, "Report storage not available")
		return
	}

	report := moderation.NewReport(
		moderation.ReportKind(req.Kind),
		moderation.ReportCategory(req.Category),
		req.ReporterPubkey,
	)
	report.TargetEventID = req.TargetEventID
	report.TargetInfohash = req.TargetInfohash
	report.Evidence = req.Evidence
	report.Scope = req.Scope
	report.Jurisdiction = req.Jurisdiction

	if err := reportStorage.Save(report); err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to save report: "+err.Error())
		return
	}

	respondJSON(w, http.StatusCreated, map[string]interface{}{
		"report_id": report.ReportID,
		"message":   "Report submitted successfully",
	})
}

// UpdateReportRequest is the request body for updating a report
type UpdateReportRequest struct {
	Status     string `json:"status,omitempty"`
	Resolution string `json:"resolution,omitempty"`
	ResolvedBy string `json:"resolved_by,omitempty"`
}

// UpdateReport updates a report's status
func UpdateReport(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		respondError(w, http.StatusBadRequest, "Report ID required")
		return
	}

	var req UpdateReportRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if reportStorage == nil {
		respondError(w, http.StatusInternalServerError, "Report storage not available")
		return
	}

	if req.Status != "" {
		if err := reportStorage.UpdateStatus(id, moderation.ReportStatus(req.Status), req.Resolution, req.ResolvedBy); err != nil {
			respondError(w, http.StatusInternalServerError, "Failed to update report: "+err.Error())
			return
		}
	}

	respondJSON(w, http.StatusOK, map[string]string{
		"message": "Report updated",
	})
}

// AcknowledgeReport marks a report as acknowledged
func AcknowledgeReport(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		respondError(w, http.StatusBadRequest, "Report ID required")
		return
	}

	if reportStorage == nil {
		respondError(w, http.StatusInternalServerError, "Report storage not available")
		return
	}

	if err := reportStorage.UpdateStatus(id, moderation.StatusAcknowledged, "", ""); err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to acknowledge report: "+err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{
		"message": "Report acknowledged",
	})
}

// GetPendingReports returns pending reports
func GetPendingReports(w http.ResponseWriter, r *http.Request) {
	if reportStorage == nil {
		respondJSON(w, http.StatusOK, map[string]interface{}{
			"reports": []ReportResponse{},
			"total":   0,
		})
		return
	}

	reports, err := reportStorage.GetPending()
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to get pending reports")
		return
	}

	response := make([]ReportResponse, 0, len(reports))
	for _, rpt := range reports {
		response = append(response, ReportResponse{
			ID:             rpt.ID,
			ReportID:       rpt.ReportID,
			Kind:           string(rpt.Kind),
			TargetEventID:  rpt.TargetEventID,
			TargetInfohash: rpt.TargetInfohash,
			Category:       string(rpt.Category),
			Status:         string(rpt.Status),
			CreatedAt:      rpt.CreatedAt.Format("2006-01-02T15:04:05Z"),
		})
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"reports": response,
		"total":   len(response),
	})
}

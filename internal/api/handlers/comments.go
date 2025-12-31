package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/gmonarque/lighthouse/internal/comments"
	"github.com/go-chi/chi/v5"
)

// CommentResponse represents a comment in API responses
type CommentResponse struct {
	ID             string   `json:"id"`
	EventID        string   `json:"event_id"`
	Infohash       string   `json:"infohash"`
	TorrentEventID string   `json:"torrent_event_id,omitempty"`
	AuthorPubkey   string   `json:"author_pubkey"`
	Content        string   `json:"content"`
	Rating         int      `json:"rating,omitempty"`
	ParentID       string   `json:"parent_id,omitempty"`
	RootID         string   `json:"root_id,omitempty"`
	Mentions       []string `json:"mentions,omitempty"`
	CreatedAt      string   `json:"created_at"`
}

// commentStorage is the storage instance
var commentStorage *comments.Storage

// SetCommentStorage sets the comment storage instance
func SetCommentStorage(s *comments.Storage) {
	commentStorage = s
}

// GetCommentsByInfohash returns comments for a specific infohash
func GetCommentsByInfohash(w http.ResponseWriter, r *http.Request) {
	infohash := chi.URLParam(r, "infohash")
	if infohash == "" {
		respondError(w, http.StatusBadRequest, "Infohash required")
		return
	}

	if commentStorage == nil {
		respondJSON(w, http.StatusOK, map[string]interface{}{
			"comments": []CommentResponse{},
			"stats":    nil,
		})
		return
	}

	limit := 50
	offset := 0

	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil {
			limit = parsed
		}
	}
	if o := r.URL.Query().Get("offset"); o != "" {
		if parsed, err := strconv.Atoi(o); err == nil {
			offset = parsed
		}
	}

	cmts, err := commentStorage.GetByInfohash(infohash, limit, offset)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to get comments")
		return
	}

	response := make([]CommentResponse, 0, len(cmts))
	for _, c := range cmts {
		response = append(response, CommentResponse{
			ID:             c.ID,
			EventID:        c.EventID,
			Infohash:       c.Infohash,
			TorrentEventID: c.TorrentEventID,
			AuthorPubkey:   c.AuthorPubkey,
			Content:        c.Content,
			Rating:         c.Rating,
			ParentID:       c.ParentID,
			RootID:         c.RootID,
			Mentions:       c.Mentions,
			CreatedAt:      c.CreatedAt.Format("2006-01-02T15:04:05Z"),
		})
	}

	stats, _ := commentStorage.GetStats(infohash)

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"comments": response,
		"stats":    stats,
	})
}

// GetComment returns a specific comment by event ID
func GetComment(w http.ResponseWriter, r *http.Request) {
	eventID := chi.URLParam(r, "eventId")
	if eventID == "" {
		respondError(w, http.StatusBadRequest, "Event ID required")
		return
	}

	if commentStorage == nil {
		respondError(w, http.StatusNotFound, "Comment not found")
		return
	}

	comment, err := commentStorage.GetByID(eventID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to get comment")
		return
	}
	if comment == nil {
		respondError(w, http.StatusNotFound, "Comment not found")
		return
	}

	respondJSON(w, http.StatusOK, CommentResponse{
		ID:             comment.ID,
		EventID:        comment.EventID,
		Infohash:       comment.Infohash,
		TorrentEventID: comment.TorrentEventID,
		AuthorPubkey:   comment.AuthorPubkey,
		Content:        comment.Content,
		Rating:         comment.Rating,
		ParentID:       comment.ParentID,
		RootID:         comment.RootID,
		Mentions:       comment.Mentions,
		CreatedAt:      comment.CreatedAt.Format("2006-01-02T15:04:05Z"),
	})
}

// AddCommentRequest is the request body for adding a comment
type AddCommentRequest struct {
	Infohash       string   `json:"infohash"`
	TorrentEventID string   `json:"torrent_event_id,omitempty"`
	Content        string   `json:"content"`
	Rating         int      `json:"rating,omitempty"`
	ParentID       string   `json:"parent_id,omitempty"`
	RootID         string   `json:"root_id,omitempty"`
	AuthorPubkey   string   `json:"author_pubkey,omitempty"`
	Mentions       []string `json:"mentions,omitempty"`
}

// AddComment adds a new comment
func AddComment(w http.ResponseWriter, r *http.Request) {
	var req AddCommentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Get infohash from URL if not in body
	if req.Infohash == "" {
		req.Infohash = chi.URLParam(r, "infohash")
	}

	if req.Infohash == "" {
		respondError(w, http.StatusBadRequest, "Infohash required")
		return
	}

	if req.Content == "" {
		respondError(w, http.StatusBadRequest, "Content required")
		return
	}

	if commentStorage == nil {
		respondError(w, http.StatusInternalServerError, "Comment storage not available")
		return
	}

	comment := comments.NewComment(req.Infohash, req.Content, req.AuthorPubkey)
	if req.TorrentEventID != "" {
		comment.SetTorrentEvent(req.TorrentEventID)
	}
	if req.Rating > 0 {
		comment.SetRating(req.Rating)
	}
	if req.ParentID != "" {
		comment.SetParent(req.ParentID, req.RootID)
	}
	comment.Mentions = req.Mentions

	if err := commentStorage.Save(comment); err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to save comment: "+err.Error())
		return
	}

	respondJSON(w, http.StatusCreated, map[string]interface{}{
		"event_id": comment.EventID,
		"message":  "Comment added successfully",
	})
}

// GetCommentThread returns a comment thread
func GetCommentThread(w http.ResponseWriter, r *http.Request) {
	eventID := chi.URLParam(r, "eventId")
	if eventID == "" {
		respondError(w, http.StatusBadRequest, "Event ID required")
		return
	}

	if commentStorage == nil {
		respondError(w, http.StatusNotFound, "Thread not found")
		return
	}

	thread, err := commentStorage.GetThread(eventID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to get thread: "+err.Error())
		return
	}

	respondJSON(w, http.StatusOK, thread)
}

// GetCommentStats returns comment statistics for an infohash
func GetCommentStats(w http.ResponseWriter, r *http.Request) {
	infohash := chi.URLParam(r, "infohash")
	if infohash == "" {
		respondError(w, http.StatusBadRequest, "Infohash required")
		return
	}

	if commentStorage == nil {
		respondJSON(w, http.StatusOK, &comments.CommentStats{
			RatingCounts: make(map[int]int),
		})
		return
	}

	stats, err := commentStorage.GetStats(infohash)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to get stats")
		return
	}

	respondJSON(w, http.StatusOK, stats)
}

// DeleteComment deletes a comment
func DeleteComment(w http.ResponseWriter, r *http.Request) {
	eventID := chi.URLParam(r, "eventId")
	if eventID == "" {
		respondError(w, http.StatusBadRequest, "Event ID required")
		return
	}

	if commentStorage == nil {
		respondError(w, http.StatusNotFound, "Comment not found")
		return
	}

	if err := commentStorage.Delete(eventID); err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to delete comment: "+err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{
		"message": "Comment deleted",
	})
}

// GetRecentComments returns recent comments across all torrents
func GetRecentComments(w http.ResponseWriter, r *http.Request) {
	if commentStorage == nil {
		respondJSON(w, http.StatusOK, map[string]interface{}{
			"comments": []CommentResponse{},
			"total":    0,
		})
		return
	}

	limit := 20
	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil {
			limit = parsed
		}
	}

	cmts, err := commentStorage.GetRecentComments(limit)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to get comments")
		return
	}

	response := make([]CommentResponse, 0, len(cmts))
	for _, c := range cmts {
		response = append(response, CommentResponse{
			ID:             c.ID,
			EventID:        c.EventID,
			Infohash:       c.Infohash,
			TorrentEventID: c.TorrentEventID,
			AuthorPubkey:   c.AuthorPubkey,
			Content:        c.Content,
			Rating:         c.Rating,
			ParentID:       c.ParentID,
			RootID:         c.RootID,
			Mentions:       c.Mentions,
			CreatedAt:      c.CreatedAt.Format("2006-01-02T15:04:05Z"),
		})
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"comments": response,
		"total":    len(response),
	})
}

package comments

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/gmonarque/lighthouse/internal/database"
	"github.com/rs/zerolog/log"
)

// Storage handles comment persistence
type Storage struct{}

// NewStorage creates a new comment storage
func NewStorage() *Storage {
	return &Storage{}
}

// Save saves a comment to the database
func (s *Storage) Save(c *Comment) error {
	db := database.Get()

	mentionsJSON, _ := json.Marshal(c.Mentions)

	_, err := db.Exec(`
		INSERT INTO torrent_comments (
			event_id, infohash, torrent_event_id, author_pubkey,
			content, rating, parent_id, root_id, mentions, created_at, signature
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(event_id) DO UPDATE SET
			content = excluded.content,
			rating = excluded.rating
	`, c.EventID, c.Infohash, c.TorrentEventID, c.AuthorPubkey,
		c.Content, c.Rating, c.ParentID, c.RootID,
		string(mentionsJSON), c.CreatedAt.Format(time.RFC3339), c.Signature)

	if err != nil {
		return fmt.Errorf("failed to save comment: %w", err)
	}

	// Update torrent comment count
	s.updateCommentCount(c.Infohash)

	log.Debug().
		Str("event_id", c.EventID).
		Str("infohash", c.Infohash).
		Msg("Saved comment")

	return nil
}

// updateCommentCount updates the comment count for a torrent
func (s *Storage) updateCommentCount(infohash string) {
	db := database.Get()

	db.Exec(`
		UPDATE torrents SET comment_count = (
			SELECT COUNT(*) FROM torrent_comments WHERE infohash = ?
		) WHERE info_hash = ?
	`, infohash, infohash)
}

// GetByID retrieves a comment by event ID
func (s *Storage) GetByID(eventID string) (*Comment, error) {
	db := database.Get()

	var c Comment
	var mentionsJSON sql.NullString
	var createdAt string
	var parentID, rootID, torrentEventID sql.NullString
	var signature sql.NullString

	err := db.QueryRow(`
		SELECT event_id, infohash, torrent_event_id, author_pubkey,
			   content, rating, parent_id, root_id, mentions, created_at, signature
		FROM torrent_comments WHERE event_id = ?
	`, eventID).Scan(&c.EventID, &c.Infohash, &torrentEventID, &c.AuthorPubkey,
		&c.Content, &c.Rating, &parentID, &rootID, &mentionsJSON, &createdAt, &signature)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get comment: %w", err)
	}

	if torrentEventID.Valid {
		c.TorrentEventID = torrentEventID.String
	}
	if parentID.Valid {
		c.ParentID = parentID.String
	}
	if rootID.Valid {
		c.RootID = rootID.String
	}
	if mentionsJSON.Valid {
		json.Unmarshal([]byte(mentionsJSON.String), &c.Mentions)
	}
	if signature.Valid {
		c.Signature = signature.String
	}
	c.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)

	return &c, nil
}

// GetByInfohash retrieves all comments for an infohash
func (s *Storage) GetByInfohash(infohash string, limit, offset int) ([]*Comment, error) {
	return s.Query(&CommentFilter{
		Infohash: infohash,
		Limit:    limit,
		Offset:   offset,
	})
}

// GetByTorrentEvent retrieves all comments for a torrent event
func (s *Storage) GetByTorrentEvent(eventID string, limit, offset int) ([]*Comment, error) {
	return s.Query(&CommentFilter{
		TorrentEvent: eventID,
		Limit:        limit,
		Offset:       offset,
	})
}

// GetReplies retrieves replies to a comment
func (s *Storage) GetReplies(parentID string) ([]*Comment, error) {
	return s.Query(&CommentFilter{
		ParentID: parentID,
	})
}

// Query queries comments with filter
func (s *Storage) Query(filter *CommentFilter) ([]*Comment, error) {
	db := database.Get()

	query := `
		SELECT event_id, infohash, torrent_event_id, author_pubkey,
			   content, rating, parent_id, root_id, mentions, created_at, signature
		FROM torrent_comments WHERE 1=1
	`
	args := []interface{}{}

	if filter.Infohash != "" {
		query += " AND infohash = ?"
		args = append(args, filter.Infohash)
	}
	if filter.TorrentEvent != "" {
		query += " AND torrent_event_id = ?"
		args = append(args, filter.TorrentEvent)
	}
	if filter.AuthorPubkey != "" {
		query += " AND author_pubkey = ?"
		args = append(args, filter.AuthorPubkey)
	}
	if filter.ParentID != "" {
		query += " AND parent_id = ?"
		args = append(args, filter.ParentID)
	}
	if filter.HasRating {
		query += " AND rating > 0"
	}
	if filter.MinRating > 0 {
		query += " AND rating >= ?"
		args = append(args, filter.MinRating)
	}
	if filter.Since != nil {
		query += " AND created_at >= ?"
		args = append(args, filter.Since.Format(time.RFC3339))
	}
	if filter.Until != nil {
		query += " AND created_at <= ?"
		args = append(args, filter.Until.Format(time.RFC3339))
	}

	query += " ORDER BY created_at DESC"

	if filter.Limit > 0 {
		query += fmt.Sprintf(" LIMIT %d", filter.Limit)
	}
	if filter.Offset > 0 {
		query += fmt.Sprintf(" OFFSET %d", filter.Offset)
	}

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query comments: %w", err)
	}
	defer rows.Close()

	var comments []*Comment
	for rows.Next() {
		var c Comment
		var mentionsJSON sql.NullString
		var createdAt string
		var parentID, rootID, torrentEventID sql.NullString
		var signature sql.NullString

		err := rows.Scan(&c.EventID, &c.Infohash, &torrentEventID, &c.AuthorPubkey,
			&c.Content, &c.Rating, &parentID, &rootID, &mentionsJSON, &createdAt, &signature)
		if err != nil {
			continue
		}

		if torrentEventID.Valid {
			c.TorrentEventID = torrentEventID.String
		}
		if parentID.Valid {
			c.ParentID = parentID.String
		}
		if rootID.Valid {
			c.RootID = rootID.String
		}
		if mentionsJSON.Valid {
			json.Unmarshal([]byte(mentionsJSON.String), &c.Mentions)
		}
		if signature.Valid {
			c.Signature = signature.String
		}
		c.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)

		comments = append(comments, &c)
	}

	return comments, nil
}

// GetThread retrieves a comment thread
func (s *Storage) GetThread(rootID string) (*CommentThread, error) {
	// Get root comment
	root, err := s.GetByID(rootID)
	if err != nil {
		return nil, err
	}
	if root == nil {
		return nil, fmt.Errorf("root comment not found")
	}

	thread := &CommentThread{
		Root: root,
	}

	// Get all replies
	replies, err := s.Query(&CommentFilter{ParentID: rootID})
	if err != nil {
		return thread, nil
	}

	thread.Replies = replies
	return thread, nil
}

// Delete deletes a comment
func (s *Storage) Delete(eventID string) error {
	db := database.Get()

	// Get infohash first for updating count
	var infohash string
	db.QueryRow("SELECT infohash FROM torrent_comments WHERE event_id = ?", eventID).Scan(&infohash)

	result, err := db.Exec("DELETE FROM torrent_comments WHERE event_id = ?", eventID)
	if err != nil {
		return fmt.Errorf("failed to delete comment: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("comment not found")
	}

	// Update comment count
	if infohash != "" {
		s.updateCommentCount(infohash)
	}

	return nil
}

// DeleteByAuthor deletes all comments from an author
func (s *Storage) DeleteByAuthor(pubkey string) (int64, error) {
	db := database.Get()

	// Get affected infohashes
	rows, err := db.Query("SELECT DISTINCT infohash FROM torrent_comments WHERE author_pubkey = ?", pubkey)
	if err != nil {
		return 0, err
	}
	var infohashes []string
	for rows.Next() {
		var ih string
		if rows.Scan(&ih) == nil {
			infohashes = append(infohashes, ih)
		}
	}
	rows.Close()

	result, err := db.Exec("DELETE FROM torrent_comments WHERE author_pubkey = ?", pubkey)
	if err != nil {
		return 0, fmt.Errorf("failed to delete comments: %w", err)
	}

	deleted, _ := result.RowsAffected()

	// Update comment counts
	for _, ih := range infohashes {
		s.updateCommentCount(ih)
	}

	return deleted, nil
}

// GetStats returns comment statistics for an infohash
func (s *Storage) GetStats(infohash string) (*CommentStats, error) {
	db := database.Get()

	stats := &CommentStats{
		RatingCounts: make(map[int]int),
	}

	// Total comments
	db.QueryRow(`
		SELECT COUNT(*) FROM torrent_comments WHERE infohash = ?
	`, infohash).Scan(&stats.TotalComments)

	// Ratings
	rows, err := db.Query(`
		SELECT rating, COUNT(*) FROM torrent_comments
		WHERE infohash = ? AND rating > 0
		GROUP BY rating
	`, infohash)
	if err != nil {
		return stats, nil
	}
	defer rows.Close()

	totalRatingSum := 0
	for rows.Next() {
		var rating, count int
		if err := rows.Scan(&rating, &count); err == nil {
			stats.RatingCounts[rating] = count
			stats.TotalRatings += count
			totalRatingSum += rating * count
		}
	}

	if stats.TotalRatings > 0 {
		stats.AverageRating = float64(totalRatingSum) / float64(stats.TotalRatings)
	}

	return stats, nil
}

// GetRecentComments returns recent comments across all torrents
func (s *Storage) GetRecentComments(limit int) ([]*Comment, error) {
	return s.Query(&CommentFilter{
		Limit: limit,
	})
}

// Count returns total comment count
func (s *Storage) Count() int64 {
	db := database.Get()

	var count int64
	db.QueryRow("SELECT COUNT(*) FROM torrent_comments").Scan(&count)
	return count
}

// CountByInfohash returns comment count for an infohash
func (s *Storage) CountByInfohash(infohash string) int64 {
	db := database.Get()

	var count int64
	db.QueryRow("SELECT COUNT(*) FROM torrent_comments WHERE infohash = ?", infohash).Scan(&count)
	return count
}

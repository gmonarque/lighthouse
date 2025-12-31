package relay

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"github.com/gmonarque/lighthouse/internal/database"
	"github.com/rs/zerolog/log"
)

// EventStorage handles relay event persistence
type EventStorage struct {
	mu    sync.RWMutex
	cache map[string]*Event // In-memory cache for recent events
}

// NewEventStorage creates a new event storage
func NewEventStorage() *EventStorage {
	return &EventStorage{
		cache: make(map[string]*Event),
	}
}

// Save saves an event to storage
func (s *EventStorage) Save(event *Event) error {
	db := database.Get()

	// Serialize event
	eventJSON, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal event: %w", err)
	}

	// Serialize tags
	tagsJSON, err := json.Marshal(event.Tags)
	if err != nil {
		return fmt.Errorf("failed to marshal tags: %w", err)
	}

	// Extract searchable fields
	infohash := event.GetInfohash()
	dTag := event.GetTagValue("d")

	_, err = db.Exec(`
		INSERT INTO relay_events (
			event_id, pubkey, kind, created_at, content, tags_json, sig,
			infohash, d_tag, raw_json
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(event_id) DO UPDATE SET
			received_at = CURRENT_TIMESTAMP
	`, event.ID, event.PubKey, event.Kind, event.CreatedAt, event.Content,
		string(tagsJSON), event.Sig, infohash, dTag, string(eventJSON))

	if err != nil {
		return fmt.Errorf("failed to save event: %w", err)
	}

	// Add to cache
	s.mu.Lock()
	s.cache[event.ID] = event
	s.mu.Unlock()

	log.Debug().
		Str("event_id", event.ID).
		Int("kind", event.Kind).
		Msg("Saved relay event")

	return nil
}

// Get retrieves an event by ID
func (s *EventStorage) Get(eventID string) (*Event, error) {
	// Check cache first
	s.mu.RLock()
	if event, ok := s.cache[eventID]; ok {
		s.mu.RUnlock()
		return event, nil
	}
	s.mu.RUnlock()

	db := database.Get()

	var rawJSON string
	err := db.QueryRow(`
		SELECT raw_json FROM relay_events WHERE event_id = ?
	`, eventID).Scan(&rawJSON)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get event: %w", err)
	}

	var event Event
	if err := json.Unmarshal([]byte(rawJSON), &event); err != nil {
		return nil, fmt.Errorf("failed to unmarshal event: %w", err)
	}

	// Add to cache
	s.mu.Lock()
	s.cache[event.ID] = &event
	s.mu.Unlock()

	return &event, nil
}

// Query queries events matching filters
func (s *EventStorage) Query(filters []Filter) []*Event {
	if len(filters) == 0 {
		return nil
	}

	db := database.Get()
	var allEvents []*Event

	for _, filter := range filters {
		events := s.queryFilter(db, filter)
		allEvents = append(allEvents, events...)
	}

	// Remove duplicates
	seen := make(map[string]bool)
	var unique []*Event
	for _, event := range allEvents {
		if !seen[event.ID] {
			seen[event.ID] = true
			unique = append(unique, event)
		}
	}

	// Sort by created_at descending
	SortEventsByCreatedAt(unique)

	return unique
}

// queryFilter queries events for a single filter
func (s *EventStorage) queryFilter(db *sql.DB, filter Filter) []*Event {
	query := "SELECT raw_json FROM relay_events WHERE 1=1"
	args := []interface{}{}

	// Build query based on filter
	if len(filter.IDs) > 0 {
		placeholders := make([]string, len(filter.IDs))
		for i, id := range filter.IDs {
			if len(id) == 64 {
				placeholders[i] = "?"
				args = append(args, id)
			} else {
				// Prefix match
				placeholders[i] = "event_id LIKE ?"
				args = append(args, id+"%")
			}
		}
		if len(filter.IDs) == 1 && len(filter.IDs[0]) == 64 {
			query += " AND event_id = ?"
		} else {
			query += " AND (" + strings.Join(placeholders, " OR ") + ")"
		}
	}

	if len(filter.Authors) > 0 {
		placeholders := make([]string, len(filter.Authors))
		for i, author := range filter.Authors {
			if len(author) == 64 {
				placeholders[i] = "pubkey = ?"
			} else {
				placeholders[i] = "pubkey LIKE ?"
				author = author + "%"
			}
			args = append(args, author)
		}
		query += " AND (" + strings.Join(placeholders, " OR ") + ")"
	}

	if len(filter.Kinds) > 0 {
		placeholders := make([]string, len(filter.Kinds))
		for i, kind := range filter.Kinds {
			placeholders[i] = "?"
			args = append(args, kind)
		}
		query += " AND kind IN (" + strings.Join(placeholders, ",") + ")"
	}

	if filter.Since > 0 {
		query += " AND created_at >= ?"
		args = append(args, filter.Since)
	}

	if filter.Until > 0 {
		query += " AND created_at <= ?"
		args = append(args, filter.Until)
	}

	// Tag filters
	for tagName, values := range filter.Tags {
		if len(values) == 0 {
			continue
		}

		// Special handling for common tags
		switch tagName {
		case "e":
			// Event references - search in tags_json
			for _, v := range values {
				query += " AND tags_json LIKE ?"
				args = append(args, fmt.Sprintf(`%%["e","%s"%%`, v))
			}
		case "p":
			// Pubkey references
			for _, v := range values {
				query += " AND tags_json LIKE ?"
				args = append(args, fmt.Sprintf(`%%["p","%s"%%`, v))
			}
		case "x", "btih":
			// Infohash search
			if len(values) > 0 {
				placeholders := make([]string, len(values))
				for i := range values {
					placeholders[i] = "?"
					args = append(args, values[i])
				}
				query += " AND infohash IN (" + strings.Join(placeholders, ",") + ")"
			}
		case "d":
			// Parameterized replaceable identifier
			if len(values) > 0 {
				placeholders := make([]string, len(values))
				for i := range values {
					placeholders[i] = "?"
					args = append(args, values[i])
				}
				query += " AND d_tag IN (" + strings.Join(placeholders, ",") + ")"
			}
		default:
			// Generic tag search
			for _, v := range values {
				query += " AND tags_json LIKE ?"
				args = append(args, fmt.Sprintf(`%%["%s","%s"%%`, tagName, v))
			}
		}
	}

	query += " ORDER BY created_at DESC"

	if filter.Limit > 0 {
		query += fmt.Sprintf(" LIMIT %d", filter.Limit)
	} else {
		query += " LIMIT 500" // Default limit
	}

	rows, err := db.Query(query, args...)
	if err != nil {
		log.Error().Err(err).Str("query", query).Msg("Query failed")
		return nil
	}
	defer rows.Close()

	var events []*Event
	for rows.Next() {
		var rawJSON string
		if err := rows.Scan(&rawJSON); err != nil {
			continue
		}

		var event Event
		if err := json.Unmarshal([]byte(rawJSON), &event); err != nil {
			continue
		}

		events = append(events, &event)
	}

	return events
}

// Delete deletes an event
func (s *EventStorage) Delete(eventID string) error {
	db := database.Get()

	_, err := db.Exec("DELETE FROM relay_events WHERE event_id = ?", eventID)
	if err != nil {
		return fmt.Errorf("failed to delete event: %w", err)
	}

	s.mu.Lock()
	delete(s.cache, eventID)
	s.mu.Unlock()

	return nil
}

// DeleteByPubkey deletes all events from a pubkey
func (s *EventStorage) DeleteByPubkey(pubkey string) (int64, error) {
	db := database.Get()

	result, err := db.Exec("DELETE FROM relay_events WHERE pubkey = ?", pubkey)
	if err != nil {
		return 0, fmt.Errorf("failed to delete events: %w", err)
	}

	deleted, _ := result.RowsAffected()

	// Clear cache entries for this pubkey
	s.mu.Lock()
	for id, event := range s.cache {
		if event.PubKey == pubkey {
			delete(s.cache, id)
		}
	}
	s.mu.Unlock()

	return deleted, nil
}

// DeleteByInfohash deletes all events with an infohash
func (s *EventStorage) DeleteByInfohash(infohash string) (int64, error) {
	db := database.Get()

	result, err := db.Exec("DELETE FROM relay_events WHERE infohash = ?", infohash)
	if err != nil {
		return 0, fmt.Errorf("failed to delete events: %w", err)
	}

	deleted, _ := result.RowsAffected()
	return deleted, nil
}

// Count returns the total number of events
func (s *EventStorage) Count() int64 {
	db := database.Get()

	var count int64
	db.QueryRow("SELECT COUNT(*) FROM relay_events").Scan(&count)
	return count
}

// CountByKind returns event count by kind
func (s *EventStorage) CountByKind() map[int]int64 {
	db := database.Get()

	rows, err := db.Query("SELECT kind, COUNT(*) FROM relay_events GROUP BY kind")
	if err != nil {
		return nil
	}
	defer rows.Close()

	counts := make(map[int]int64)
	for rows.Next() {
		var kind int
		var count int64
		if err := rows.Scan(&kind, &count); err == nil {
			counts[kind] = count
		}
	}

	return counts
}

// Cleanup removes old events
func (s *EventStorage) Cleanup(maxAge int64) (int64, error) {
	db := database.Get()

	cutoff := maxAge
	result, err := db.Exec(`
		DELETE FROM relay_events WHERE created_at < ? AND kind < 10000
	`, cutoff)
	if err != nil {
		return 0, err
	}

	deleted, _ := result.RowsAffected()
	return deleted, nil
}

// GetByInfohash returns events for an infohash
func (s *EventStorage) GetByInfohash(infohash string) ([]*Event, error) {
	filters := []Filter{{
		Tags: map[string][]string{
			"x": {infohash},
		},
	}}
	return s.Query(filters), nil
}

// GetLatestByAuthor returns the latest event from an author
func (s *EventStorage) GetLatestByAuthor(pubkey string, kind int) (*Event, error) {
	db := database.Get()

	var rawJSON string
	err := db.QueryRow(`
		SELECT raw_json FROM relay_events
		WHERE pubkey = ? AND kind = ?
		ORDER BY created_at DESC
		LIMIT 1
	`, pubkey, kind).Scan(&rawJSON)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	var event Event
	if err := json.Unmarshal([]byte(rawJSON), &event); err != nil {
		return nil, err
	}

	return &event, nil
}

package torznab

import (
	"database/sql"
	"strconv"
	"strings"
	"time"

	"github.com/gmonarque/lighthouse/internal/database"
)

// Service handles Torznab API logic
type Service struct{}

// NewService creates a new Torznab service
func NewService() *Service {
	return &Service{}
}

// Search performs a Torznab search
func (s *Service) Search(params SearchParams) ([]SearchResult, int, error) {
	db := database.Get()

	query := `
		SELECT t.id, t.info_hash, t.name, t.size, t.category, t.seeders, t.leechers,
			   t.magnet_uri, t.title, t.year, t.tmdb_id, t.imdb_id, t.poster_url,
			   t.overview, t.first_seen_at
		FROM torrents t
	`
	countQuery := "SELECT COUNT(*) FROM torrents t"

	var conditions []string
	var args []interface{}

	// Text search
	if params.Query != "" {
		query = `
			SELECT t.id, t.info_hash, t.name, t.size, t.category, t.seeders, t.leechers,
				   t.magnet_uri, t.title, t.year, t.tmdb_id, t.imdb_id, t.poster_url,
				   t.overview, t.first_seen_at
			FROM torrents t
			JOIN torrents_fts fts ON t.id = fts.rowid
			WHERE torrents_fts MATCH ?
		`
		countQuery = `
			SELECT COUNT(*) FROM torrents t
			JOIN torrents_fts fts ON t.id = fts.rowid
			WHERE torrents_fts MATCH ?
		`
		args = append(args, params.Query)
	}

	// Category filter
	if len(params.Categories) > 0 {
		catConditions := make([]string, len(params.Categories))
		for i, cat := range params.Categories {
			if cat%1000 == 0 {
				// Main category - include all subcategories
				catConditions[i] = "(t.category >= ? AND t.category < ?)"
				args = append(args, cat, cat+1000)
			} else {
				catConditions[i] = "t.category = ?"
				args = append(args, cat)
			}
		}
		conditions = append(conditions, "("+strings.Join(catConditions, " OR ")+")")
	}

	// IMDB ID filter
	if params.ImdbID != "" {
		conditions = append(conditions, "t.imdb_id = ?")
		args = append(args, params.ImdbID)
	}

	// TMDB ID filter
	if params.TmdbID > 0 {
		conditions = append(conditions, "t.tmdb_id = ?")
		args = append(args, params.TmdbID)
	}

	// Season/Episode filter (for TV)
	if params.Season > 0 && params.Query != "" {
		// Append season to search query
		seasonStr := strconv.Itoa(params.Season)
		if params.Season < 10 {
			seasonStr = "0" + seasonStr
		}
		// This is a simplification - in production you'd want more sophisticated matching
	}

	// Build WHERE clause
	if params.Query == "" && len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
		countQuery += " WHERE " + strings.Join(conditions, " AND ")
	} else if params.Query != "" && len(conditions) > 0 {
		query += " AND " + strings.Join(conditions, " AND ")
		countQuery += " AND " + strings.Join(conditions, " AND ")
	}

	// Order by trust score and date
	query += " ORDER BY t.trust_score DESC, t.first_seen_at DESC"

	// Pagination
	query += " LIMIT ? OFFSET ?"
	args = append(args, params.Limit, params.Offset)

	// Execute query
	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var results []SearchResult
	for rows.Next() {
		var r SearchResult
		var id int64
		var name string
		var size, category, seeders, leechers sql.NullInt64
		var title, imdbID, posterURL, overview sql.NullString
		var year, tmdbID sql.NullInt64
		var firstSeenAt string

		err := rows.Scan(&id, &r.InfoHash, &name, &size, &category, &seeders, &leechers,
			&r.MagnetURI, &title, &year, &tmdbID, &imdbID, &posterURL, &overview, &firstSeenAt)
		if err != nil {
			continue
		}

		// Use clean title if available, otherwise use name
		r.Title = title.String
		if r.Title == "" {
			r.Title = name
		}

		r.GUID = r.InfoHash
		r.Size = size.Int64
		r.Category = int(category.Int64)
		r.Seeders = int(seeders.Int64)
		r.Leechers = int(leechers.Int64)
		r.Year = int(year.Int64)
		r.TmdbID = int(tmdbID.Int64)
		r.ImdbID = imdbID.String
		r.PosterURL = posterURL.String
		r.Description = overview.String

		// Parse date
		if t, err := time.Parse("2006-01-02 15:04:05", firstSeenAt); err == nil {
			r.PubDate = t
		}

		results = append(results, r)
	}

	// Get total count
	var total int
	countArgs := args[:len(args)-2] // Remove LIMIT and OFFSET
	db.QueryRow(countQuery, countArgs...).Scan(&total)

	return results, total, nil
}

// SearchParams represents Torznab search parameters
type SearchParams struct {
	Query      string
	Categories []int
	ImdbID     string
	TmdbID     int
	Season     int
	Episode    int
	Limit      int
	Offset     int
}

// ParseCategories parses a comma-separated category string
func ParseCategories(catStr string) []int {
	if catStr == "" {
		return nil
	}

	parts := strings.Split(catStr, ",")
	cats := make([]int, 0, len(parts))

	for _, p := range parts {
		if cat, err := strconv.Atoi(strings.TrimSpace(p)); err == nil {
			cats = append(cats, cat)
		}
	}

	return cats
}

// NormalizeImdbID ensures IMDB ID has correct format (tt1234567)
func NormalizeImdbID(id string) string {
	id = strings.TrimSpace(id)
	if id == "" {
		return ""
	}

	// Remove "tt" prefix if present
	id = strings.TrimPrefix(id, "tt")

	// Add "tt" prefix back
	return "tt" + id
}

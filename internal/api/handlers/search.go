package handlers

import (
	"database/sql"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/lighthouse-client/lighthouse/internal/database"
)

// Search performs full-text search on torrents
func Search(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	category := r.URL.Query().Get("category")
	limitParam := r.URL.Query().Get("limit")
	offsetParam := r.URL.Query().Get("offset")

	limit := 50
	if limitParam != "" {
		if l, err := strconv.Atoi(limitParam); err == nil && l > 0 && l <= 100 {
			limit = l
		}
	}

	offset := 0
	if offsetParam != "" {
		if o, err := strconv.Atoi(offsetParam); err == nil && o >= 0 {
			offset = o
		}
	}

	// Parse category and determine if it's a base category (e.g., 2000) or subcategory (e.g., 2045)
	var categoryNum int
	var isBaseCategory bool
	if category != "" {
		if c, err := strconv.Atoi(category); err == nil {
			categoryNum = c
			// Base categories end in 000 (1000, 2000, 3000, etc.)
			isBaseCategory = c%1000 == 0
		}
	}

	db := database.Get()

	var rows *sql.Rows
	var err error

	if query != "" {
		// Full-text search
		sqlQuery := `
			SELECT t.id, t.info_hash, t.name, t.size, t.category, t.seeders, t.leechers,
				   t.magnet_uri, t.title, t.year, t.poster_url, t.overview, t.trust_score, t.first_seen_at
			FROM torrents t
			JOIN torrents_fts fts ON t.id = fts.rowid
			WHERE torrents_fts MATCH ?
		`
		args := []interface{}{query}

		if category != "" {
			if isBaseCategory {
				// Match all subcategories within the base category range (e.g., 2000-2999)
				sqlQuery += " AND t.category >= ? AND t.category < ?"
				args = append(args, categoryNum, categoryNum+1000)
			} else {
				// Exact match for subcategory
				sqlQuery += " AND t.category = ?"
				args = append(args, categoryNum)
			}
		}

		sqlQuery += " ORDER BY t.trust_score DESC, t.first_seen_at DESC LIMIT ? OFFSET ?"
		args = append(args, limit, offset)

		rows, err = db.Query(sqlQuery, args...)
	} else {
		// List all (no search query)
		sqlQuery := `
			SELECT id, info_hash, name, size, category, seeders, leechers,
				   magnet_uri, title, year, poster_url, overview, trust_score, first_seen_at
			FROM torrents
			WHERE 1=1
		`
		args := []interface{}{}

		if category != "" {
			if isBaseCategory {
				// Match all subcategories within the base category range
				sqlQuery += " AND category >= ? AND category < ?"
				args = append(args, categoryNum, categoryNum+1000)
			} else {
				// Exact match for subcategory
				sqlQuery += " AND category = ?"
				args = append(args, categoryNum)
			}
		}

		sqlQuery += " ORDER BY trust_score DESC, first_seen_at DESC LIMIT ? OFFSET ?"
		args = append(args, limit, offset)

		rows, err = db.Query(sqlQuery, args...)
	}

	if err != nil {
		respondError(w, http.StatusInternalServerError, "Search failed")
		return
	}
	defer rows.Close()

	torrents := []map[string]interface{}{}
	for rows.Next() {
		var id int64
		var infoHash, name, magnetURI, firstSeenAt string
		var size, category, seeders, leechers sql.NullInt64
		var title, posterURL, overview sql.NullString
		var year sql.NullInt64
		var trustScore int64

		if err := rows.Scan(&id, &infoHash, &name, &size, &category, &seeders, &leechers,
			&magnetURI, &title, &year, &posterURL, &overview, &trustScore, &firstSeenAt); err != nil {
			continue
		}

		torrents = append(torrents, map[string]interface{}{
			"id":            id,
			"info_hash":     infoHash,
			"name":          name,
			"size":          size.Int64,
			"category":      category.Int64,
			"seeders":       seeders.Int64,
			"leechers":      leechers.Int64,
			"magnet_uri":    magnetURI,
			"title":         title.String,
			"year":          year.Int64,
			"poster_url":    posterURL.String,
			"overview":      overview.String,
			"trust_score":   trustScore,
			"first_seen_at": firstSeenAt,
		})
	}

	// Get total count for pagination
	var total int64
	if query != "" {
		countQuery := `
			SELECT COUNT(*) FROM torrents t
			JOIN torrents_fts fts ON t.id = fts.rowid
			WHERE torrents_fts MATCH ?
		`
		countArgs := []interface{}{query}
		if category != "" {
			if isBaseCategory {
				countQuery += " AND t.category >= ? AND t.category < ?"
				countArgs = append(countArgs, categoryNum, categoryNum+1000)
			} else {
				countQuery += " AND t.category = ?"
				countArgs = append(countArgs, categoryNum)
			}
		}
		db.QueryRow(countQuery, countArgs...).Scan(&total)
	} else {
		countQuery := "SELECT COUNT(*) FROM torrents WHERE 1=1"
		countArgs := []interface{}{}
		if category != "" {
			if isBaseCategory {
				countQuery += " AND category >= ? AND category < ?"
				countArgs = append(countArgs, categoryNum, categoryNum+1000)
			} else {
				countQuery += " AND category = ?"
				countArgs = append(countArgs, categoryNum)
			}
		}
		db.QueryRow(countQuery, countArgs...).Scan(&total)
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"results": torrents,
		"total":   total,
		"limit":   limit,
		"offset":  offset,
	})
}

// ListTorrents returns paginated list of torrents
func ListTorrents(w http.ResponseWriter, r *http.Request) {
	Search(w, r)
}

// GetTorrent returns a single torrent by ID
func GetTorrent(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid torrent ID")
		return
	}

	db := database.Get()
	row := db.QueryRow(`
		SELECT id, info_hash, name, size, category, seeders, leechers,
			   magnet_uri, files, title, year, tmdb_id, imdb_id, poster_url,
			   backdrop_url, overview, genres, rating, trust_score, upload_count,
			   first_seen_at, updated_at
		FROM torrents WHERE id = ?
	`, id)

	var torrent struct {
		ID           int64
		InfoHash     string
		Name         string
		Size         sql.NullInt64
		Category     sql.NullInt64
		Seeders      sql.NullInt64
		Leechers     sql.NullInt64
		MagnetURI    string
		Files        sql.NullString
		Title        sql.NullString
		Year         sql.NullInt64
		TmdbID       sql.NullInt64
		ImdbID       sql.NullString
		PosterURL    sql.NullString
		BackdropURL  sql.NullString
		Overview     sql.NullString
		Genres       sql.NullString
		Rating       sql.NullFloat64
		TrustScore   int64
		UploadCount  int64
		FirstSeenAt  string
		UpdatedAt    string
	}

	if err := row.Scan(
		&torrent.ID, &torrent.InfoHash, &torrent.Name, &torrent.Size,
		&torrent.Category, &torrent.Seeders, &torrent.Leechers, &torrent.MagnetURI,
		&torrent.Files, &torrent.Title, &torrent.Year, &torrent.TmdbID,
		&torrent.ImdbID, &torrent.PosterURL, &torrent.BackdropURL, &torrent.Overview,
		&torrent.Genres, &torrent.Rating, &torrent.TrustScore, &torrent.UploadCount,
		&torrent.FirstSeenAt, &torrent.UpdatedAt,
	); err != nil {
		if err == sql.ErrNoRows {
			respondError(w, http.StatusNotFound, "Torrent not found")
		} else {
			respondError(w, http.StatusInternalServerError, "Failed to get torrent")
		}
		return
	}

	// Get uploaders
	rows, err := db.Query(`
		SELECT uploader_npub, nostr_event_id, relay_url, uploaded_at
		FROM torrent_uploads
		WHERE torrent_id = ?
		ORDER BY uploaded_at ASC
	`, id)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to get uploaders")
		return
	}
	defer rows.Close()

	uploaders := make([]map[string]interface{}, 0)
	for rows.Next() {
		var npub, eventID, relayURL, uploadedAt string
		if err := rows.Scan(&npub, &eventID, &relayURL, &uploadedAt); err != nil {
			continue
		}
		uploaders = append(uploaders, map[string]interface{}{
			"npub":        npub,
			"event_id":    eventID,
			"relay_url":   relayURL,
			"uploaded_at": uploadedAt,
		})
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"id":            torrent.ID,
		"info_hash":     torrent.InfoHash,
		"name":          torrent.Name,
		"size":          torrent.Size.Int64,
		"category":      torrent.Category.Int64,
		"seeders":       torrent.Seeders.Int64,
		"leechers":      torrent.Leechers.Int64,
		"magnet_uri":    torrent.MagnetURI,
		"files":         torrent.Files.String,
		"title":         torrent.Title.String,
		"year":          torrent.Year.Int64,
		"tmdb_id":       torrent.TmdbID.Int64,
		"imdb_id":       torrent.ImdbID.String,
		"poster_url":    torrent.PosterURL.String,
		"backdrop_url":  torrent.BackdropURL.String,
		"overview":      torrent.Overview.String,
		"genres":        torrent.Genres.String,
		"rating":        torrent.Rating.Float64,
		"trust_score":   torrent.TrustScore,
		"upload_count":  torrent.UploadCount,
		"uploaders":     uploaders,
		"first_seen_at": torrent.FirstSeenAt,
		"updated_at":    torrent.UpdatedAt,
	})
}

// DeleteTorrent removes a torrent from the index
func DeleteTorrent(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid torrent ID")
		return
	}

	db := database.Get()
	result, err := db.Exec("DELETE FROM torrents WHERE id = ?", id)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to delete torrent")
		return
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		respondError(w, http.StatusNotFound, "Torrent not found")
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

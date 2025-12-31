package indexer

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gmonarque/lighthouse/internal/config"
	"github.com/gmonarque/lighthouse/internal/database"
	"github.com/rs/zerolog/log"
)

// Enricher handles metadata enrichment from external APIs
type Enricher struct {
	httpClient *http.Client
}

// TMDBSearchResult represents TMDB search response
type TMDBSearchResult struct {
	Results []struct {
		ID           int     `json:"id"`
		Title        string  `json:"title"`
		Name         string  `json:"name"` // For TV shows
		Overview     string  `json:"overview"`
		PosterPath   string  `json:"poster_path"`
		BackdropPath string  `json:"backdrop_path"`
		ReleaseDate  string  `json:"release_date"`
		FirstAirDate string  `json:"first_air_date"` // For TV shows
		VoteAverage  float64 `json:"vote_average"`
		GenreIDs     []int   `json:"genre_ids"`
	} `json:"results"`
}

// OMDBResult represents OMDB API response
type OMDBResult struct {
	Title    string `json:"Title"`
	Year     string `json:"Year"`
	ImdbID   string `json:"imdbID"`
	Poster   string `json:"Poster"`
	Plot     string `json:"Plot"`
	Genre    string `json:"Genre"`
	Rating   string `json:"imdbRating"`
	Response string `json:"Response"`
}

// NewEnricher creates a new Enricher
func NewEnricher() *Enricher {
	return &Enricher{
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// EnrichTorrent enriches a torrent with metadata
func (e *Enricher) EnrichTorrent(infoHash string) {
	cfg := config.Get()
	if !cfg.Enrichment.Enabled {
		return
	}

	db := database.Get()

	// Get torrent info
	var id int64
	var name string
	var category int
	err := db.QueryRow(`
		SELECT id, name, category FROM torrents WHERE info_hash = ?
	`, infoHash).Scan(&id, &name, &category)

	if err != nil {
		return
	}

	// Parse title and year from name
	title, year := parseTitle(name)

	// Try TMDB first for movies and TV
	if cfg.Enrichment.TMDBAPIKey != "" && (category == 2000 || category == 5000) {
		if e.enrichFromTMDB(id, title, year, category, cfg.Enrichment.TMDBAPIKey) {
			return
		}
	}

	// Fall back to OMDB
	if cfg.Enrichment.OMDBAPIKey != "" {
		e.enrichFromOMDB(id, title, year, cfg.Enrichment.OMDBAPIKey)
	}
}

// enrichFromTMDB enriches from TMDB API
func (e *Enricher) enrichFromTMDB(torrentID int64, title string, year int, category int, apiKey string) bool {
	// Determine search type
	searchType := "movie"
	if category == 5000 {
		searchType = "tv"
	}

	// Search TMDB
	searchURL := fmt.Sprintf(
		"https://api.themoviedb.org/3/search/%s?api_key=%s&query=%s",
		searchType, apiKey, url.QueryEscape(title),
	)

	if year > 0 {
		if searchType == "movie" {
			searchURL += fmt.Sprintf("&year=%d", year)
		} else {
			searchURL += fmt.Sprintf("&first_air_date_year=%d", year)
		}
	}

	resp, err := e.httpClient.Get(searchURL)
	if err != nil {
		log.Debug().Err(err).Msg("TMDB search failed")
		return false
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return false
	}

	var result TMDBSearchResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return false
	}

	if len(result.Results) == 0 {
		return false
	}

	// Use first result
	match := result.Results[0]

	// Get title
	matchTitle := match.Title
	if matchTitle == "" {
		matchTitle = match.Name
	}

	// Get year
	matchYear := 0
	releaseDate := match.ReleaseDate
	if releaseDate == "" {
		releaseDate = match.FirstAirDate
	}
	if len(releaseDate) >= 4 {
		matchYear, _ = strconv.Atoi(releaseDate[:4])
	}

	// Build poster/backdrop URLs
	posterURL := ""
	if match.PosterPath != "" {
		posterURL = "https://image.tmdb.org/t/p/w500" + match.PosterPath
	}
	backdropURL := ""
	if match.BackdropPath != "" {
		backdropURL = "https://image.tmdb.org/t/p/w1280" + match.BackdropPath
	}

	// Update database
	db := database.Get()
	_, err = db.Exec(`
		UPDATE torrents SET
			title = ?,
			year = ?,
			tmdb_id = ?,
			poster_url = ?,
			backdrop_url = ?,
			overview = ?,
			rating = ?,
			updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, matchTitle, matchYear, match.ID, posterURL, backdropURL, match.Overview, match.VoteAverage, torrentID)

	if err != nil {
		log.Error().Err(err).Msg("Failed to update torrent metadata")
		return false
	}

	log.Debug().
		Int64("id", torrentID).
		Str("title", matchTitle).
		Int("year", matchYear).
		Msg("Enriched torrent from TMDB")

	return true
}

// enrichFromOMDB enriches from OMDB API
func (e *Enricher) enrichFromOMDB(torrentID int64, title string, year int, apiKey string) bool {
	searchURL := fmt.Sprintf(
		"http://www.omdbapi.com/?apikey=%s&t=%s",
		apiKey, url.QueryEscape(title),
	)

	if year > 0 {
		searchURL += fmt.Sprintf("&y=%d", year)
	}

	resp, err := e.httpClient.Get(searchURL)
	if err != nil {
		log.Debug().Err(err).Msg("OMDB search failed")
		return false
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return false
	}

	var result OMDBResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return false
	}

	if result.Response != "True" {
		return false
	}

	// Parse year
	resultYear, _ := strconv.Atoi(result.Year)

	// Parse rating
	rating, _ := strconv.ParseFloat(result.Rating, 64)

	// Get poster
	posterURL := ""
	if result.Poster != "" && result.Poster != "N/A" {
		posterURL = result.Poster
	}

	// Update database
	db := database.Get()
	_, err = db.Exec(`
		UPDATE torrents SET
			title = ?,
			year = ?,
			imdb_id = ?,
			poster_url = COALESCE(NULLIF(poster_url, ''), ?),
			overview = ?,
			genres = ?,
			rating = ?,
			updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, result.Title, resultYear, result.ImdbID, posterURL, result.Plot, result.Genre, rating, torrentID)

	if err != nil {
		log.Error().Err(err).Msg("Failed to update torrent metadata")
		return false
	}

	log.Debug().
		Int64("id", torrentID).
		Str("title", result.Title).
		Str("imdb_id", result.ImdbID).
		Msg("Enriched torrent from OMDB")

	return true
}

// parseTitle extracts clean title and year from a torrent name
func parseTitle(name string) (string, int) {
	// Common patterns for year extraction
	yearPatterns := []*regexp.Regexp{
		regexp.MustCompile(`[\.\s\-_\(\[](19\d{2}|20\d{2})[\.\s\-_\)\]]`),
		regexp.MustCompile(`^(.+?)(19\d{2}|20\d{2})`),
	}

	var year int
	title := name

	for _, pattern := range yearPatterns {
		matches := pattern.FindStringSubmatch(name)
		if len(matches) >= 2 {
			yearStr := matches[len(matches)-1]
			if y, err := strconv.Atoi(yearStr); err == nil {
				year = y
				// Remove year and everything after from title
				idx := strings.Index(name, yearStr)
				if idx > 0 {
					title = name[:idx]
				}
				break
			}
		}
	}

	// Clean up title
	title = cleanTitle(title)

	return title, year
}

// cleanTitle removes common junk from torrent titles
func cleanTitle(title string) string {
	// Replace common separators with spaces
	replacer := strings.NewReplacer(
		".", " ",
		"_", " ",
		"-", " ",
	)
	title = replacer.Replace(title)

	// Remove quality indicators
	qualityPatterns := []string{
		`(?i)\b(720p|1080p|2160p|4k|uhd)\b`,
		`(?i)\b(bluray|bdrip|brrip|webrip|web-dl|webdl|hdtv|dvdrip|hdrip)\b`,
		`(?i)\b(x264|x265|h264|h265|hevc|avc)\b`,
		`(?i)\b(aac|ac3|dts|truehd|atmos)\b`,
		`(?i)\b(remux|proper|repack)\b`,
		`(?i)\[(.*?)\]`,
		`(?i)\((.*?)\)`,
	}

	for _, pattern := range qualityPatterns {
		re := regexp.MustCompile(pattern)
		title = re.ReplaceAllString(title, "")
	}

	// Remove extra spaces
	title = regexp.MustCompile(`\s+`).ReplaceAllString(title, " ")
	title = strings.TrimSpace(title)

	return title
}

// EnrichBatch enriches multiple torrents
func (e *Enricher) EnrichBatch(infoHashes []string) {
	for _, hash := range infoHashes {
		e.EnrichTorrent(hash)
		// Rate limit
		time.Sleep(250 * time.Millisecond)
	}
}

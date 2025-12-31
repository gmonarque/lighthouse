package handlers

import (
	"encoding/xml"
	"net/http"
	"strconv"

	"github.com/gmonarque/lighthouse/internal/config"
	"github.com/gmonarque/lighthouse/internal/torznab"
)

// Torznab handles all Torznab API requests
func Torznab(w http.ResponseWriter, r *http.Request) {
	// Validate API key
	cfg := config.Get()
	apiKey := r.URL.Query().Get("apikey")

	if cfg.Server.APIKey != "" && apiKey != cfg.Server.APIKey {
		respondTorznabError(w, torznab.ErrorIncorrectUserCreds, "Invalid API key")
		return
	}

	// Get function type
	t := r.URL.Query().Get("t")

	switch t {
	case "caps":
		handleCaps(w, r)
	case "search":
		handleSearch(w, r)
	case "tvsearch":
		handleTVSearch(w, r)
	case "movie":
		handleMovieSearch(w, r)
	case "music":
		handleMusicSearch(w, r)
	case "book":
		handleBookSearch(w, r)
	default:
		respondTorznabError(w, torznab.ErrorNoFunction, "Unknown function")
	}
}

// handleCaps returns Torznab capabilities
func handleCaps(w http.ResponseWriter, r *http.Request) {
	baseURL := getBaseURL(r)
	caps := torznab.NewCaps(baseURL)

	w.Header().Set("Content-Type", "application/xml")
	xml.NewEncoder(w).Encode(caps)
}

// handleSearch handles general search requests
func handleSearch(w http.ResponseWriter, r *http.Request) {
	params := parseSearchParams(r)
	executeSearch(w, r, params)
}

// handleTVSearch handles TV-specific search requests
func handleTVSearch(w http.ResponseWriter, r *http.Request) {
	params := parseSearchParams(r)

	// Default to TV categories if none specified
	if len(params.Categories) == 0 {
		params.Categories = []int{torznab.CategoryTV}
	}

	// Parse season/episode
	if s := r.URL.Query().Get("season"); s != "" {
		params.Season, _ = strconv.Atoi(s)
	}
	if e := r.URL.Query().Get("ep"); e != "" {
		params.Episode, _ = strconv.Atoi(e)
	}

	executeSearch(w, r, params)
}

// handleMovieSearch handles movie-specific search requests
func handleMovieSearch(w http.ResponseWriter, r *http.Request) {
	params := parseSearchParams(r)

	// Default to movie categories if none specified
	if len(params.Categories) == 0 {
		params.Categories = []int{torznab.CategoryMovies}
	}

	// Parse IMDB/TMDB IDs
	if imdb := r.URL.Query().Get("imdbid"); imdb != "" {
		params.ImdbID = torznab.NormalizeImdbID(imdb)
	}
	if tmdb := r.URL.Query().Get("tmdbid"); tmdb != "" {
		params.TmdbID, _ = strconv.Atoi(tmdb)
	}

	executeSearch(w, r, params)
}

// handleMusicSearch handles music-specific search requests
func handleMusicSearch(w http.ResponseWriter, r *http.Request) {
	params := parseSearchParams(r)

	// Default to audio categories if none specified
	if len(params.Categories) == 0 {
		params.Categories = []int{torznab.CategoryAudio}
	}

	// Append artist/album to query if provided
	if artist := r.URL.Query().Get("artist"); artist != "" {
		if params.Query != "" {
			params.Query += " "
		}
		params.Query += artist
	}
	if album := r.URL.Query().Get("album"); album != "" {
		if params.Query != "" {
			params.Query += " "
		}
		params.Query += album
	}

	executeSearch(w, r, params)
}

// handleBookSearch handles book-specific search requests
func handleBookSearch(w http.ResponseWriter, r *http.Request) {
	params := parseSearchParams(r)

	// Default to book categories if none specified
	if len(params.Categories) == 0 {
		params.Categories = []int{torznab.CategoryBooks}
	}

	// Append author/title to query if provided
	if author := r.URL.Query().Get("author"); author != "" {
		if params.Query != "" {
			params.Query += " "
		}
		params.Query += author
	}
	if title := r.URL.Query().Get("title"); title != "" {
		if params.Query != "" {
			params.Query += " "
		}
		params.Query += title
	}

	executeSearch(w, r, params)
}

// parseSearchParams parses common search parameters
func parseSearchParams(r *http.Request) torznab.SearchParams {
	params := torznab.SearchParams{
		Query:  r.URL.Query().Get("q"),
		Limit:  50,
		Offset: 0,
	}

	// Parse categories
	if cat := r.URL.Query().Get("cat"); cat != "" {
		params.Categories = torznab.ParseCategories(cat)
	}

	// Parse limit
	if limit := r.URL.Query().Get("limit"); limit != "" {
		if l, err := strconv.Atoi(limit); err == nil && l > 0 && l <= 100 {
			params.Limit = l
		}
	}

	// Parse offset
	if offset := r.URL.Query().Get("offset"); offset != "" {
		if o, err := strconv.Atoi(offset); err == nil && o >= 0 {
			params.Offset = o
		}
	}

	return params
}

// executeSearch executes a search and returns results
func executeSearch(w http.ResponseWriter, r *http.Request, params torznab.SearchParams) {
	service := torznab.NewService()

	results, total, err := service.Search(params)
	if err != nil {
		respondTorznabError(w, torznab.ErrorNoResults, "Search failed")
		return
	}

	baseURL := getBaseURL(r)
	rss := torznab.NewSearchResponse(baseURL, results, params.Offset, total)

	w.Header().Set("Content-Type", "application/xml")
	xml.NewEncoder(w).Encode(rss)
}

// respondTorznabError sends a Torznab error response
func respondTorznabError(w http.ResponseWriter, code int, description string) {
	w.Header().Set("Content-Type", "application/xml")
	w.WriteHeader(http.StatusOK) // Torznab errors still return 200
	xml.NewEncoder(w).Encode(torznab.NewErrorResponse(code, description))
}

// getBaseURL constructs the base URL from the request
func getBaseURL(r *http.Request) string {
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	if fwdProto := r.Header.Get("X-Forwarded-Proto"); fwdProto != "" {
		scheme = fwdProto
	}

	host := r.Host
	if fwdHost := r.Header.Get("X-Forwarded-Host"); fwdHost != "" {
		host = fwdHost
	}

	return scheme + "://" + host
}

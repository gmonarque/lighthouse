package api

import (
	"embed"
	"io/fs"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/lighthouse-client/lighthouse/internal/api/handlers"
	apiMiddleware "github.com/lighthouse-client/lighthouse/internal/api/middleware"
	"github.com/lighthouse-client/lighthouse/internal/config"
	"github.com/rs/zerolog/log"
)

//go:embed all:static
var staticFiles embed.FS

// NewRouter creates and configures the HTTP router
func NewRouter(cfg *config.Config) *chi.Mux {
	r := chi.NewRouter()

	// Middleware stack
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Compress(5))

	// CORS configuration
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-API-Key"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	// Health check (no auth required)
	r.Get("/health", handlers.HealthCheck)

	// API routes
	r.Route("/api", func(r chi.Router) {
		// Public endpoints (no auth required)
		r.Group(func(r chi.Router) {
			// Torznab API - uses query param apikey
			r.Get("/torznab", handlers.Torznab)

			// Setup wizard - public for first run
			r.Get("/setup/status", handlers.GetSetupStatus)
			r.Post("/setup/complete", handlers.CompleteSetup)
			r.Post("/settings/identity/generate", handlers.GenerateIdentity)
			r.Post("/settings/identity/import", handlers.ImportIdentity)

			// API key retrieval - for frontend authentication
			r.Get("/auth/key", handlers.GetAPIKey)
		})

		// Protected API endpoints
		r.Group(func(r chi.Router) {
			r.Use(apiMiddleware.APIKeyAuth)

			// Stats & Dashboard
			r.Get("/stats", handlers.GetStats)
			r.Get("/stats/chart", handlers.GetStatsChart)

			// Search
			r.Get("/search", handlers.Search)
			r.Get("/torrents", handlers.ListTorrents)
			r.Get("/torrents/{id}", handlers.GetTorrent)
			r.Delete("/torrents/{id}", handlers.DeleteTorrent)

			// Trust management
			r.Route("/trust", func(r chi.Router) {
				r.Get("/whitelist", handlers.GetWhitelist)
				r.Post("/whitelist", handlers.AddToWhitelist)
				r.Delete("/whitelist/{npub}", handlers.RemoveFromWhitelist)

				r.Get("/blacklist", handlers.GetBlacklist)
				r.Post("/blacklist", handlers.AddToBlacklist)
				r.Delete("/blacklist/{npub}", handlers.RemoveFromBlacklist)

				r.Get("/settings", handlers.GetTrustSettings)
				r.Put("/settings", handlers.UpdateTrustSettings)
			})

			// Relay management
			r.Route("/relays", func(r chi.Router) {
				r.Get("/", handlers.GetRelays)
				r.Post("/", handlers.AddRelay)
				r.Put("/{id}", handlers.UpdateRelay)
				r.Delete("/{id}", handlers.DeleteRelay)
				r.Post("/{id}/connect", handlers.ConnectRelay)
				r.Post("/{id}/disconnect", handlers.DisconnectRelay)
			})

			// Settings
			r.Route("/settings", func(r chi.Router) {
				r.Get("/", handlers.GetSettings)
				r.Put("/", handlers.UpdateSettings)
				r.Get("/export", handlers.ExportConfig)
				r.Post("/import", handlers.ImportConfig)
			})

			// Activity & Logs
			r.Get("/activity", handlers.GetActivity)
			r.Get("/logs", handlers.GetLogs)

			// Indexer control
			r.Post("/indexer/start", handlers.StartIndexer)
			r.Post("/indexer/stop", handlers.StopIndexer)
			r.Get("/indexer/status", handlers.GetIndexerStatus)

			// Publish torrent
			r.Post("/publish/parse-torrent", handlers.ParseTorrentFile)
			r.Post("/publish", handlers.PublishTorrent)
		})
	})

	// Serve static frontend files
	r.Get("/*", staticFileHandler())

	log.Info().Msg("Router initialized")
	return r
}

// staticFileHandler serves the embedded frontend files
func staticFileHandler() http.HandlerFunc {
	// Try to get the static subdirectory
	staticFS, err := fs.Sub(staticFiles, "static")
	if err != nil {
		log.Warn().Err(err).Msg("Static files not found, frontend will not be served")
		return func(w http.ResponseWriter, r *http.Request) {
			// Return a simple HTML page if no frontend is built
			if r.URL.Path == "/" || r.URL.Path == "" {
				w.Header().Set("Content-Type", "text/html")
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(`<!DOCTYPE html>
<html>
<head>
	<title>Lighthouse</title>
	<style>
		body { font-family: system-ui; max-width: 600px; margin: 100px auto; padding: 20px; }
		h1 { color: #333; }
		code { background: #f4f4f4; padding: 2px 6px; border-radius: 3px; }
	</style>
</head>
<body>
	<h1>Lighthouse</h1>
	<p>The backend is running, but the frontend hasn't been built yet.</p>
	<p>Run <code>make frontend</code> to build the web interface.</p>
	<p>API available at <code>/api</code></p>
</body>
</html>`))
				return
			}
			http.NotFound(w, r)
		}
	}

	fileServer := http.FileServer(http.FS(staticFS))

	return func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		if path == "/" {
			path = "/index.html"
		}

		// Try to serve the file
		file, err := staticFS.Open(strings.TrimPrefix(path, "/"))
		if err != nil {
			// For SPA routing, serve index.html for non-existent paths
			if !strings.Contains(path, ".") {
				r.URL.Path = "/"
				fileServer.ServeHTTP(w, r)
				return
			}
			http.NotFound(w, r)
			return
		}
		file.Close()

		fileServer.ServeHTTP(w, r)
	}
}

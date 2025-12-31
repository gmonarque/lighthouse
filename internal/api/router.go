package api

import (
	"embed"
	"io/fs"
	"net/http"
	"strings"

	"github.com/gmonarque/lighthouse/internal/api/handlers"
	apiMiddleware "github.com/gmonarque/lighthouse/internal/api/middleware"
	"github.com/gmonarque/lighthouse/internal/config"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
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

	// Rate limiting by IP (applies to all requests)
	r.Use(apiMiddleware.RateLimitByIP)

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
			// Torznab API - uses query param apikey with stricter rate limit
			r.With(apiMiddleware.RateLimitTorznab).Get("/torznab", handlers.Torznab)

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
			r.Use(apiMiddleware.RateLimitByAPIKey)

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
				r.Post("/whitelist/{npub}/discover-relays", handlers.DiscoverUserRelays)
				r.Post("/whitelist/discover-all-relays", handlers.DiscoverAllTrustedRelays)

				r.Get("/blacklist", handlers.GetBlacklist)
				r.Post("/blacklist", handlers.AddToBlacklist)
				r.Delete("/blacklist/{npub}", handlers.RemoveFromBlacklist)

				r.Get("/settings", handlers.GetTrustSettings)
				r.Put("/settings", handlers.UpdateTrustSettings)

				// Curator management (federated trust)
				r.Get("/curators", handlers.GetCurators)
				r.Post("/curators", handlers.AddCurator)
				r.Put("/curators/{pubkey}", handlers.UpdateCurator)
				r.Delete("/curators/{pubkey}", handlers.RevokeCurator)

				// Trust policy
				r.Get("/policy", handlers.GetTrustPolicy)

				// Aggregation policy
				r.Get("/aggregation", handlers.GetAggregationPolicy)
				r.Put("/aggregation", handlers.UpdateAggregationPolicy)
			})

			// Rulesets management
			r.Route("/rulesets", func(r chi.Router) {
				r.Get("/", handlers.GetRulesets)
				r.Get("/active", handlers.GetActiveRulesets)
				r.Get("/{id}", handlers.GetRuleset)
				r.Post("/", handlers.ImportRuleset)
				r.Post("/{id}/activate", handlers.ActivateRuleset)
				r.Post("/{id}/deactivate", handlers.DeactivateRuleset)
				r.Delete("/{id}", handlers.DeleteRuleset)
			})

			// Verification decisions
			r.Route("/decisions", func(r chi.Router) {
				r.Get("/", handlers.GetDecisions)
				r.Get("/stats", handlers.GetDecisionStats)
				r.Get("/reason-codes", handlers.GetReasonCodes)
				r.Get("/infohash/{infohash}", handlers.GetDecisionsByInfohash)
			})

			// Reports & Appeals
			r.Route("/reports", func(r chi.Router) {
				r.Get("/", handlers.GetReports)
				r.Get("/pending", handlers.GetPendingReports)
				r.Get("/{id}", handlers.GetReport)
				r.Post("/", handlers.SubmitReport)
				r.Put("/{id}", handlers.UpdateReport)
				r.Post("/{id}/acknowledge", handlers.AcknowledgeReport)
			})

			// Comments
			r.Route("/comments", func(r chi.Router) {
				r.Get("/recent", handlers.GetRecentComments)
				r.Get("/{eventId}", handlers.GetComment)
				r.Get("/{eventId}/thread", handlers.GetCommentThread)
				r.Delete("/{eventId}", handlers.DeleteComment)
			})

			// Torrent-specific comments
			r.Get("/torrents/{infohash}/comments", handlers.GetCommentsByInfohash)
			r.Post("/torrents/{infohash}/comments", handlers.AddComment)
			r.Get("/torrents/{infohash}/comments/stats", handlers.GetCommentStats)

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

			// Explorer stats
			r.Get("/explorer/stats", handlers.GetExplorerStats)

			// SLA monitoring
			r.Get("/sla/status", handlers.GetSLAStatus)
			r.Get("/sla/history", handlers.GetSLAHistory)

			// API Keys management
			r.Route("/apikeys", func(r chi.Router) {
				r.Get("/", handlers.GetAPIKeys)
				r.Post("/", handlers.CreateAPIKey)
				r.Get("/permissions", handlers.GetAvailablePermissions)
				r.Get("/{id}", handlers.GetAPIKeyByID)
				r.Put("/{id}", handlers.UpdateAPIKey)
				r.Delete("/{id}", handlers.DeleteAPIKey)
				r.Post("/{id}/enable", handlers.EnableAPIKey)
				r.Post("/{id}/disable", handlers.DisableAPIKey)
			})

			// Indexer control
			r.Post("/indexer/start", handlers.StartIndexer)
			r.Post("/indexer/stop", handlers.StopIndexer)
			r.Post("/indexer/resync", handlers.ResyncIndexer)
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

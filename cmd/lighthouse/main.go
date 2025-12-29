package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/lighthouse-client/lighthouse/internal/api"
	"github.com/lighthouse-client/lighthouse/internal/api/handlers"
	"github.com/lighthouse-client/lighthouse/internal/config"
	"github.com/lighthouse-client/lighthouse/internal/database"
	"github.com/lighthouse-client/lighthouse/internal/indexer"
	"github.com/lighthouse-client/lighthouse/internal/nostr"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

const banner = `
    __    _       __    __  __
   / /   (_)___ _/ /_  / /_/ /_  ____  __  __________
  / /   / / __ '/ __ \/ __/ __ \/ __ \/ / / / ___/ _ \
 / /___/ / /_/ / / / / /_/ / / / /_/ / /_/ (__  )  __/
/_____/_/\__, /_/ /_/\__/_/ /_/\____/\__,_/____/\___/
        /____/

    Decentralized Torrent Indexer powered by Nostr
    Your Node, Your Rules.
`

func main() {
	// Setup logging
	setupLogging()

	// Print banner
	fmt.Print(banner)

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to load configuration")
	}

	log.Info().
		Str("host", cfg.Server.Host).
		Int("port", cfg.Server.Port).
		Msg("Starting Lighthouse")

	// Initialize database
	if err := database.Init(cfg.Database.Path); err != nil {
		log.Fatal().Err(err).Msg("Failed to initialize database")
	}
	defer database.Close()

	// Seed default relays from config
	defaultRelays := make([]database.RelayConfig, len(cfg.Nostr.Relays))
	for i, r := range cfg.Nostr.Relays {
		defaultRelays[i] = database.RelayConfig{
			URL:     r.URL,
			Name:    r.Name,
			Preset:  r.Preset,
			Enabled: r.Enabled,
		}
	}
	if err := database.SeedDefaultRelays(defaultRelays); err != nil {
		log.Warn().Err(err).Msg("Failed to seed default relays")
	}

	// Seed default whitelist entries
	if err := database.SeedDefaultWhitelist(); err != nil {
		log.Warn().Err(err).Msg("Failed to seed default whitelist")
	}

	// Initialize Nostr relay manager
	relayManager := nostr.NewRelayManager(cfg.Nostr.Relays)

	// Initialize indexer
	idx := indexer.New(relayManager)

	// Wire up handlers to use the real indexer and relay manager
	handlers.SetIndexerController(idx)
	handlers.SetRelayLoader(relayManager)

	// Create router
	router := api.NewRouter(cfg)

	// Create HTTP server
	addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
	server := &http.Server{
		Addr:         addr,
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start server in goroutine
	go func() {
		log.Info().Str("address", addr).Msg("HTTP server listening")
		log.Info().Msgf("Open http://localhost:%d in your browser", cfg.Server.Port)

		if cfg.Server.APIKey != "" {
			log.Info().Str("api_key", cfg.Server.APIKey[:8]+"...").Msg("API Key (first 8 chars)")
		}

		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal().Err(err).Msg("HTTP server failed")
		}
	}()

	// Start indexer only if setup is completed
	if config.IsSetupCompleted() {
		go func() {
			// Load relays from database before starting
			if err := relayManager.LoadRelaysFromDB(); err != nil {
				log.Warn().Err(err).Msg("Failed to load relays from database")
			}

			if err := idx.Start(context.Background()); err != nil {
				log.Error().Err(err).Msg("Indexer failed to start")
			}
		}()
	} else {
		log.Info().Msg("Setup not complete - complete the setup wizard to start indexing")
	}

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info().Msg("Shutting down...")

	// Stop indexer
	idx.Stop()

	// Graceful shutdown with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Error().Err(err).Msg("Server forced to shutdown")
	}

	log.Info().Msg("Lighthouse stopped")
}

func setupLogging() {
	// Pretty console output for development
	output := zerolog.ConsoleWriter{
		Out:        os.Stdout,
		TimeFormat: "15:04:05",
	}

	log.Logger = zerolog.New(output).
		With().
		Timestamp().
		Caller().
		Logger()

	// Set log level from environment
	level := os.Getenv("LOG_LEVEL")
	switch level {
	case "debug":
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	case "warn":
		zerolog.SetGlobalLevel(zerolog.WarnLevel)
	case "error":
		zerolog.SetGlobalLevel(zerolog.ErrorLevel)
	default:
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
	}
}

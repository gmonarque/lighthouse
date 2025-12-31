// Package explorer handles discovery and collection of torrent events from Nostr relays.
package explorer

import (
	"context"
	"sync"
	"time"

	"github.com/gmonarque/lighthouse/internal/nostr"
	gonostr "github.com/nbd-wtf/go-nostr"
	"github.com/rs/zerolog/log"
)

// EventHandler is called when a new torrent event is discovered
type EventHandler func(event *gonostr.Event, relayURL string)

// Explorer discovers and collects torrent events from Nostr relays
// It does NOT make decisions - that's the Curator's responsibility
type Explorer struct {
	mu sync.RWMutex

	// Relay management
	relayManager *nostr.RelayManager

	// Event handling
	eventHandler EventHandler
	eventQueue   chan *QueuedEvent

	// Configuration
	queueSize       int
	lookbackPeriod  time.Duration
	reconnectPeriod time.Duration

	// State
	running bool
	ctx     context.Context
	cancel  context.CancelFunc
	stats   ExplorerStats
}

// QueuedEvent represents an event waiting to be processed
type QueuedEvent struct {
	Event      *gonostr.Event
	RelayURL   string
	ReceivedAt time.Time
}

// ExplorerStats tracks explorer statistics
type ExplorerStats struct {
	EventsDiscovered int64
	EventsQueued     int64
	EventsDropped    int64 // Dropped due to full queue
	RelaysConnected  int
	LastEventAt      time.Time
	StartedAt        time.Time
}

// Config holds explorer configuration
type Config struct {
	RelayManager    *nostr.RelayManager
	QueueSize       int           // Event queue size (default 1000)
	LookbackPeriod  time.Duration // How far back to fetch on start (default 24h)
	ReconnectPeriod time.Duration // Relay reconnect interval (default 5m)
}

// New creates a new Explorer
func New(cfg Config) *Explorer {
	if cfg.QueueSize == 0 {
		cfg.QueueSize = 1000
	}
	if cfg.LookbackPeriod == 0 {
		cfg.LookbackPeriod = 24 * time.Hour
	}
	if cfg.ReconnectPeriod == 0 {
		cfg.ReconnectPeriod = 5 * time.Minute
	}

	return &Explorer{
		relayManager:    cfg.RelayManager,
		queueSize:       cfg.QueueSize,
		lookbackPeriod:  cfg.LookbackPeriod,
		reconnectPeriod: cfg.ReconnectPeriod,
		eventQueue:      make(chan *QueuedEvent, cfg.QueueSize),
	}
}

// SetEventHandler sets the callback for discovered events
// This is typically the Curator's ProcessEvent method
func (e *Explorer) SetEventHandler(handler EventHandler) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.eventHandler = handler
}

// Start begins exploring relays for torrent events
func (e *Explorer) Start(ctx context.Context) error {
	e.mu.Lock()
	if e.running {
		e.mu.Unlock()
		return nil
	}
	e.ctx, e.cancel = context.WithCancel(ctx)
	e.running = true
	e.stats.StartedAt = time.Now()
	e.mu.Unlock()

	log.Info().
		Int("queue_size", e.queueSize).
		Dur("lookback", e.lookbackPeriod).
		Msg("Starting explorer")

	// Start relay manager
	if err := e.relayManager.Start(e.ctx); err != nil {
		log.Error().Err(err).Msg("Failed to start relay manager")
		return err
	}

	// Start event processor goroutine
	go e.processQueue()

	// Subscribe to torrent events
	since := time.Now().Add(-e.lookbackPeriod)
	err := e.relayManager.SubscribeTorrents(e.ctx, since, e.onEventReceived)
	if err != nil {
		log.Error().Err(err).Msg("Failed to subscribe to torrents")
		return err
	}

	// Start background maintenance
	go e.runMaintenance()

	log.Info().Msg("Explorer started")
	return nil
}

// Stop stops the explorer
func (e *Explorer) Stop() {
	e.mu.Lock()
	defer e.mu.Unlock()

	if !e.running {
		return
	}

	log.Info().Msg("Stopping explorer")

	if e.cancel != nil {
		e.cancel()
	}

	e.relayManager.Stop()
	e.running = false

	log.Info().Msg("Explorer stopped")
}

// IsRunning returns whether the explorer is running
func (e *Explorer) IsRunning() bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.running
}

// GetStats returns explorer statistics
func (e *Explorer) GetStats() ExplorerStats {
	e.mu.RLock()
	defer e.mu.RUnlock()
	stats := e.stats
	stats.RelaysConnected = e.relayManager.ConnectedCount()
	return stats
}

// onEventReceived is called when a torrent event is received from a relay
func (e *Explorer) onEventReceived(event *gonostr.Event, relayURL string) {
	e.mu.Lock()
	e.stats.EventsDiscovered++
	e.stats.LastEventAt = time.Now()
	e.mu.Unlock()

	// Queue the event for processing
	queued := &QueuedEvent{
		Event:      event,
		RelayURL:   relayURL,
		ReceivedAt: time.Now(),
	}

	select {
	case e.eventQueue <- queued:
		e.mu.Lock()
		e.stats.EventsQueued++
		e.mu.Unlock()
	default:
		// Queue is full, drop the event
		e.mu.Lock()
		e.stats.EventsDropped++
		e.mu.Unlock()
		log.Warn().
			Str("event_id", event.ID).
			Str("relay", relayURL).
			Msg("Event queue full, dropping event")
	}
}

// processQueue processes events from the queue
func (e *Explorer) processQueue() {
	for {
		select {
		case <-e.ctx.Done():
			return
		case queued := <-e.eventQueue:
			e.handleEvent(queued)
		}
	}
}

// handleEvent passes the event to the handler (Curator)
func (e *Explorer) handleEvent(queued *QueuedEvent) {
	e.mu.RLock()
	handler := e.eventHandler
	e.mu.RUnlock()

	if handler != nil {
		// Pass to Curator - Explorer does NOT make decisions
		handler(queued.Event, queued.RelayURL)
	}
}

// runMaintenance runs periodic maintenance tasks
func (e *Explorer) runMaintenance() {
	reconnectTicker := time.NewTicker(e.reconnectPeriod)
	defer reconnectTicker.Stop()

	statsTicker := time.NewTicker(1 * time.Minute)
	defer statsTicker.Stop()

	for {
		select {
		case <-e.ctx.Done():
			return

		case <-reconnectTicker.C:
			// Reconnect disconnected relays
			e.relayManager.ReconnectAll()

		case <-statsTicker.C:
			// Log stats
			stats := e.GetStats()
			log.Info().
				Int64("discovered", stats.EventsDiscovered).
				Int64("queued", stats.EventsQueued).
				Int64("dropped", stats.EventsDropped).
				Int("relays", stats.RelaysConnected).
				Msg("Explorer stats")
		}
	}
}

// FetchHistorical fetches historical events from relays
func (e *Explorer) FetchHistorical(days int) error {
	if !e.IsRunning() {
		return nil
	}

	since := time.Now().AddDate(0, 0, -days)
	log.Info().Int("days", days).Msg("Fetching historical events")

	return e.relayManager.SubscribeTorrents(e.ctx, since, e.onEventReceived)
}

// QueueLength returns the current queue length
func (e *Explorer) QueueLength() int {
	return len(e.eventQueue)
}

// GetRelayManager returns the relay manager for external configuration
func (e *Explorer) GetRelayManager() *nostr.RelayManager {
	return e.relayManager
}

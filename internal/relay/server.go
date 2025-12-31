// Package relay implements a Nostr relay for torrent events
package relay

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/gmonarque/lighthouse/internal/config"
	"github.com/gorilla/websocket"
	"github.com/rs/zerolog/log"
)

// Server is a Nostr relay server for torrent events
type Server struct {
	mu sync.RWMutex

	// Configuration
	listen          string
	mode            string // "public" or "community"
	requireCuration bool

	// Components
	policy        *TorrentPolicy
	storage       *EventStorage
	subscriptions map[string]*Subscription

	// WebSocket upgrader
	upgrader websocket.Upgrader

	// State
	running bool
	server  *http.Server
	clients map[*websocket.Conn]*Client
}

// Client represents a connected WebSocket client
type Client struct {
	conn          *websocket.Conn
	subscriptions map[string]*Subscription
	pubkey        string
	authenticated bool
	lastPing      time.Time
}

// Subscription represents a client subscription
type Subscription struct {
	ID      string
	Filters []Filter
	Client  *Client
}

// Config holds relay configuration
type Config struct {
	Listen          string
	Mode            string
	RequireCuration bool
	SyncWith        []string
	EnableDiscovery bool
}

// NewServer creates a new relay server
func NewServer(cfg Config) (*Server, error) {
	s := &Server{
		listen:          cfg.Listen,
		mode:            cfg.Mode,
		requireCuration: cfg.RequireCuration,
		policy:          NewTorrentPolicy(),
		storage:         NewEventStorage(),
		subscriptions:   make(map[string]*Subscription),
		clients:         make(map[*websocket.Conn]*Client),
		upgrader: websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
			CheckOrigin: func(r *http.Request) bool {
				return true // Allow all origins for relay
			},
		},
	}

	return s, nil
}

// Start starts the relay server
func (s *Server) Start() error {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return fmt.Errorf("relay already running")
	}
	s.running = true
	s.mu.Unlock()

	mux := http.NewServeMux()
	mux.HandleFunc("/", s.handleWebSocket)
	mux.HandleFunc("/health", s.handleHealth)

	s.server = &http.Server{
		Addr:         s.listen,
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	log.Info().
		Str("listen", s.listen).
		Str("mode", s.mode).
		Msg("Starting Nostr relay server")

	go func() {
		if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error().Err(err).Msg("Relay server error")
		}
	}()

	return nil
}

// Stop stops the relay server
func (s *Server) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running {
		return nil
	}

	s.running = false

	// Close all client connections
	for conn := range s.clients {
		conn.Close()
	}

	if s.server != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return s.server.Shutdown(ctx)
	}

	return nil
}

// handleWebSocket handles WebSocket connections
func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Error().Err(err).Msg("WebSocket upgrade failed")
		return
	}

	client := &Client{
		conn:          conn,
		subscriptions: make(map[string]*Subscription),
		lastPing:      time.Now(),
	}

	s.mu.Lock()
	s.clients[conn] = client
	s.mu.Unlock()

	log.Debug().Str("remote", conn.RemoteAddr().String()).Msg("Client connected")

	defer func() {
		s.mu.Lock()
		delete(s.clients, conn)
		s.mu.Unlock()
		conn.Close()
		log.Debug().Str("remote", conn.RemoteAddr().String()).Msg("Client disconnected")
	}()

	// Handle messages
	for {
		_, message, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Error().Err(err).Msg("WebSocket read error")
			}
			break
		}

		s.handleMessage(client, message)
	}
}

// handleMessage processes incoming Nostr messages
func (s *Server) handleMessage(client *Client, message []byte) {
	var msg []json.RawMessage
	if err := json.Unmarshal(message, &msg); err != nil {
		s.sendNotice(client, "Invalid message format")
		return
	}

	if len(msg) < 1 {
		s.sendNotice(client, "Empty message")
		return
	}

	var msgType string
	if err := json.Unmarshal(msg[0], &msgType); err != nil {
		s.sendNotice(client, "Invalid message type")
		return
	}

	switch msgType {
	case "EVENT":
		if len(msg) < 2 {
			s.sendNotice(client, "Missing event data")
			return
		}
		s.handleEvent(client, msg[1])

	case "REQ":
		if len(msg) < 3 {
			s.sendNotice(client, "Missing subscription data")
			return
		}
		s.handleReq(client, msg[1:])

	case "CLOSE":
		if len(msg) < 2 {
			s.sendNotice(client, "Missing subscription ID")
			return
		}
		s.handleClose(client, msg[1])

	default:
		s.sendNotice(client, fmt.Sprintf("Unknown message type: %s", msgType))
	}
}

// handleEvent processes EVENT messages
func (s *Server) handleEvent(client *Client, eventData json.RawMessage) {
	// Rate limit event submissions per client
	clientID := client.conn.RemoteAddr().String()
	if !checkRelayRateLimit(clientID) {
		s.sendOK(client, "", false, "Rate limit exceeded. Try again later.")
		return
	}

	var event Event
	if err := json.Unmarshal(eventData, &event); err != nil {
		s.sendOK(client, "", false, "Invalid event format")
		return
	}

	// Verify event signature
	if !event.VerifySignature() {
		s.sendOK(client, event.ID, false, "Invalid signature")
		return
	}

	// Check if event kind is allowed
	if !s.isEventAllowed(&event) {
		s.sendOK(client, event.ID, false, "Event kind not allowed")
		return
	}

	// Apply torrent policy
	if event.Kind == 2003 { // Torrent event
		if allowed, reason := s.policy.CheckEvent(&event); !allowed {
			s.sendOK(client, event.ID, false, reason)
			return
		}
	}

	// Store event
	if err := s.storage.Save(&event); err != nil {
		s.sendOK(client, event.ID, false, fmt.Sprintf("Storage error: %v", err))
		return
	}

	// Send OK
	s.sendOK(client, event.ID, true, "")

	// Broadcast to subscribers
	s.broadcastEvent(&event)
}

// handleReq processes REQ messages
func (s *Server) handleReq(client *Client, msg []json.RawMessage) {
	var subID string
	if err := json.Unmarshal(msg[0], &subID); err != nil {
		s.sendNotice(client, "Invalid subscription ID")
		return
	}

	// Parse filters
	var filters []Filter
	for _, filterData := range msg[1:] {
		var filter Filter
		if err := json.Unmarshal(filterData, &filter); err != nil {
			s.sendNotice(client, "Invalid filter format")
			return
		}
		filters = append(filters, filter)
	}

	// Create subscription
	sub := &Subscription{
		ID:      subID,
		Filters: filters,
		Client:  client,
	}

	client.subscriptions[subID] = sub

	s.mu.Lock()
	s.subscriptions[subID] = sub
	s.mu.Unlock()

	// Query stored events matching filters
	events := s.storage.Query(filters)
	for _, event := range events {
		s.sendEvent(client, subID, event)
	}

	// Send EOSE (End of Stored Events)
	s.sendEOSE(client, subID)
}

// handleClose processes CLOSE messages
func (s *Server) handleClose(client *Client, subIDData json.RawMessage) {
	var subID string
	if err := json.Unmarshal(subIDData, &subID); err != nil {
		return
	}

	delete(client.subscriptions, subID)

	s.mu.Lock()
	delete(s.subscriptions, subID)
	s.mu.Unlock()
}

// isEventAllowed checks if an event kind is allowed
func (s *Server) isEventAllowed(event *Event) bool {
	// Torrent-related kinds
	allowedKinds := map[int]bool{
		2003:  true, // Torrent
		2004:  true, // Torrent comment
		30175: true, // Verification decision
		30173: true, // Trust policy
	}

	if s.mode == "public" {
		// Public mode allows all standard kinds plus torrent kinds
		return event.Kind < 10000 || allowedKinds[event.Kind]
	}

	// Community mode only allows torrent-related kinds
	return allowedKinds[event.Kind]
}

// broadcastEvent sends an event to all matching subscribers
func (s *Server) broadcastEvent(event *Event) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, sub := range s.subscriptions {
		if matchesFilters(event, sub.Filters) {
			s.sendEvent(sub.Client, sub.ID, event)
		}
	}
}

// sendEvent sends an EVENT message to a client
func (s *Server) sendEvent(client *Client, subID string, event *Event) {
	msg, _ := json.Marshal([]interface{}{"EVENT", subID, event})
	client.conn.WriteMessage(websocket.TextMessage, msg)
}

// sendOK sends an OK message to a client
func (s *Server) sendOK(client *Client, eventID string, success bool, message string) {
	msg, _ := json.Marshal([]interface{}{"OK", eventID, success, message})
	client.conn.WriteMessage(websocket.TextMessage, msg)
}

// sendNotice sends a NOTICE message to a client
func (s *Server) sendNotice(client *Client, message string) {
	msg, _ := json.Marshal([]interface{}{"NOTICE", message})
	client.conn.WriteMessage(websocket.TextMessage, msg)
}

// sendEOSE sends an EOSE message to a client
func (s *Server) sendEOSE(client *Client, subID string) {
	msg, _ := json.Marshal([]interface{}{"EOSE", subID})
	client.conn.WriteMessage(websocket.TextMessage, msg)
}

// handleHealth handles health check requests
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	clientCount := len(s.clients)
	s.mu.RUnlock()

	info := map[string]interface{}{
		"status":  "ok",
		"mode":    s.mode,
		"clients": clientCount,
		"events":  s.storage.Count(),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(info)
}

// GetStats returns relay statistics
func (s *Server) GetStats() RelayStats {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return RelayStats{
		Running:     s.running,
		ClientCount: len(s.clients),
		EventCount:  s.storage.Count(),
		SubCount:    len(s.subscriptions),
	}
}

// RelayStats contains relay statistics
type RelayStats struct {
	Running     bool
	ClientCount int
	EventCount  int64
	SubCount    int
}

// Global relay instance
var globalRelay *Server

// InitGlobal initializes the global relay instance
func InitGlobal() error {
	cfg := config.Get()

	if !cfg.Relay.Enabled {
		log.Info().Msg("Relay server disabled")
		return nil
	}

	relayCfg := Config{
		Listen:          cfg.Relay.Listen,
		Mode:            cfg.Relay.Mode,
		RequireCuration: cfg.Relay.RequireCuration,
		SyncWith:        cfg.Relay.SyncWith,
		EnableDiscovery: cfg.Relay.EnableDiscovery,
	}

	var err error
	globalRelay, err = NewServer(relayCfg)
	if err != nil {
		return err
	}

	return globalRelay.Start()
}

// Get returns the global relay instance
func Get() *Server {
	return globalRelay
}

// matchesFilters checks if an event matches any of the filters
func matchesFilters(event *Event, filters []Filter) bool {
	for _, filter := range filters {
		if matchesFilter(event, filter) {
			return true
		}
	}
	return false
}

// relayRateLimiter for rate limiting relay events
var relayRateLimiter = newRelayRateLimiter(30, time.Minute) // 30 events per minute per client

type relayRateLimiterImpl struct {
	mu       sync.RWMutex
	limiters map[string]*relayTokenBucket
	rate     int
	window   time.Duration
}

type relayTokenBucket struct {
	tokens    int
	lastReset time.Time
}

func newRelayRateLimiter(rate int, window time.Duration) *relayRateLimiterImpl {
	return &relayRateLimiterImpl{
		limiters: make(map[string]*relayTokenBucket),
		rate:     rate,
		window:   window,
	}
}

func (rl *relayRateLimiterImpl) allow(key string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	bucket, exists := rl.limiters[key]

	if !exists {
		rl.limiters[key] = &relayTokenBucket{
			tokens:    rl.rate - 1,
			lastReset: now,
		}
		return true
	}

	if now.Sub(bucket.lastReset) >= rl.window {
		bucket.tokens = rl.rate - 1
		bucket.lastReset = now
		return true
	}

	if bucket.tokens > 0 {
		bucket.tokens--
		return true
	}

	return false
}

func checkRelayRateLimit(clientID string) bool {
	return relayRateLimiter.allow(clientID)
}

// matchesFilter checks if an event matches a single filter
func matchesFilter(event *Event, filter Filter) bool {
	// Check IDs
	if len(filter.IDs) > 0 {
		found := false
		for _, id := range filter.IDs {
			if event.ID == id || (len(id) < 64 && len(event.ID) >= len(id) && event.ID[:len(id)] == id) {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// Check authors
	if len(filter.Authors) > 0 {
		found := false
		for _, author := range filter.Authors {
			if event.PubKey == author || (len(author) < 64 && len(event.PubKey) >= len(author) && event.PubKey[:len(author)] == author) {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// Check kinds
	if len(filter.Kinds) > 0 {
		found := false
		for _, kind := range filter.Kinds {
			if event.Kind == kind {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// Check since/until
	if filter.Since > 0 && event.CreatedAt < filter.Since {
		return false
	}
	if filter.Until > 0 && event.CreatedAt > filter.Until {
		return false
	}

	// Check tags
	for tagName, values := range filter.Tags {
		if len(values) == 0 {
			continue
		}
		found := false
		for _, tag := range event.Tags {
			if len(tag) >= 2 && tag[0] == tagName {
				for _, v := range values {
					if tag[1] == v {
						found = true
						break
					}
				}
			}
			if found {
				break
			}
		}
		if !found {
			return false
		}
	}

	return true
}

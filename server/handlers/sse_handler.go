// ============================================================
// FILE: server/handlers/sse_handler.go
// TYPE: Handler + SSE Infrastructure — Real-Time Event Streaming
//
// WHAT IS SSE (Server-Sent Events)?
// SSE is a one-way, persistent HTTP connection where the server
// continuously pushes data to the browser — without the browser asking.
//
// ANALOGY: SSE is like a radio broadcast.
//   - Server = radio station (broadcasts to everyone tuned in)
//   - Browser = radio listener (receives, doesn't send back)
//
// SSE vs WebSocket:
//   SSE: simpler, one-way (server → client), built on HTTP, auto-reconnects
//   WebSocket: two-way (client ↔ server), custom protocol, more complex
//   Use SSE when: you only need server → client (price alerts ✅)
//   Use WebSocket when: you need bidirectional (chat, games)
//
// HOW OUR SSE WORKS:
//   1. Browser opens: GET /api/stream?token=<jwt>
//   2. SSEManager registers the connection as a named SSEClient
//   3. Go's notification worker calls mgr.SendToUser(userID, jsonMsg)
//   4. SSEManager finds all connections for that user, sends the message
//   5. Browser's EventSource receives it and updates the UI
//
// FAANG CONCEPT: Fan-out pattern
// One event can go to MULTIPLE connections for the same user
// (e.g., user has app open in 3 browser tabs → all 3 get the alert)
//
// GO CONCEPTS:
//   sync.RWMutex (read-write lock), channels (chan string),
//   http.Flusher interface, r.Context().Done(), goroutine lifecycle
// ============================================================
package handlers

import (
	"fmt"
	"net/http"
	"sync"

	"pokemontool/config"
	"pokemontool/middleware"
)

// SSEClient represents a single browser connection to /api/stream.
// Each tab open = one SSEClient instance.
// Each user can have multiple SSEClient instances (multiple tabs).
type SSEClient struct {
	UserID string      // which user owns this connection
	Send   chan string  // the channel to send JSON messages into
	// Channel: a Go built-in for safe communication between goroutines
	// Think of it like a pipe: one goroutine writes in, another reads out
}

// SSEManager tracks ALL active SSE connections across all users.
// It's the registry — you look up a userID and get their connections.
//
// sync.RWMutex = Read-Write Mutex (a lock for concurrent access)
// PROBLEM: SendToUser and Add/Remove can be called from different goroutines.
// Without a lock, concurrent map access causes a DATA RACE (crash in Go).
// SOLUTION: RWMutex
//   - Multiple goroutines can READ (SendToUser) simultaneously
//   - Only ONE goroutine can WRITE (Add/Remove) at a time
type SSEManager struct {
	mu      sync.RWMutex          // the lock
	clients map[string][]*SSEClient // userID → list of their connections
	// map[string][]*SSEClient: one user can have multiple open connections (tabs)
}

// NewSSEManager creates a new manager with an empty clients map.
func NewSSEManager() *SSEManager {
	return &SSEManager{clients: make(map[string][]*SSEClient)}
}

// Add registers a new browser connection.
// mu.Lock() takes an exclusive write lock — readers block until done.
func (m *SSEManager) Add(c *SSEClient) {
	m.mu.Lock()
	defer m.mu.Unlock() // defer: releases the lock when Add() returns (even on panic)
	m.clients[c.UserID] = append(m.clients[c.UserID], c)
}

// Remove unregisters a connection when the browser disconnects.
// Called via defer when r.Context().Done() fires (tab closed, network drop, etc.)
func (m *SSEManager) Remove(c *SSEClient) {
	m.mu.Lock()
	defer m.mu.Unlock()
	list := m.clients[c.UserID]
	for i, existing := range list {
		if existing == c {
			// Remove by replacing with a new slice that excludes this element
			m.clients[c.UserID] = append(list[:i], list[i+1:]...)
			return
		}
	}
}

// SendToUser pushes a JSON message to ALL connections for a specific user.
// This is the fan-out: one price alert sends to all the user's open tabs.
// mu.RLock() = shared read lock — multiple callers can SendToUser concurrently.
func (m *SSEManager) SendToUser(userID, jsonMsg string) {
	m.mu.RLock() // read lock (not write lock — we're only reading the map)
	defer m.mu.RUnlock()
	for _, c := range m.clients[userID] {
		// Non-blocking send: if the channel is full (slow client), SKIP it.
		// We never block the notification worker for a slow browser.
		// SELECT with default = try to send; if buffer full, do the default case.
		select {
		case c.Send <- jsonMsg:  // successfully queued
		default:                 // client is too slow — drop this message
		}
	}
}

// Stream is the HTTP handler for GET /api/stream?token=<jwt>
// This is a long-lived HTTP handler — it never returns until the browser disconnects.
// It validates the JWT, registers the SSEClient, and streams events forever.
func Stream(mgr *SSEManager, cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// ── Authenticate via query param ───────────────────────────
		// EventSource in the browser can't set custom headers.
		// So we accept the JWT in the ?token= query parameter instead.
		// ValidateStreamToken parses and validates the JWT.
		token := r.URL.Query().Get("token")
		user, ok := middleware.ValidateStreamToken(token, cfg.JWTSecret)
		if !ok {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		// ── Set SSE Headers ─────────────────────────────────────────
		// These headers tell the browser this is a streaming response.
		// Content-Type: text/event-stream = SSE format
		// Cache-Control: no-cache = don't cache this stream
		// X-Accel-Buffering: no = tells nginx not to buffer (required for streaming through a proxy)
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.Header().Set("X-Accel-Buffering", "no")
		w.WriteHeader(http.StatusOK) // 200 OK before we start streaming

		// ── Get the HTTP Flusher ───────────────────────────────────
		// Normal ResponseWriter buffers output — it only sends when the handler returns.
		// http.Flusher.Flush() forces immediate send after each event.
		// If the response writer doesn't support Flusher (it always does for SSE), fail.
		flusher, ok := w.(http.Flusher) // type assertion: does w implement Flusher?
		if !ok {
			http.Error(w, "streaming unsupported", http.StatusInternalServerError)
			return
		}

		// ── Register this connection ───────────────────────────────
		client := &SSEClient{UserID: user.ID, Send: make(chan string, 16)}
		// buffered channel with capacity 16 — sender never blocks on a slow reader
		mgr.Add(client)
		defer mgr.Remove(client) // ALWAYS remove when this handler exits (tab closes, etc.)

		// Send an immediate confirmation message so the browser knows it's connected.
		// SSE format: "data: <json>\n\n" (MUST end with double newline)
		fmt.Fprintf(w, "data: {\"type\":\"CONNECTED\"}\n\n")
		flusher.Flush()

		// ── Event Loop ─────────────────────────────────────────────
		// This is the core of SSE: sit here forever, sending events as they arrive.
		// select waits on MULTIPLE channels simultaneously — like a switch statement for channels.
		for {
			select {
			case msg := <-client.Send:
				// A new event arrived — write it and flush immediately
				fmt.Fprintf(w, "data: %s\n\n", msg)
				flusher.Flush()
			case <-r.Context().Done():
				// r.Context().Done() fires when the HTTP request is cancelled:
				//   - User closed the browser tab
				//   - Network disconnected
				//   - Server timeout
				// When this fires, we return — defer mgr.Remove(client) cleans up.
				return
			}
		}
	}
}

// TODO #1 (Practice): Add a heartbeat to keep connections alive
// After 30-60 seconds of no events, some proxies (nginx, AWS ALB) drop idle connections.
// Fix: send a "heartbeat" comment every 30 seconds.
// SSE comments are lines starting with ": " and are ignored by EventSource.
// Add a time.NewTicker(30 * time.Second) to the for/select loop:
//   case <-heartbeat.C:
//       fmt.Fprintf(w, ": heartbeat\n\n")
//       flusher.Flush()

// TODO #2 (Practice): Add connection counting to SSEManager
// Add a method: ActiveConnections() int
// It should count all clients across all users.
// Use this in the /health endpoint: {"status":"ok","sse_connections":42}
// This is useful for monitoring — you can graph SSE connections over time.

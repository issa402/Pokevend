// handlers/sse_handler.go — SSE stream endpoint + SSEManager
// SSEManager is in handlers because it bridges HTTP connections and the worker layer.
package handlers

import (
	"fmt"
	"net/http"
	"sync"

	"pokemontool/config"
	"pokemontool/middleware"
)

// SSEClient represents one browser tab connected to /api/stream
type SSEClient struct {
	UserID string
	Send   chan string
}

// SSEManager tracks all active SSE connections per user
type SSEManager struct {
	mu      sync.RWMutex
	clients map[string][]*SSEClient
}

func NewSSEManager() *SSEManager {
	return &SSEManager{clients: make(map[string][]*SSEClient)}
}

func (m *SSEManager) Add(c *SSEClient) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.clients[c.UserID] = append(m.clients[c.UserID], c)
}

func (m *SSEManager) Remove(c *SSEClient) {
	m.mu.Lock()
	defer m.mu.Unlock()
	list := m.clients[c.UserID]
	for i, existing := range list {
		if existing == c {
			m.clients[c.UserID] = append(list[:i], list[i+1:]...)
			return
		}
	}
}

// SendToUser pushes a message to all connections for a specific user
func (m *SSEManager) SendToUser(userID, jsonMsg string) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, c := range m.clients[userID] {
		select {
		case c.Send <- jsonMsg:
		default: // drop if client is slow — don't block
		}
	}
}

// Stream is the HTTP handler for GET /api/stream?token=<jwt>
// EventSource in the browser can't set headers, so auth is via query param.
func Stream(mgr *SSEManager, cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token := r.URL.Query().Get("token")
		user, ok := middleware.ValidateStreamToken(token, cfg.JWTSecret)
		if !ok {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.Header().Set("X-Accel-Buffering", "no")
		w.WriteHeader(http.StatusOK)

		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming unsupported", http.StatusInternalServerError)
			return
		}

		client := &SSEClient{UserID: user.ID, Send: make(chan string, 16)}
		mgr.Add(client)
		defer mgr.Remove(client)

		fmt.Fprintf(w, "data: {\"type\":\"CONNECTED\"}\n\n")
		flusher.Flush()

		for {
			select {
			case msg := <-client.Send:
				fmt.Fprintf(w, "data: %s\n\n", msg)
				flusher.Flush()
			case <-r.Context().Done():
				return
			}
		}
	}
}

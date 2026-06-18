package server

import (
	"fmt"
	"net/http"
	"sync"
)

// sseHub broadcasts named events to all connected clients.
type sseHub struct {
	mu      sync.Mutex
	clients map[chan sseMessage]struct{}
}

type sseMessage struct {
	event string
	data  string
}

func newSSEHub() *sseHub {
	return &sseHub{clients: make(map[chan sseMessage]struct{})}
}

func (h *sseHub) add() chan sseMessage {
	ch := make(chan sseMessage, 8)
	h.mu.Lock()
	h.clients[ch] = struct{}{}
	h.mu.Unlock()
	return ch
}

func (h *sseHub) remove(ch chan sseMessage) {
	h.mu.Lock()
	if _, ok := h.clients[ch]; ok {
		delete(h.clients, ch)
		close(ch)
	}
	h.mu.Unlock()
}

func (h *sseHub) broadcast(event, data string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for ch := range h.clients {
		select {
		case ch <- sseMessage{event: event, data: data}:
		default: // drop if client is slow
		}
	}
}

// serveHTTP streams events to a single client until disconnect.
func (h *sseHub) serveHTTP(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	ch := h.add()
	defer h.remove(ch)

	// initial comment to open the stream
	fmt.Fprint(w, ": connected\n\n")
	flusher.Flush()

	ctx := r.Context()
	for {
		select {
		case <-ctx.Done():
			return
		case msg, ok := <-ch:
			if !ok {
				return
			}
			fmt.Fprintf(w, "event: %s\ndata: %s\n\n", msg.event, msg.data)
			flusher.Flush()
		}
	}
}

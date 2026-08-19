package viewer

import (
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"
)

// changeHub fans out file-change notifications to every connected SSE client.
type changeHub struct {
	mu   sync.Mutex
	subs map[chan struct{}]struct{}
}

func newChangeHub() *changeHub {
	return &changeHub{subs: make(map[chan struct{}]struct{})}
}

func (h *changeHub) subscribe() chan struct{} {
	ch := make(chan struct{}, 1)
	h.mu.Lock()
	h.subs[ch] = struct{}{}
	h.mu.Unlock()
	return ch
}

func (h *changeHub) unsubscribe(ch chan struct{}) {
	h.mu.Lock()
	delete(h.subs, ch)
	h.mu.Unlock()
}

func (h *changeHub) broadcast() {
	h.mu.Lock()
	defer h.mu.Unlock()
	for ch := range h.subs {
		select {
		case ch <- struct{}{}:
		default: // drop for slow clients; they re-sync on reconnect
		}
	}
}

func (srv *Server) handleEvents(writer http.ResponseWriter, request *http.Request) {
	flusher, ok := writer.(http.Flusher)
	if !ok {
		http.Error(writer, "streaming unsupported", http.StatusInternalServerError)
		return
	}

	writer.Header().Set("Content-Type", "text/event-stream")
	writer.Header().Set("Cache-Control", "no-cache")
	writer.Header().Set("Connection", "keep-alive")

	// The HTTP server sets a write deadline from WriteTimeout; clear it so the
	// long-lived stream is not killed between events.
	if err := http.NewResponseController(writer).SetWriteDeadline(time.Time{}); err != nil {
		log.Printf("unable to clear write deadline for SSE: %v", err)
	}

	writer.WriteHeader(http.StatusOK)
	if _, err := fmt.Fprint(writer, ": connected\n\n"); err != nil {
		return
	}
	flusher.Flush()

	sub := srv.hub.subscribe()
	defer srv.hub.unsubscribe(sub)

	for {
		select {
		case <-request.Context().Done():
			return
		case <-sub:
			if _, err := fmt.Fprint(writer, "data: reload\n\n"); err != nil {
				return
			}
			flusher.Flush()
		}
	}
}

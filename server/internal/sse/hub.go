package sse

import (
	"encoding/json"
	"sync"
)

type Hub struct {
	mu      sync.RWMutex
	clients map[string]map[chan []byte]struct{}
}

func NewHub() *Hub {
	return &Hub{clients: make(map[string]map[chan []byte]struct{})}
}

func (h *Hub) Subscribe(userID string) chan []byte {
	ch := make(chan []byte, 8)
	h.mu.Lock()
	if h.clients[userID] == nil {
		h.clients[userID] = make(map[chan []byte]struct{})
	}
	h.clients[userID][ch] = struct{}{}
	h.mu.Unlock()
	return ch
}

func (h *Hub) Unsubscribe(userID string, ch chan []byte) {
	h.mu.Lock()
	if subs, ok := h.clients[userID]; ok {
		delete(subs, ch)
		if len(subs) == 0 {
			delete(h.clients, userID)
		}
	}
	h.mu.Unlock()
	close(ch)
}

func (h *Hub) NotifyMessage(userID string, payload any) {
	data, err := json.Marshal(payload)
	if err != nil {
		return
	}
	h.mu.RLock()
	defer h.mu.RUnlock()
	for ch := range h.clients[userID] {
		select {
		case ch <- data:
		default:
		}
	}
}

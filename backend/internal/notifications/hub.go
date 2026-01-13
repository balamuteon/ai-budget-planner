package notifications

import (
	"sync"
	"time"

	"github.com/google/uuid"
)

type Event struct {
	Type      string      `json:"type"`
	Timestamp time.Time   `json:"timestamp"`
	Data      interface{} `json:"data,omitempty"`
}

type Hub struct {
	mu          sync.RWMutex
	subscribers map[uuid.UUID]map[chan Event]struct{}
}

// NewHub создает хаб для SSE-подписок.
func NewHub() *Hub {
	return &Hub{
		subscribers: make(map[uuid.UUID]map[chan Event]struct{}),
	}
}

// Subscribe подписывает пользователя на события и возвращает канал и функцию отписки.
func (h *Hub) Subscribe(userID uuid.UUID) (<-chan Event, func()) {
	ch := make(chan Event, 10)

	h.mu.Lock()
	defer h.mu.Unlock()

	userSubs, ok := h.subscribers[userID]
	if !ok {
		userSubs = make(map[chan Event]struct{})
		h.subscribers[userID] = userSubs
	}
	userSubs[ch] = struct{}{}

	return ch, func() {
		h.mu.Lock()
		defer h.mu.Unlock()

		if subs, exists := h.subscribers[userID]; exists {
			delete(subs, ch)
			if len(subs) == 0 {
				delete(h.subscribers, userID)
			}
		}
		close(ch)
	}
}

// Publish отправляет событие всем подписчикам пользователя.
func (h *Hub) Publish(userID uuid.UUID, event Event) {
	event.Timestamp = time.Now().UTC()

	h.mu.RLock()
	defer h.mu.RUnlock()

	subs, ok := h.subscribers[userID]
	if !ok {
		return
	}

	for ch := range subs {
		select {
		case ch <- event:
		default:
		}
	}
}

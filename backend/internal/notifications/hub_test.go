package notifications

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

// TestHubPublishSubscribe проверяет доставку событий подписчику.
func TestHubPublishSubscribe(t *testing.T) {
	hub := NewHub()
	userID := uuid.New()

	ch, unsubscribe := hub.Subscribe(userID)
	defer unsubscribe()

	hub.Publish(userID, Event{Type: "test"})

	select {
	case event := <-ch:
		if event.Type != "test" {
			t.Fatalf("expected event type test, got %s", event.Type)
		}
		if event.Timestamp.IsZero() {
			t.Fatal("expected timestamp to be set")
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("expected event to be delivered")
	}
}

// TestHubUnsubscribe проверяет закрытие канала после отписки.
func TestHubUnsubscribe(t *testing.T) {
	hub := NewHub()
	userID := uuid.New()

	ch, unsubscribe := hub.Subscribe(userID)
	unsubscribe()

	if _, ok := <-ch; ok {
		t.Fatal("expected channel to be closed")
	}
}

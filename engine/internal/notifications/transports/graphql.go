package transports

import (
	"context"
	"platrium/internal/graphql"
	"platrium/internal/identity"
	"platrium/internal/notifications"
	"platrium/internal/notifications/events"
	"sync"
)

// GraphQLTransport holds the local WebSocket channels and satisfies the NotificationTransport interface.
type GraphQLTransport struct {
	mu sync.RWMutex
	// subscribers maps UserID to a map of active channels expecting a generated DriveItemEvent
	subscribers map[string]map[chan *graphql.DriveItemEvent]struct{}
}

func NewGraphQLTransport() *GraphQLTransport {
	return &GraphQLTransport{
		subscribers: make(map[string]map[chan *graphql.DriveItemEvent]struct{}),
	}
}

func (t *GraphQLTransport) GetTransportType() notifications.TransportType {
	return notifications.TransportGraphQL
}

// DeliverEvent routes the unified NotificationEvent to active WebSockets.
func (t *GraphQLTransport) DeliverEvent(ctx context.Context, targets []*identity.Device, event notifications.NotificationEvent) error {
	t.mu.RLock()
	defer t.mu.RUnlock()

	var gqlModel *graphql.DriveItemEvent

	switch e := event.(type) {
	case events.DriveEvent:
		gqlModel = serializeDriveEvent(e)
	default:
		// Event doesn't support GraphQL, ignore it.
		return nil
	}

	for _, target := range targets {
		if subs, ok := t.subscribers[target.UserID]; ok {
			for ch := range subs {
				// Non-blocking send
				select {
				case ch <- gqlModel:
				default:
					// Channel full, drop the event.
				}
			}
		}
	}
	return nil
}

// Subscribe returns a channel for a specific user to listen to WebSocket events.
// Used directly by gqlgen.
func (t *GraphQLTransport) Subscribe(userID string) (<-chan *graphql.DriveItemEvent, func()) {
	t.mu.Lock()
	defer t.mu.Unlock()

	ch := make(chan *graphql.DriveItemEvent, 100)
	if _, ok := t.subscribers[userID]; !ok {
		t.subscribers[userID] = make(map[chan *graphql.DriveItemEvent]struct{})
	}
	t.subscribers[userID][ch] = struct{}{}

	unsub := func() {
		t.mu.Lock()
		defer t.mu.Unlock()
		if subs, ok := t.subscribers[userID]; ok {
			delete(subs, ch)
			if len(subs) == 0 {
				delete(t.subscribers, userID)
			}
		}
		close(ch)
	}

	return ch, unsub
}

func serializeDriveEvent(e events.DriveEvent) *graphql.DriveItemEvent {
	var gqlEventType graphql.DriveItemEventType
	switch e.EventType {
	case events.EventUpdated:
		gqlEventType = graphql.DriveItemEventTypeUpdated
	case events.EventDeleted:
		gqlEventType = graphql.DriveItemEventTypeDeleted
	default:
		gqlEventType = graphql.DriveItemEventTypeUpdated
	}

	return &graphql.DriveItemEvent{
		EventType: gqlEventType,
		ItemID:    e.ItemID,
		DeletedID: e.DeletedID,
	}
}

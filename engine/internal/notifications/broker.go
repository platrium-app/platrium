package notifications

import (
	"context"
	"platrium/internal/identity"
)

// Broker routes internal events to all configured transports.
type Broker struct {
	transports []Transport
}

// NewBroker initializes the broker with a list of transports.
func NewBroker(transports ...Transport) *Broker {
	return &Broker{
		transports: transports,
	}
}

// Publish pushes an event to targeted users.
// The broker groups devices by TransportType and forwards the event.
func (b *Broker) Publish(ctx context.Context, targetUserIDs []string, event NotificationEvent) {
	// TODO: b.db.GetDevicesForUsers(ctx, targetUserIDs)
	// For now, hardcode fake GraphQL devices since WebSockets live in RAM
	var devices []*identity.Device
	for _, id := range targetUserIDs {
		// We are assuming a connected GRAPHQL device for now
		devices = append(devices, &identity.Device{
			UserID:                    id,
			NotificationTransportType: string(TransportGraphQL),
		})
	}

	grouped := make(map[TransportType][]*identity.Device)
	for _, d := range devices {
		tt := TransportType(d.NotificationTransportType)
		grouped[tt] = append(grouped[tt], d)
	}

	for _, t := range b.transports {
		targets := grouped[t.GetTransportType()]
		if len(targets) > 0 {
			go func(transport Transport, tgts []*identity.Device) {
				_ = transport.DeliverEvent(ctx, tgts, event)
			}(t, targets)
		}
	}
}

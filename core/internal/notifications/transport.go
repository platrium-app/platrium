package notifications

import (
	"context"
	"platrium/internal/identity"
)

// TransportType identifies the delivery mechanism.
type TransportType string

const (
	TransportGraphQL TransportType = "GRAPHQL"
	TransportAPNS    TransportType = "APNS"
	TransportFCM     TransportType = "FCM"
)

// NotificationEvent is a marker interface for any domain event that needs to be delivered.
// Transports will type-switch on this to serialize it for their specific network.
type NotificationEvent any

// NotificationTransport defines the contract for any notification delivery mechanism.
type NotificationTransport interface {
	GetTransportType() TransportType
	DeliverEvent(ctx context.Context, targets []*identity.Device, event NotificationEvent) error
}

package transports

import (
	"context"
	"encoding/json"
	"platrium/internal/identity"
	"platrium/internal/notifications"
	"platrium/internal/notifications/events"
)

// APNSTransport delivers notifications to Apple devices via APNs.
type APNSTransport struct {
	// e.g. apnsClient *apns2.Client
}

func NewAPNSTransport() *APNSTransport {
	return &APNSTransport{}
}

func (t *APNSTransport) GetTransportType() notifications.TransportType {
	return notifications.TransportAPNS
}

func (t *APNSTransport) DeliverEvent(ctx context.Context, targets []*identity.Device, event notifications.NotificationEvent) error {
	// Serialize the event for APNS based on its type
	var payload map[string]interface{}

	switch e := event.(type) {
	case events.DriveEvent:
		payload = map[string]interface{}{
			"aps": map[string]interface{}{
				"content-available": 1, // Silent push for background sync
			},
			"platrium": map[string]interface{}{
				"type":      "drive_event",
				"event":     string(e.EventType),
				"item_id":   e.ItemID,
				"parent_id": e.ParentID,
			},
		}
	default:
		// Unknown event type, ignore
		return nil
	}

	bytes, _ := json.Marshal(payload)
	_ = bytes

	// TODO: For each target in targets:
	// 1. target.Metadata["token"]
	// 2. Fire the HTTP request to Apple using the generated JSON payload.
	return nil
}

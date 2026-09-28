package identity

import (
	"context"
	"fmt"

	"platrium/internal/infra/graph"
)

// Device represents a user's registered physical device for push notifications, MDM, etc.
type Device struct {
	ID                        string            `json:"id"`
	UserID                    string            `json:"userId"`
	NotificationTransportType string            `json:"notificationTransportType"` // Maps to notifications.TransportType (e.g. "GRAPHQL", "APNS")
	Metadata                  map[string]string `json:"metadata"`
	CreatedAt                 int64             `json:"createdAt"`
}

// DeviceStore defines operations for managing user devices.
type DeviceStore interface {
	RegisterDevice(ctx context.Context, userID string, req RegisterDeviceReq) (*Device, error)
	DeregisterDevice(ctx context.Context, deviceID string) error
	GetDevicesForUser(ctx context.Context, userID string) ([]*Device, error)
}

type RegisterDeviceReq struct {
	NotificationTransportType string
	Metadata                  map[string]string
}

// GraphDeviceStore manages Device nodes in the GraphDB.
type GraphDeviceStore struct {
	store graph.Graph
}

func NewGraphDeviceStore(store graph.Graph) *GraphDeviceStore {
	return &GraphDeviceStore{store: store}
}

func (s *GraphDeviceStore) RegisterDevice(ctx context.Context, userID string, req RegisterDeviceReq) (*Device, error) {
	// TODO: implement
	return nil, fmt.Errorf("not implemented")
}

func (s *GraphDeviceStore) DeregisterDevice(ctx context.Context, deviceID string) error {
	// TODO: implement
	return fmt.Errorf("not implemented")
}

// GetDevicesForUser fetches all devices belonging to a specific user.
func (s *GraphDeviceStore) GetDevicesForUser(ctx context.Context, userID string) ([]*Device, error) {
	query := `
		MATCH (d:Device)-[:BELONGS_TO]->(u:User {id: $userId})
		RETURN 
			d.id AS id, 
			u.id AS userId, 
			d.transportType AS notificationTransportType, 
			d.metadata AS metadata, 
			d.createdAt AS createdAt
	`

	params := map[string]interface{}{
		"userId": userID,
	}

	var devices []*Device
	err := s.store.ReadTx(ctx, func(tx graph.Tx) error {
		res, err := tx.Query(ctx, query, params)
		if err != nil {
			return err
		}
		defer res.Close()

		for res.Next() {
			var device Device
			if err := res.Scan(&device); err != nil {
				return fmt.Errorf("failed to scan device: %w", err)
			}
			devices = append(devices, &device)
		}

		return res.Err()
	})

	if err != nil {
		return nil, fmt.Errorf("failed to get devices for user: %w", err)
	}

	return devices, nil
}

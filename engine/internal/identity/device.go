package identity

import (
	"context"
	"fmt"
	"time"

	"platrium/internal/infra/db"
	"platrium/internal/infra/db/ent/device"
)

// Device represents a user's registered physical device for push notifications, MDM, etc.
type Device struct {
	ID                        string            `json:"id"`
	UserID                    string            `json:"user_id"`
	NotificationTransportType string            `json:"notification_transport_type"` // Maps to notifications.TransportType (e.g. "GRAPHQL", "APNS")
	Metadata                  map[string]string `json:"metadata"`
	CreatedAt                 time.Time         `json:"created_at"`
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

// EntDeviceStore manages Device records.
type EntDeviceStore struct {
	db *db.DB
}

var _ DeviceStore = (*EntDeviceStore)(nil)

func NewEntDeviceStore(d *db.DB) *EntDeviceStore {
	return &EntDeviceStore{db: d}
}

func (s *EntDeviceStore) RegisterDevice(ctx context.Context, userID string, req RegisterDeviceReq) (*Device, error) {
	// TODO: implement
	return nil, fmt.Errorf("not implemented")
}

func (s *EntDeviceStore) DeregisterDevice(ctx context.Context, deviceID string) error {
	// TODO: implement
	return fmt.Errorf("not implemented")
}

// GetDevicesForUser fetches all devices belonging to a specific user.
func (s *EntDeviceStore) GetDevicesForUser(ctx context.Context, userID string) ([]*Device, error) {
	rows, err := s.db.Device.Query().
		Where(device.UserID(userID)).
		Order(device.ByCreatedAt(), device.ByID()).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get devices for user: %w", err)
	}

	devices := make([]*Device, 0, len(rows))
	for _, d := range rows {
		devices = append(devices, &Device{
			ID:                        d.ID,
			UserID:                    d.UserID,
			NotificationTransportType: d.TransportType,
			Metadata:                  d.Metadata,
			CreatedAt:                 d.CreatedAt,
		})
	}
	return devices, nil
}

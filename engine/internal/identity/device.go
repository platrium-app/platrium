package identity

import (
	"context"
	"fmt"
	"time"

	"platrium/internal/infra/db"
	"platrium/internal/infra/db/ent"
	"platrium/internal/infra/db/ent/device"
)

// Device is a user's registered native client (iOS, macOS, Android app).
type Device struct {
	ID                        string    `json:"id"`
	UserID                    string    `json:"user_id"`
	Name                      string    `json:"name"`
	Platform                  string    `json:"platform"`
	AppVersion                string    `json:"app_version"`
	NotificationTransportType string    `json:"notification_transport_type"` // Maps to notifications.TransportType (APNS, FCM); empty until a push token is registered
	PushToken                 string    `json:"-"`
	CreatedAt                 time.Time `json:"created_at"`
}

// RegisterDeviceReq describes a device being registered at sign-in.
type RegisterDeviceReq struct {
	Name       string
	Platform   string
	AppVersion string
}

// DeviceStore manages the push-related state of registered devices. Devices
// are created and deleted together with their AuthToken (see auth/token).
type DeviceStore interface {
	SetPush(ctx context.Context, tenantID, deviceID, transport, pushToken string) error
	ClearPush(ctx context.Context, tenantID, deviceID string) error
	GetDevicesForUser(ctx context.Context, userID string) ([]*Device, error)
}

// EntDeviceStore manages Device records.
type EntDeviceStore struct {
	db *db.DB
}

var _ DeviceStore = (*EntDeviceStore)(nil)

func NewEntDeviceStore(d *db.DB) *EntDeviceStore {
	return &EntDeviceStore{db: d}
}

// CreateTx inserts a device within the caller's transaction. The caller is
// responsible for having verified that userID belongs to tenantID.
func (s *EntDeviceStore) CreateTx(ctx context.Context, tx *ent.Tx, tenantID, userID string, req RegisterDeviceReq) (*ent.Device, error) {
	d, err := tx.Device.Create().
		SetTenantID(tenantID).
		SetUserID(userID).
		SetName(req.Name).
		SetPlatform(req.Platform).
		SetAppVersion(req.AppVersion).
		Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create device: %w", err)
	}
	return d, nil
}

// SetPush records where to deliver push notifications for a device.
func (s *EntDeviceStore) SetPush(ctx context.Context, tenantID, deviceID, transport, pushToken string) error {
	n, err := s.db.Device.Update().
		Where(device.ID(deviceID), device.TenantID(tenantID)).
		SetPushTransport(transport).
		SetPushToken(pushToken).
		Save(ctx)
	if err != nil {
		return fmt.Errorf("failed to set push token: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("%w: device not found", ErrNotFound)
	}
	return nil
}

// ClearPush stops push delivery to a device.
func (s *EntDeviceStore) ClearPush(ctx context.Context, tenantID, deviceID string) error {
	n, err := s.db.Device.Update().
		Where(device.ID(deviceID), device.TenantID(tenantID)).
		ClearPushTransport().
		ClearPushToken().
		Save(ctx)
	if err != nil {
		return fmt.Errorf("failed to clear push token: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("%w: device not found", ErrNotFound)
	}
	return nil
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
		dev := &Device{
			ID:         d.ID,
			UserID:     d.UserID,
			Name:       d.Name,
			Platform:   d.Platform,
			AppVersion: d.AppVersion,
			CreatedAt:  d.CreatedAt,
		}
		if d.PushTransport != nil {
			dev.NotificationTransportType = *d.PushTransport
		}
		if d.PushToken != nil {
			dev.PushToken = *d.PushToken
		}
		devices = append(devices, dev)
	}
	return devices, nil
}

package fsops

import "time"

// Folder represents a folder node in the graph database.
type Folder struct {
	ID           string    `json:"id"`
	TenantID     string    `json:"tenant_id"`
	ParentID     *string   `json:"parent_id,omitempty"`
	Name         string    `json:"name"`
	Type         string    `json:"type"` // e.g., "FOLDER" or "PRIVATE_DRIVE"
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`

	// Drive specific metadata, if this folder acts as a root drive
	DriveMetadata *DriveMetadata `json:"drive_metadata,omitempty"`
}

type DriveMetadata struct {
	DriveType    string `json:"drive_type"`
	StorageUsed  int64  `json:"storage_used"`
	StorageQuota *int64 `json:"storage_quota,omitempty"`
}


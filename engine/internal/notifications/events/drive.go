package events

import (
	"time"
)

type DriveEventType string

const (
	EventUpdated DriveEventType = "UPDATED"
	EventDeleted DriveEventType = "DELETED"
)

// DriveEvent represents an internal file system event.
// Notice it does NOT contain TargetUserIDs. It is a pure domain event describing WHAT happened.
type DriveEvent struct {
	EventType DriveEventType
	ItemID    string
	ParentID  *string
	DeletedID *string
	Timestamp time.Time
}

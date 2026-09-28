package graphql

import (
	"platrium/internal/fsops"
	"platrium/internal/notifications"
)

type SubscriptionManager interface {
	Subscribe(userID string) (<-chan *DriveItemEvent, func())
}

// This file will not be regenerated automatically.
//
// It serves as dependency injection for your app, add any dependencies you require
// here.

type Resolver struct {
	FSOps       *fsops.FSOps
	Broker      *notifications.Broker
	SubsManager SubscriptionManager
}

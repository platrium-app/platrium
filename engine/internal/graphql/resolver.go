package graphql

import (
	"platrium/internal/auth"
	"platrium/internal/auth/actor"
	"platrium/internal/authz"
	"platrium/internal/fsops"
	"platrium/internal/identity"
	"platrium/internal/notifications"
	"platrium/internal/orchestrator"
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
	Authz       authz.Authorizer
	Actors      *actor.Resolver
	DriveOrch   *orchestrator.DriveOrchestrator
	Broker      *notifications.Broker
	SubsManager SubscriptionManager
	UserStore   *identity.UserStore
	GroupStore  *identity.GroupStore
	TenantStore *identity.TenantStore
	IdpStore    *auth.IdpStore
	UserAdmin   *orchestrator.UserAdmin
	IdpAdmin    *orchestrator.IdpAdmin
}

package orchestrator_test

import (
	"platrium/internal/authz"
	"platrium/internal/fsops"
	"platrium/internal/identity"
	"platrium/internal/infra/db"
	"platrium/internal/orchestrator"
)

type driveOrchestrator = orchestrator.DriveOrchestrator

func newDriveOrchestratorWith(d *db.DB, fs *fsops.FSOps, az authz.Authorizer) *driveOrchestrator {
	return orchestrator.NewDriveOrchestrator(d, fs, az, identity.NewUserStore(d), identity.NewPolicyStore(d))
}

func newDriveOrchestrator(d *db.DB, fs *fsops.FSOps, az authz.Authorizer) *driveOrchestrator {
	return newDriveOrchestratorWith(d, fs, az)
}

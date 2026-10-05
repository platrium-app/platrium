package sqlauthz_test

import (
	"context"
	"testing"

	"platrium/internal/authz"
	"platrium/internal/authz/authztest"
	"platrium/internal/infra/db/ent/idpprovider"
)

// sqlWorld lets the shared authztest scenarios build their fixtures in SQL.
type sqlWorld struct{ e *env }

func (w sqlWorld) Authorizer() authz.Authorizer { return w.e.az }

func (w sqlWorld) NewTenant(t *testing.T) string { return w.e.tenant(t, "t").id }

func (w sqlWorld) tn(id string) tenant {
	idp, err := w.e.db.IdpProvider.Query().Where(idpprovider.TenantID(id)).Only(t0())
	if err != nil {
		panic(err)
	}
	return tenant{id: id, idp: idp.ID}
}

func (w sqlWorld) NewUser(t *testing.T, tenantID, name string) string {
	return w.e.user(t, w.tn(tenantID), name)
}
func (w sqlWorld) NewGroup(t *testing.T, tenantID, name string) string {
	return w.e.group(t, w.tn(tenantID), name)
}
func (w sqlWorld) NewPrivateDrive(t *testing.T, tenantID, ownerID string) string {
	return w.e.drive(t, w.tn(tenantID), ownerID)
}
func (w sqlWorld) NewSharedDrive(t *testing.T, tenantID, name, adminID string) string {
	return w.e.sharedDrive(t, w.tn(tenantID), name, adminID)
}
func (w sqlWorld) NewFolder(t *testing.T, tenantID, parentID string) string {
	item, err := w.e.db.DriveItem.Get(t0(), parentID)
	if err != nil {
		t.Fatal(err)
	}
	return w.e.folder(t, w.tn(tenantID), item.DriveID, parentID, "f")
}

func TestConformance(t *testing.T) {
	authztest.Run(t, func(t *testing.T) authztest.World { return sqlWorld{newEnv(t)} })
}

func t0() context.Context { return context.Background() }

package sqlauthz

import (
	"context"
	"database/sql"
	"fmt"

	"platrium/internal/authz"
	"platrium/internal/infra/db/ent"
	"platrium/internal/infra/db/ent/drive"
	"platrium/internal/infra/db/ent/grant"
)

// Every write to an item's grants goes through the same four steps, so that
// the rules in authz.CheckChange cannot be skipped by a writer that forgets
// them:
//
//  1. changeTx opens a transaction,
//  2. lockDrive serializes writers on the item's drive,
//  3. checkChange loads the grants as they are, builds them as they would be,
//     and runs authz.CheckChange,
//  4. the writer writes.
//
// The rules themselves live in package authz and know nothing of SQL.

// changeTx runs a grant change in a transaction. READ COMMITTED so that reads
// after the drive lock see other writers' committed work on MySQL and MariaDB
// (see fsops.MoveItem).
func (a *Authorizer) changeTx(ctx context.Context, fn func(tx *ent.Tx) error) error {
	return a.db.WithTxOpts(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted}, fn)
}

// lockDrive takes the drive's row lock, which serializes every grant change in
// the drive. Two admins removing each other at once cannot both pass the rules.
// SQLite has no row locks and serializes writers anyway.
func (a *Authorizer) lockDrive(ctx context.Context, tx *ent.Tx, driveID string) (*ent.Drive, error) {
	q := tx.Drive.Query().Where(drive.ID(driveID))
	if a.db.RowLocks() {
		q = q.ForUpdate()
	}
	d, err := q.Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, fmt.Errorf("%w: drive", authz.ErrNotFound)
		}
		return nil, err
	}
	return d, nil
}

// pending is a change a writer is about to make.
type pending struct {
	op        authz.ChangeOp
	actor     authz.Principal
	actorCaps authz.Capability
	item      *ent.DriveItem
	drive     *ent.Drive
	// after builds the item's grants as they will be, from how they are now.
	after func(before []authz.Grant) []authz.Grant
}

// checkChange loads the item's grants and runs the rules over the change. Call
// it with the drive locked.
func (a *Authorizer) checkChange(ctx context.Context, tx *ent.Tx, p pending) error {
	rows, err := tx.Grant.Query().Where(grant.ResourceID(p.item.ID)).All(ctx)
	if err != nil {
		return err
	}
	before := make([]authz.Grant, 0, len(rows))
	for _, g := range rows {
		before = append(before, *grantFromEnt(g))
	}

	roles, err := roleContext(ctx, tx, p.item)
	if err != nil {
		return err
	}
	t, err := tx.Tenant.Get(ctx, p.item.TenantID)
	if err != nil {
		return err
	}

	return authz.CheckChange(authz.Change{
		Op:        p.op,
		Actor:     p.actor,
		ActorCaps: p.actorCaps,
		Item: authz.ItemFacts{
			ID:          p.item.ID,
			IsDriveRoot: p.item.ParentID == nil,
			SharedDrive: p.drive.Type == drive.TypeSHARED,
			Roles:       roles,
		},
		PublicSharingAllowed: t.AllowPublicSharing,
		Before:               before,
		After:                p.after(before),
	})
}

// withGrant returns grants with g replacing any grant to the same subject.
func withGrant(grants []authz.Grant, g authz.Grant) []authz.Grant {
	out := withoutSubject(grants, g.Subject)
	return append(out, g)
}

// withoutSubject returns grants without the one to s.
func withoutSubject(grants []authz.Grant, s authz.Subject) []authz.Grant {
	out := make([]authz.Grant, 0, len(grants)+1)
	for _, g := range grants {
		if g.Subject != s {
			out = append(out, g)
		}
	}
	return out
}

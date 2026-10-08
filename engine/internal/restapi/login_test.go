package restapi_test

import (
	"context"
	"testing"
	"time"

	"platrium/internal/auth"
	"platrium/internal/auth/actor"
	"platrium/internal/auth/protocol/local"
	"platrium/internal/authz/sqlauthz"
	"platrium/internal/identity"
	"platrium/internal/infra/db/dbtest"
	"platrium/internal/restapi"
)

// "No such user" must take about as long as "wrong password", or a stopwatch
// tells an attacker which emails have accounts.
func TestLoginDoesNotRevealWhichEmailsExist(t *testing.T) {
	t.Setenv("PLATRIUM_SECRET_KEY", "test-secret-key")
	ctx := context.Background()
	d := dbtest.New(t)
	users := identity.NewUserStore(d)
	api := restapi.NewRestAPI(nil, actor.NewResolver(sqlauthz.New(d), users), nil, nil, nil, auth.NewIdpStore(d), users, local.NewLocalUserStore(d, nil), nil, nil, nil, nil)

	tn := d.Tenant.Create().SetAlias("acme").SetName("acme").SaveX(ctx)
	idp := d.IdpProvider.Create().SetTenantID(tn.ID).SetType("LOCAL").SetName("l").SaveX(ctx)
	u := d.User.Create().SetTenantID(tn.ID).SetIdpID(idp.ID).SetExternalID("bob@acme.com").SetEmail("bob@acme.com").SetDisplayName("bob").SaveX(ctx)
	hash, err := local.HashPassword("right-password")
	if err != nil {
		t.Fatal(err)
	}
	d.LocalCredential.Create().SetTenantID(tn.ID).SetUserID(u.ID).SetPasswordHash(hash).ExecX(ctx)

	try := func(email string) time.Duration {
		t.Helper()
		pw := "wrong-password"
		start := time.Now()
		res, err := api.AuthLocalUserLogin(ctx, restapi.AuthLocalUserLoginRequestObject{Body: &restapi.AuthLocalUserLoginJSONRequestBody{IdpId: idp.ID, Email: email, Password: &pw}})
		took := time.Since(start)
		if err != nil {
			t.Fatal(err)
		}
		if _, ok := res.(restapi.AuthLocalUserLogin401JSONResponse); !ok {
			t.Fatalf("%s: want a refusal, got %T", email, res)
		}
		return took
	}

	known := try("bob@acme.com") // a real bcrypt comparison
	unknown := try("nobody@acme.com")
	if unknown < known/2 {
		t.Errorf("an unknown email answered in %v, a wrong password in %v: the difference gives accounts away", unknown, known)
	}
}

package graphql

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/99designs/gqlgen/client"
	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/handler/transport"

	"platrium/internal/auth"
	"platrium/internal/auth/protocol/local"
	"platrium/internal/auth/session"
	"platrium/internal/identity"
	"platrium/internal/orchestrator"
)

// adminClient runs real GraphQL documents through the executable schema, so the
// @requires directive is part of what is tested. The X-User header stands in
// for the session middleware.
func (h *harness) adminClient(t *testing.T) func(userID, query string, vars ...client.Option) (map[string]any, error) {
	t.Helper()
	users := identity.NewUserStore(h.db)
	lus := local.NewLocalUserStore(h.db, orchestrator.NewUserOrchestrator(users, h.r.FSOps))
	h.r.UserAdmin = orchestrator.NewUserAdmin(h.db, users, auth.NewIdpStore(h.db), lus)

	srv := handler.New(NewExecutableSchema(Config{Resolvers: h.r, Directives: h.r.Directives()}))
	srv.AddTransport(transport.POST{})
	srv.SetErrorPresenter(ErrorPresenter)
	c := client.New(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if id := r.Header.Get("X-User"); id != "" {
			r = r.WithContext(session.WithSession(r.Context(), &session.PlatriumSession{UserID: id, TenantID: h.tenant}))
		}
		srv.ServeHTTP(w, r)
	}))
	return func(userID, query string, vars ...client.Option) (map[string]any, error) {
		var resp map[string]any
		opts := append([]client.Option{}, vars...)
		if userID != "" {
			opts = append(opts, client.AddHeader("X-User", userID))
		}
		err := c.Post(query, &resp, opts...)
		return resp, err
	}
}

func mustFail(t *testing.T, what string, err error, code string) {
	t.Helper()
	if err == nil || !strings.Contains(err.Error(), `"code":"`+code+`"`) {
		t.Errorf("%s: want a %s error, got %v", what, code, err)
	}
}

const usersQuery = `query { adminUsers(first: 50) { totalCount pageInfo { hasNextPage endCursor } edges { cursor node { id email role disabled editable manageable source { name type isLocal } } } } }`

func TestAdminFieldsAreGatedByPermission(t *testing.T) {
	h := newHarness(t)
	do := h.adminClient(t)

	_, err := do("", usersQuery)
	mustFail(t, "no session", err, "UNAUTHENTICATED")
	_, err = do(h.bob, usersQuery)
	mustFail(t, "a member listing users", err, "FORBIDDEN")
	_, err = do(h.bob, `query { adminIdentitySources { id } }`)
	mustFail(t, "a member listing sources", err, "FORBIDDEN")

	// The directive stops the field before its resolver runs: nothing is created.
	_, err = do(h.bob, `mutation { createLocalUser(input: {email: "x@acme.com", displayName: "X", password: "password-2"}) { id } }`)
	mustFail(t, "a member creating a user", err, "FORBIDDEN")
	_, err = do(h.bob, `mutation { setUserDisabled(id: "`+h.carol+`", disabled: true) { id } }`)
	mustFail(t, "a member disabling a user", err, "FORBIDDEN")
	if n, _ := h.db.User.Query().Count(context.Background()); n != 4 {
		t.Errorf("users = %d, a refused request must change nothing", n)
	}

	resp, err := do(h.admin, usersQuery)
	if err != nil {
		t.Fatal(err)
	}
	conn := resp["adminUsers"].(map[string]any)
	if conn["totalCount"].(float64) != 4 || len(conn["edges"].([]any)) != 4 {
		t.Fatalf("an admin sees the 4 users: %v", conn)
	}
	src := conn["edges"].([]any)[0].(map[string]any)["node"].(map[string]any)["source"].(map[string]any)
	if src["type"] != "LOCAL" || src["isLocal"] != true {
		t.Errorf("source: %v", src)
	}
}

func TestAdminCreateDisableAndMe(t *testing.T) {
	h := newHarness(t)
	do := h.adminClient(t)

	resp, err := do(h.admin, `mutation { createLocalUser(input: {email: "New@Acme.com", displayName: "New", password: "password-2"}) { id email role editable source { isLocal } } }`)
	if err != nil {
		t.Fatal(err)
	}
	created := resp["createLocalUser"].(map[string]any)
	if created["email"] != "new@acme.com" || created["role"] != "MEMBER" || created["editable"] != true {
		t.Fatalf("created: %v", created)
	}
	_, err = do(h.admin, `mutation { createLocalUser(input: {email: "new@acme.com", displayName: "Dup", password: "password-2"}) { id } }`)
	mustFail(t, "duplicate", err, "CONFLICT")
	_, err = do(h.admin, `mutation { createLocalUser(input: {email: "bad", displayName: "Bad", password: "password-2"}) { id } }`)
	mustFail(t, "invalid email", err, "BAD_REQUEST")

	resp, err = do(h.admin, `query { me { role permissions assignableRoles } }`)
	if err != nil {
		t.Fatal(err)
	}
	me := resp["me"].(map[string]any)
	perms := me["permissions"].([]any)
	has := func(p string) bool {
		for _, x := range perms {
			if x == p {
				return true
			}
		}
		return false
	}
	if me["role"] != "ADMIN" || !has("USERS_READ") || !has("USERS_DISABLE") || has("ROLES_ASSIGN") || has("TENANTS_MANAGE") || len(me["assignableRoles"].([]any)) != 0 {
		t.Errorf("an admin's me: %v", me)
	}
	resp, err = do(h.bob, `query { me { role permissions } }`)
	if err != nil || len(resp["me"].(map[string]any)["permissions"].([]any)) != 0 {
		t.Errorf("a member's me: %v %v", resp, err)
	}

	// Disabling signs the user out of everything at once.
	if _, err := do(h.admin, `mutation { setUserDisabled(id: "`+h.bob+`", disabled: true) { disabled disabledAt } }`); err != nil {
		t.Fatal(err)
	}
	_, err = do(h.bob, `query { me { userId } }`)
	mustFail(t, "a disabled user's next request", err, "UNAUTHENTICATED")
	resp, err = do(h.admin, `mutation { setUserDisabled(id: "`+h.bob+`", disabled: false) { disabled } }`)
	if err != nil || resp["setUserDisabled"].(map[string]any)["disabled"] != false {
		t.Fatalf("enable: %v %v", resp, err)
	}
	if _, err := do(h.bob, `query { me { userId } }`); err != nil {
		t.Errorf("an enabled user is back: %v", err)
	}
	_, err = do(h.admin, `mutation { setUserDisabled(id: "`+h.admin+`", disabled: true) { id } }`)
	mustFail(t, "disabling yourself", err, "FORBIDDEN")
	_, err = do(h.admin, `mutation { setUserDisabled(id: "nope", disabled: true) { id } }`)
	mustFail(t, "unknown user", err, "NOT_FOUND")
}

func TestAdminUsersPagination(t *testing.T) {
	h := newHarness(t)
	do := h.adminClient(t)

	var seen []string
	after := ""
	for i := 0; i < 5; i++ {
		resp, err := do(h.admin, `query($after: String) { adminUsers(first: 3, after: $after) { totalCount pageInfo { hasNextPage endCursor } edges { node { email } } } }`, client.Var("after", nilIfEmpty(after)))
		if err != nil {
			t.Fatal(err)
		}
		conn := resp["adminUsers"].(map[string]any)
		for _, e := range conn["edges"].([]any) {
			seen = append(seen, e.(map[string]any)["node"].(map[string]any)["email"].(string))
		}
		pi := conn["pageInfo"].(map[string]any)
		if pi["hasNextPage"] != true {
			break
		}
		after = pi["endCursor"].(string)
	}
	if len(seen) != 4 {
		t.Errorf("paging must visit each of the 4 users exactly once: %v", seen)
	}
	_, err := do(h.admin, `query { adminUsers(after: "garbage!") { totalCount } }`)
	mustFail(t, "a bad cursor", err, "BAD_REQUEST")
}

func nilIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}

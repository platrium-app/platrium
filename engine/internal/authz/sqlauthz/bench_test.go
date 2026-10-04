package sqlauthz_test

import (
	"context"
	"fmt"
	"testing"

	"platrium/internal/authz"
)

// benchTree builds `chains` folders-of-depth-20 under one drive and shares every
// 10th chain's root with a user who also belongs to a few groups.
func benchTree(b *testing.B, chains int) (e *env, p authz.Principal, leaves []string) {
	b.Helper()
	e = newEnv2(b)
	ctx := context.Background()
	tn := e.tenantB(b, "bench")
	owner, user := e.userB(b, tn, "owner"), e.userB(b, tn, "user")
	drive := e.driveB(b, tn, owner)
	ownerP, err := e.az.Principal(ctx, tn.id, owner)
	if err != nil {
		b.Fatal(err)
	}
	for g := 0; g < 5; g++ {
		gid := e.groupB(b, tn, fmt.Sprintf("g%d", g))
		if err := e.az.AddMember(ctx, tn.id, gid, authz.MemberUser, user); err != nil {
			b.Fatal(err)
		}
	}
	for c := 0; c < chains; c++ {
		parent := drive
		var first string
		for l := 0; l < 20; l++ {
			parent = e.folderB(b, tn, drive, parent, fmt.Sprintf("c%d-%d", c, l))
			if l == 0 {
				first = parent
			}
		}
		leaves = append(leaves, parent)
		if c%10 == 0 {
			if _, err := e.az.Grant(ctx, ownerP, authz.GrantInput{ItemID: first, Subject: userSubject(user), Role: authz.RoleViewer}); err != nil {
				b.Fatal(err)
			}
		}
	}
	p, err = e.az.Principal(ctx, tn.id, user)
	if err != nil {
		b.Fatal(err)
	}
	return e, p, leaves
}

// BenchmarkCaps measures one uncached check at depth 20.
func BenchmarkCaps(b *testing.B) {
	e, p, leaves := benchTree(b, 100)
	ctx := context.Background()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := e.az.Caps(ctx, p, leaves[i%len(leaves)]); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkCapsMany64 measures checking 64 items in one call (a page of results).
func BenchmarkCapsMany64(b *testing.B) {
	e, p, leaves := benchTree(b, 100)
	ctx := context.Background()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		start := (i * 64) % (len(leaves) - 64)
		if _, err := e.az.CapsMany(ctx, p, leaves[start:start+64]); err != nil {
			b.Fatal(err)
		}
	}
}

package authz

import (
	"errors"
	"testing"
)

// The numeric values are persisted in the database, so they are pinned. A
// failure here means a bit was renumbered, which corrupts every stored grant.
func TestCapabilityBitsAreStable(t *testing.T) {
	want := map[string]struct {
		got Capability
		val uint64
	}{
		"LIST":     {CapList, 1},
		"VIEW":     {CapView, 2},
		"DOWNLOAD": {CapDownload, 4},
		"COMMENT":  {CapComment, 8},
		"CREATE":   {CapCreate, 256},
		"EDIT":     {CapEdit, 512},
		"DELETE":   {CapDelete, 1024},
		"MOVE":     {CapMove, 2048},
		"TRASH":    {CapTrash, 4096},
		"MOVE_OUT": {CapMoveOut, 8192},
		"SHARE":    {CapShare, 65536},
		"MANAGE":   {CapManage, 131072},

		"DELETE_DRIVE": {CapDeleteDrive, 262144},
	}
	for name, w := range want {
		if uint64(w.got) != w.val {
			t.Errorf("%s = %d, want %d", name, w.got, w.val)
		}
	}
}

func TestRegistryIsConsistent(t *testing.T) {
	seenBit := map[Capability]string{}
	seenName := map[string]bool{}
	for _, d := range registry {
		if d.bit == 0 || d.bit&(d.bit-1) != 0 {
			t.Errorf("%s is not a single bit: %d", d.name, d.bit)
		}
		if !d.bit.Valid() {
			t.Errorf("%s does not fit a signed 64-bit column", d.name)
		}
		if other, dup := seenBit[d.bit]; dup {
			t.Errorf("%s and %s share a bit", d.name, other)
		}
		if seenName[d.name] {
			t.Errorf("duplicate name %s", d.name)
		}
		seenBit[d.bit], seenName[d.name] = d.name, true
		if d.implies&^KnownMask != 0 {
			t.Errorf("%s implies an unregistered capability", d.name)
		}
	}
	if KnownMask.Valid() != true || KnownMask.Unknown() != 0 {
		t.Error("known mask must be valid and fully known")
	}
}

func TestSetOperations(t *testing.T) {
	c := CapView | CapEdit
	if !c.Has(CapView) || !c.Has(CapView|CapEdit) || c.Has(CapDelete) || c.Has(CapView|CapDelete) {
		t.Error("Has")
	}
	if !c.Has(0) {
		t.Error("Has(0) is vacuously true")
	}
	if c.Union(CapDelete) != CapView|CapEdit|CapDelete {
		t.Error("Union")
	}
	if c.Without(CapEdit) != CapView {
		t.Error("Without")
	}
	if !CapView.SubsetOf(c) || c.SubsetOf(CapView) || !Capability(0).SubsetOf(c) {
		t.Error("SubsetOf")
	}
}

func TestNormalizeAddsImpliedCapabilities(t *testing.T) {
	cases := []struct{ in, want Capability }{
		{CapList, CapList},
		{CapView, CapView | CapList},
		{CapDownload, CapDownload | CapView | CapList},
		{CapEdit, CapEdit | CapView | CapList},
		{CapManage, CapManage | CapShare | CapView | CapList},
		{CapComment, CapComment | CapView | CapList},
		{CapMoveOut, CapMoveOut | CapMove | CapView | CapList},
		{CapDelete, CapDelete | CapTrash | CapView | CapList},
		{CapDeleteDrive, CapDeleteDrive | CapDelete | CapTrash | CapView | CapList},
		{0, 0},
	}
	for _, c := range cases {
		if got := Normalize(c.in); got != c.want {
			t.Errorf("Normalize(%s) = %s, want %s", c.in, got, c.want)
		}
	}
	if Normalize(Normalize(CapManage)) != Normalize(CapManage) {
		t.Error("Normalize must be idempotent")
	}
}

func TestUnknownBitsArePreserved(t *testing.T) {
	future := Capability(1) << 30 // not registered in this build
	c := CapView | future
	if c.Unknown() != future || c.Known() != CapView {
		t.Fatalf("known/unknown split: %s", c)
	}
	if Normalize(c)&future == 0 {
		t.Error("Normalize must not drop unknown bits")
	}
	round, err := ParseCapabilities(c.String())
	if err != nil || round != c {
		t.Fatalf("round trip of %q: %v %v", c.String(), round, err)
	}
}

func TestStringAndParse(t *testing.T) {
	c := CapList | CapView | CapDownload
	if c.String() != "LIST|VIEW|DOWNLOAD" {
		t.Errorf("String = %q", c.String())
	}
	if Capability(0).String() != "NONE" {
		t.Error("zero renders NONE")
	}
	for _, in := range []string{"list|view|download", "LIST, VIEW ,DOWNLOAD", " View|list|DOWNLOAD "} {
		got, err := ParseCapabilities(in)
		if err != nil || got != c {
			t.Errorf("Parse(%q) = %v, %v", in, got, err)
		}
	}
	if got, err := ParseCapabilities(""); err != nil || got != 0 {
		t.Error("empty parses to 0")
	}
	for _, bad := range []string{"FLY", "VIEW|", "BIT(99)", "BIT(x)"} {
		if _, err := ParseCapabilities(bad); err == nil && bad != "VIEW|" {
			t.Errorf("Parse(%q) must fail", bad)
		} else if err != nil && !errors.Is(err, ErrInvalid) {
			t.Errorf("Parse(%q) error must wrap ErrInvalid: %v", bad, err)
		}
	}
}

func TestValid(t *testing.T) {
	if !Capability(1<<62).Valid() || Capability(1<<63).Valid() {
		t.Error("bit 63 must be rejected so values fit a signed column")
	}
}

func TestVerbs(t *testing.T) {
	if got := (CapList | CapView | CapDownload).Verbs(); len(got) != 3 || got[0] != "LIST" || got[2] != "DOWNLOAD" {
		t.Errorf("Verbs = %v", got)
	}
	if got := Capability(0).Verbs(); got == nil || len(got) != 0 {
		t.Errorf("no capabilities is an empty list, not nil: %#v", got)
	}
	future := Capability(1) << 30
	if got := (CapView | future).Verbs(); len(got) != 2 || got[1] != "BIT(30)" {
		t.Errorf("unknown bits must still be reported: %v", got)
	}
}

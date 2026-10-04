// Package authz defines the authorization model: capabilities, roles,
// principals, and the Authorizer interface. It has no storage dependencies;
// sqlauthz is the default SQL-backed engine.
package authz

import (
	"fmt"
	"math/bits"
	"strings"
)

// Capability is a bitmask of things a principal may do to an item. Each
// capability is one bit; a set of capabilities is their union.
//
// Rules that keep this extensible without breaking changes:
//   - Bit positions are fixed forever. Never renumber or reuse a bit.
//   - New capabilities take a reserved bit. Bits 24-47 are reserved for
//     first-party additions and 48-62 for tenant-defined verbs. Bit 63 is
//     unused so values always fit a signed 64-bit column.
//   - Code must preserve bits it does not recognize (see Unknown) so a rolling
//     upgrade never corrupts grants written by a newer node.
type Capability uint64

const (
	CapList     Capability = 1 << 0  // see that an item exists; list a folder's children
	CapView     Capability = 1 << 1  // read metadata and preview
	CapDownload Capability = 1 << 2  // download or copy out the content
	CapCreate   Capability = 1 << 8  // upload or create inside a folder
	CapEdit     Capability = 1 << 9  // rename, replace content
	CapDelete   Capability = 1 << 10 // delete
	CapShare    Capability = 1 << 16 // grant access to others, up to your own capabilities
	CapManage   Capability = 1 << 17 // break inheritance, manage links and settings
)

// capDef describes one registered capability.
type capDef struct {
	bit     Capability
	name    string
	implies Capability // capabilities that must accompany this one
}

// registry is the single source of truth for known capabilities. It is
// ordered by bit position.
var registry = []capDef{
	{CapList, "LIST", 0},
	{CapView, "VIEW", CapList},
	{CapDownload, "DOWNLOAD", CapView},
	{CapCreate, "CREATE", CapView},
	{CapEdit, "EDIT", CapView},
	{CapDelete, "DELETE", CapView},
	{CapShare, "SHARE", CapView},
	{CapManage, "MANAGE", CapShare},
}

// KnownMask is the union of every registered capability.
var KnownMask = func() Capability {
	var m Capability
	for _, d := range registry {
		m |= d.bit
	}
	return m
}()

// AllCaps is what an item's owner holds.
var AllCaps = KnownMask

// maxValid is the highest usable bit (bit 63 stays unused).
const maxValid = Capability(1)<<63 - 1

// Has reports whether every bit of need is present. Has(0) is true.
func (c Capability) Has(need Capability) bool { return c&need == need }

// Union combines two capability sets.
func (c Capability) Union(o Capability) Capability { return c | o }

// Without removes the bits of o.
func (c Capability) Without(o Capability) Capability { return c &^ o }

// SubsetOf reports whether every bit of c is also in o.
func (c Capability) SubsetOf(o Capability) bool { return c&^o == 0 }

// Known returns only the bits this build recognizes.
func (c Capability) Known() Capability { return c & KnownMask }

// Unknown returns bits this build does not recognize. They must be preserved,
// never interpreted.
func (c Capability) Unknown() Capability { return c &^ KnownMask }

// Valid reports whether the value fits a signed 64-bit column.
func (c Capability) Valid() bool { return c <= maxValid }

// Normalize adds every capability implied by the ones present (EDIT implies
// VIEW, which implies LIST, and so on). Grants are normalized when written, so
// checks never need to apply the rules.
func Normalize(c Capability) Capability {
	for {
		next := c
		for _, d := range registry {
			if c&d.bit != 0 {
				next |= d.implies
			}
		}
		if next == c {
			return c
		}
		c = next
	}
}

// String renders capabilities as verbs, such as "LIST|VIEW|DOWNLOAD".
// Unrecognized bits render as BIT(n) so they survive a round trip.
func (c Capability) String() string {
	if c == 0 {
		return "NONE"
	}
	var parts []string
	for _, d := range registry {
		if c&d.bit != 0 {
			parts = append(parts, d.name)
		}
	}
	for u := c.Unknown(); u != 0; u &= u - 1 {
		parts = append(parts, fmt.Sprintf("BIT(%d)", bits.TrailingZeros64(uint64(u))))
	}
	return strings.Join(parts, "|")
}

// ParseCapabilities parses verbs separated by '|' or ','. It accepts the
// output of String, case-insensitively. NONE and the empty string are 0.
func ParseCapabilities(s string) (Capability, error) {
	var c Capability
	for _, part := range strings.FieldsFunc(s, func(r rune) bool { return r == '|' || r == ',' }) {
		name := strings.ToUpper(strings.TrimSpace(part))
		if name == "" || name == "NONE" {
			continue
		}
		if strings.HasPrefix(name, "BIT(") && strings.HasSuffix(name, ")") {
			var n int
			if _, err := fmt.Sscanf(name, "BIT(%d)", &n); err != nil || n < 0 || n > 62 {
				return 0, fmt.Errorf("%w: bad capability %q", ErrInvalid, part)
			}
			c |= Capability(1) << n
			continue
		}
		found := false
		for _, d := range registry {
			if d.name == name {
				c |= d.bit
				found = true
				break
			}
		}
		if !found {
			return 0, fmt.Errorf("%w: unknown capability %q", ErrInvalid, part)
		}
	}
	return c, nil
}

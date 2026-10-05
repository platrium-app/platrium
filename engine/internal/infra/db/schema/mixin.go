package schema

import (
	"fmt"
	"time"

	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/mixin"
	nanoid "github.com/matoous/go-nanoid/v2"
)

// Column sizes shared across the schema. Indexed strings stay bounded so they
// fit MySQL/MariaDB index key limits (utf8mb4, 3072 bytes on InnoDB).
const (
	idLen    = 64
	nameLen  = 255
	emailLen = 255
)

// binaryType is the SchemaType for string columns that compare byte-for-byte
// on every backend. MySQL/MariaDB default to case-insensitive collations and
// Postgres to locale-aware ones, which would make IDs ("aB" vs "Ab") collide
// or sort differently per database, and would break keyset pagination. Use it
// for IDs and for identifiers that are case-sensitive by spec (OIDC subjects).
func binaryType(size int) map[string]string {
	return map[string]string{
		dialect.MySQL:    fmt.Sprintf("varchar(%d) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin", size),
		dialect.Postgres: fmt.Sprintf("varchar(%d) COLLATE ucs_basic", size),
	}
}

// idType is binaryType sized for entity IDs and references to them.
var idType = binaryType(idLen)

func utcNow() time.Time { return time.Now().UTC() }

// timestampType pins sub-second precision on MySQL/MariaDB, where ent would
// otherwise emit second-resolution columns. Keyset and change-feed queries
// need microseconds.
var timestampType = map[string]string{
	dialect.MySQL: "datetime(6)",
}

// IDMixin gives every entity an application-generated string primary key.
// Callers may override it with SetID (e.g. when an ID is minted up front).
type IDMixin struct{ mixin.Schema }

func (IDMixin) Fields() []ent.Field {
	return []ent.Field{
		field.String("id").
			MaxLen(idLen).
			SchemaType(idType).
			NotEmpty().
			Immutable().
			DefaultFunc(func() string { return nanoid.Must() }),
	}
}

// TimeMixin adds created_at/updated_at, set by the application in UTC.
// updated_at is bumped automatically on every ent update.
type TimeMixin struct{ mixin.Schema }

func (TimeMixin) Fields() []ent.Field {
	return []ent.Field{
		field.Time("created_at").
			Immutable().
			Default(utcNow).
			SchemaType(timestampType),
		field.Time("updated_at").
			Default(utcNow).
			UpdateDefault(utcNow).
			SchemaType(timestampType),
	}
}

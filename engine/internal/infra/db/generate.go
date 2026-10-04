package db

// schema/ holds the hand-written entity definitions; ent/ is generated from
// them and must not be edited.
//go:generate go tool ent generate --target ./ent --feature sql/upsert,sql/lock,sql/execquery ./schema

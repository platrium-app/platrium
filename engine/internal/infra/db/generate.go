package db

//go:generate go tool ent generate --feature sql/upsert,sql/lock,sql/execquery ./ent/schema

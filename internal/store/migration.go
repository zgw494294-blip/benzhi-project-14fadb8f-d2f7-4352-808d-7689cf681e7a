package store

import "database/sql"

func migrate(db *sql.DB) error {
	_, e := db.Exec(`CREATE TABLE IF NOT EXISTS schema_meta (version INTEGER NOT NULL);`)
	return e
}

package sqlite

import (
	"database/sql"
	"embed"
	"fmt"
	"sort"
)

//go:embed migrations/*.sql
var migrations embed.FS

// migrate applies every migration not yet recorded, each in its own transaction.
//
// Applied names are recorded rather than a version number: a number tells you how far you got and
// not which files got you there, and the two disagree the first time somebody inserts a migration
// out of order.
func migrate(db *sql.DB) error {
	if _, err := db.Exec(`create table if not exists schema_migrations (name text primary key) strict`); err != nil {
		return fmt.Errorf("sqlite: migration table: %w", err)
	}

	entries, err := migrations.ReadDir("migrations")
	if err != nil {
		return err
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	sort.Strings(names)

	for _, name := range names {
		var applied string
		err := db.QueryRow(`select name from schema_migrations where name = ?`, name).Scan(&applied)
		if err == nil {
			continue
		}
		if err != sql.ErrNoRows {
			return fmt.Errorf("sqlite: reading migration state: %w", err)
		}

		body, err := migrations.ReadFile("migrations/" + name)
		if err != nil {
			return err
		}

		tx, err := db.Begin()
		if err != nil {
			return err
		}
		if _, err := tx.Exec(string(body)); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("sqlite: applying %s: %w", name, err)
		}
		if _, err := tx.Exec(`insert into schema_migrations (name) values (?)`, name); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("sqlite: recording %s: %w", name, err)
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("sqlite: committing %s: %w", name, err)
		}
	}

	return nil
}

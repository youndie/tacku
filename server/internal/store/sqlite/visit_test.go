package sqlite_test

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"

	_ "modernc.org/sqlite"

	"github.com/youndie/tacku/server/internal/domain"
	"github.com/youndie/tacku/server/internal/store/sqlite"
)

// A database written before the visit existed must not have its boundary rewound by the migration
// that introduces it.
//
// The failure this rules out is the nastiest kind a migration has: everybody who had ever pressed
// "mark all as seen" would arrive the next morning to the entire history of the workspace, labelled
// as news — the exact thing the boundary exists to prevent, delivered by the change that was meant
// to improve it. Nothing else in this package can see it, because every other test starts from a
// database that never had the old shape.
func TestAnOlderDatabaseKeepsItsBoundary(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tacku.db")
	stamp := time.Now().UTC().Format(time.RFC3339Nano)

	// The database as it stood at migration 0005: the seen table with one column for the boundary
	// and a member who had marked everything read.
	old := openRaw(t, path)
	applyThrough(t, old, "0005_seen.sql")
	if _, err := old.Exec(`insert into seen (member, cursor, at) values (?, ?, ?)`,
		string(anna), "c7", stamp); err != nil {
		t.Fatal(err)
	}
	if err := old.Close(); err != nil {
		t.Fatal(err)
	}

	// Opening applies what is left, which is the migration under test.
	store, err := sqlite.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })

	visit, err := store.Visit(context.Background(), anna)
	if err != nil {
		t.Fatal(err)
	}
	if visit.Boundary != domain.CursorAt(7) {
		t.Errorf("the boundary came out of the migration as %q, want c7", visit.Boundary)
	}
	if visit.Pending != domain.CursorAt(7) {
		t.Errorf("the next visit would start at %q, want c7 — a default here replays the journal",
			visit.Pending)
	}
}

func openRaw(t *testing.T, path string) *sql.DB {
	t.Helper()

	db, err := sql.Open("sqlite", path+"?_pragma=foreign_keys(ON)")
	if err != nil {
		t.Fatal(err)
	}
	return db
}

// applyThrough replays the migrations up to and including last, and records them the way the real
// runner does, so that opening the store afterwards applies exactly the rest.
func applyThrough(t *testing.T, db *sql.DB, last string) {
	t.Helper()

	if _, err := db.Exec(`create table if not exists schema_migrations (name text primary key) strict`); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir("migrations")
	if err != nil {
		t.Fatal(err)
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	sort.Strings(names)

	found := false
	for _, name := range names {
		body, err := os.ReadFile(filepath.Join("migrations", name))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec(string(body)); err != nil {
			t.Fatalf("applying %s: %v", name, err)
		}
		if _, err := db.Exec(`insert into schema_migrations (name) values (?)`, name); err != nil {
			t.Fatal(err)
		}
		if name == last {
			found = true
			break
		}
	}
	// A name that matched nothing would silently apply every migration, including the one under
	// test, and the check below would pass by having tested nothing.
	if !found {
		t.Fatalf("no migration named %q, so the old shape was never built", last)
	}
}

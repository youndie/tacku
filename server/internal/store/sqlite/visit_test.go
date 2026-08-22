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

// Every arrival that began a visit leaves a row behind.
//
// `seen` overwrites itself by design — it answers where a reader is now — and the threshold B-38
// asks about is chosen from the SHAPE of the gaps, which one value per person cannot show. The
// history is therefore appended, and it is appended at the moment of arrival because nothing can
// reconstruct it afterwards.
func TestEveryVisitThatBeganAfterAGapIsKept(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tacku.db")
	s, err := sqlite.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	ctx := context.Background()

	base := time.Date(2026, 8, 22, 9, 0, 0, 0, time.UTC)
	gaps := []time.Duration{0, 10 * time.Hour, 30 * time.Minute, 26 * time.Hour}
	for i, away := range gaps {
		visit := domain.Visit{
			Boundary: domain.Start,
			Pending:  domain.Start,
			At:       base.Add(time.Duration(i) * time.Hour),
			Away:     away,
		}
		if err := s.RecordVisit(ctx, anna, visit); err != nil {
			t.Fatal(err)
		}
	}

	kept := visitGaps(t, path)

	// Three rather than four: an arrival with no gap is the same visit continuing, and counting it
	// would fill the history with rows that measure nothing.
	if len(kept) != 3 {
		t.Fatalf("kept %v, want the three arrivals that began a visit", kept)
	}
	if kept[0] != 30*time.Minute || kept[2] != 26*time.Hour {
		t.Errorf("the gaps came back as %v, which is not what was recorded", kept)
	}
}

// visitGaps reads the history the way the measurement does, and through the same door: a query
// against the file, because that is what `make measure` has.
func visitGaps(t *testing.T, path string) []time.Duration {
	t.Helper()

	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()

	rows, err := db.Query(`select away from visits order by away`)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = rows.Close() }()

	found := []time.Duration{}
	for rows.Next() {
		var seconds int64
		if err := rows.Scan(&seconds); err != nil {
			t.Fatal(err)
		}
		found = append(found, time.Duration(seconds)*time.Second)
	}
	return found
}

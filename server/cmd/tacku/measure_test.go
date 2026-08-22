package main

import (
	"database/sql"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	_ "modernc.org/sqlite"
)

// A share is printed when there is something to divide, and refused when there is not.
//
// The refusal is the point. Both numbers this command exists for have been waiting on people who do
// not exist yet, and the failure that waiting invites is a percentage computed over four rows and
// quoted afterwards as if it had been measured. This repository has a rule about that — a number
// nobody measured does not get written — and here the rule is the code.
func TestAShareIsRefusedUntilThereIsSomethingToDivide(t *testing.T) {
	for _, one := range []struct {
		moves   int
		refused bool
	}{
		{moves: 4, refused: true},
		{moves: 40, refused: false},
	} {
		path := filepath.Join(t.TempDir(), "tacku.db")
		seedMoves(t, path, one.moves)

		out := &strings.Builder{}
		if err := measure([]string{"-db", path}, out); err != nil {
			t.Fatal(err)
		}

		said := out.String()
		if !strings.Contains(said, "board") {
			t.Fatalf("%d moves: the report says nothing about the surfaces at all: %s", one.moves, said)
		}
		switch {
		case one.refused && !strings.Contains(said, "no share printed"):
			t.Errorf("%d moves: a share was printed over too little: %s", one.moves, said)
		case !one.refused && strings.Contains(said, "no share printed"):
			t.Errorf("%d moves: enough was recorded and nothing was divided: %s", one.moves, said)
		}
	}
}

// seedMoves writes status changes split between the two surfaces, and nothing else: what is being
// tested is the arithmetic and its refusal, not the store.
func seedMoves(t *testing.T, path string, moves int) {
	t.Helper()

	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()

	if _, err := db.Exec(`create table changes (
		seq integer primary key autoincrement, task text, kind text, surface text)`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`create table visits (
		id integer primary key autoincrement, member text, at text, away integer not null)`); err != nil {
		t.Fatal(err)
	}
	for i := range moves {
		surface := "board"
		if i%3 == 0 {
			surface = "task"
		}
		if _, err := db.Exec(`insert into changes (task, kind, surface) values (?, 'status_moved', ?)`,
			fmt.Sprintf("TAC-%d", i+1), surface); err != nil {
			t.Fatal(err)
		}
	}
}

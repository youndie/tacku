package main

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"flag"
	"fmt"
	"os"

	"github.com/youndie/tacku/server/internal/domain"
	"github.com/youndie/tacku/server/internal/store/sqlite"
)

// runAddMember creates one person who can sign in.
//
// It exists because deploying the server made the gap visible: a fresh database has no members, and
// `seed` says in its own comment that it is for a throwaway database and never for a real
// deployment — the thing that would replace it is inviting somebody, and inviting does not exist.
// Without this, a running instance is one nobody can enter, which is a deploy that succeeds and
// delivers nothing.
//
// This is not that invitation. It is the smallest thing that makes a deployment usable: a person
// created from the shell of the machine holding the database, which is a place only somebody who
// already has the cluster can reach. What it is not is written down in the backlog rather than
// implied by the absence.
func runAddMember(args []string) error {
	flags := flag.NewFlagSet("add-member", flag.ContinueOnError)
	path := flags.String("db", "/data/tacku.db", "path to the database file")
	id := flags.String("id", "", "the identifier an agent acts on behalf of")
	email := flags.String("email", "", "the address they sign in with")
	name := flags.String("name", "", "how they are shown to everybody else")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *id == "" || *email == "" || *name == "" {
		return fmt.Errorf("add-member needs -id, -email and -name")
	}

	store, err := sqlite.Open(*path)
	if err != nil {
		return err
	}
	defer store.Close()

	// Generated rather than taken as an argument, and printed once. A password passed on the
	// command line is a password in the shell history of whoever ran it and in the process list of
	// everybody on that machine while it runs.
	password, err := freshPassword()
	if err != nil {
		return err
	}

	if _, err := store.AddMember(context.Background(), domain.MemberID(*id), *email, *name, password); err != nil {
		return err
	}

	fmt.Fprintf(os.Stdout, "%s can now sign in as %s\npassword: %s\n", *name, *email, password)
	fmt.Fprintln(os.Stderr, "tacku: this password is shown once and is not stored anywhere in readable form")
	return nil
}

func freshPassword() (string, error) {
	raw := make([]byte, 18)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}

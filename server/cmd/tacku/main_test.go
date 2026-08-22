package main

import (
	"testing"
	"time"

	"github.com/youndie/tacku/server/internal/wizard"
)

// The lifetime of a scenario is configuration, and this is where the configuration is read.
//
// The httpsrv tests prove the server honours the value it is handed; this proves the value comes
// from the environment. Between the two there is one line — the field in the config literal — and
// it is named here rather than pretended away.
func TestTheScenarioLifetimeIsReadFromTheEnvironment(t *testing.T) {
	t.Setenv("TACKU_WIZARD_TTL", "45m")

	ttl, err := wizardTTL()
	if err != nil {
		t.Fatal(err)
	}
	if ttl != 45*time.Minute {
		t.Errorf("TACKU_WIZARD_TTL=45m was read as %s", ttl)
	}
	if ttl == wizard.DefaultTTL {
		t.Error("the test value equals the default, so it cannot show the variable was read at all")
	}
}

// Unset means the default, and the zero is how that travels: the store substitutes its own constant
// rather than this command repeating the number.
func TestAnUnsetLifetimeLeavesTheDefaultToTheStore(t *testing.T) {
	t.Setenv("TACKU_WIZARD_TTL", "")

	ttl, err := wizardTTL()
	if err != nil {
		t.Fatal(err)
	}
	if ttl != 0 {
		t.Errorf("an unset variable was read as %s; the default belongs in one place", ttl)
	}
}

// A value nobody can read stops the process instead of falling back.
//
// A server quietly running on thirty minutes while its configuration says two hours is wrong in the
// way nobody finds: everything works, and the only symptom is scenarios expiring earlier than the
// person who set the variable expects.
func TestALifetimeThatCannotBeReadRefusesToStart(t *testing.T) {
	for name, value := range map[string]string{
		"not a duration": "half an hour",
		"a bare number":  "30",
		"zero":           "0s",
		"negative":       "-5m",
	} {
		t.Setenv("TACKU_WIZARD_TTL", value)

		if _, err := wizardTTL(); err == nil {
			t.Errorf("%s (%q) was accepted", name, value)
		}
	}
}

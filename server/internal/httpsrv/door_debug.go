//go:build debugdoor

package httpsrv

import (
	"net/http"

	"github.com/youndie/tacku/server/internal/auth"
	"github.com/youndie/tacku/server/internal/domain"
)

// The door the instrument uses: an email and a password exchanged for a session this server signs.
//
// Compiled in only under `debugdoor`, and that is the whole mechanism. It could have been a
// configuration flag, and a flag is a thing somebody sets wrong once: a second way into a product is
// worth having only where nothing is at stake, and "nothing is at stake" is not a property a
// deployment can be trusted to have declared correctly. Absent from the image means absent.
//
// It exists because the desktop client is an instrument rather than a surface. A person uses the
// page and goes through the provider; a developer points the window at a stand and needs a password
// there, where the provider may not even be running.
// DoorPresent is exported so that a check can ask what this build serves rather than assume.
const DoorPresent = true

func sessionDoor(mux *http.ServeMux, members domain.Members, sessions *auth.Sessions) func(string) (auth.Principal, error) {
	mux.Handle("GET /forms/sign-in", loginForm())
	mux.Handle("POST "+LoginPath, submitLogin(members, sessions))

	return sessions.Verify
}

//go:build !debugdoor

package httpsrv

import (
	"net/http"

	"github.com/youndie/tacku/server/internal/auth"
	"github.com/youndie/tacku/server/internal/domain"
)

// No second door.
//
// This is what a shipped server is: one way in, through the identity provider, for people and for
// agents alike. The form and the session it hands out are not switched off here — they are not
// present, which is a stronger statement and the one worth making. See `door_debug.go` for why it is
// a build tag rather than a setting.
// DoorPresent is exported so that a check can ask what this build serves rather than assume.
const DoorPresent = false

func sessionDoor(*http.ServeMux, domain.Members, *auth.Sessions) func(string) (auth.Principal, error) {
	return nil
}

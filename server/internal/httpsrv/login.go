package httpsrv

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/youndie/tacku/server/internal/auth"
	"github.com/youndie/tacku/server/internal/domain"
	"github.com/youndie/tacku/server/internal/forms"
	"github.com/youndie/tacku/server/internal/render"
)

const loginFormID = "sign-in"

// LoginPath is where a session begins, and the one route of the KOMPOT surface that is public.
const LoginPath = "/submit/sign-in"

// loginForm is a form like any other, and public.
//
// There is no centring in the vocabulary, so the card sits between spacers — which is not a trick to
// work around a missing feature but the mechanism the vocabulary offers.
func loginForm() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		form := forms.New(loginFormID)

		email := form.TextInput("email", "Email", "you@company.com",
			[]forms.Rule{forms.Required("Enter the email you signed up with.")},
			forms.Keyboard("EMAIL"))

		password := form.TextInput("password", "Password", "",
			[]forms.Rule{forms.Required("Enter your password.")},
			forms.Secret())

		card := render.Column("sign-in-card", 24,
			[]render.Modifier{render.WidthDp(420)},
			render.Column("sign-in-heading", 4, nil,
				render.Text("sign-in-brand", "tacku", render.TextDisplay),
				render.Text("sign-in-title", "Sign in to your workspace", render.TextBodyMuted),
			),
			email,
			password,
			render.Row("sign-in-actions", 0, nil,
				render.PrimaryButton("sign-in-submit", "Sign in", render.SubmitForm(form.FormID()),
					render.PaddingXY(14, 24)),
				render.Spacer("sign-in-actions-spacer"),
			),
			render.Text("sign-in-note",
				"Your agent signs in with its own credentials. Every action it takes is recorded on your behalf.",
				render.TextMeta),
		)

		// A row at the root, and that is not decoration.
		//
		// The client draws a screen so that it scrolls when it is taller than the window, and the
		// mechanism lays the ROOT's children out as separate items — which is why a `weight` among
		// them has nothing to divide and collapses to nothing. Centring is two weighted spacers, so
		// on this screen, and only on this screen, the form ended up jammed against the top edge
		// with its heading clipped. Every other screen has a row at the root already and never saw
		// it. Wrapping the column in one puts the whole thing inside a single item, where weight
		// means what it means everywhere else.
		screen := render.Row("screen-sign-in", 0,
			[]render.Modifier{render.FillWidth(), render.FillHeight(), render.Background(render.ColorSurface)},
			render.Column("sign-in-body", 0,
				[]render.Modifier{render.Weight(1), render.FillHeight()},
				render.Spacer("sign-in-top"),
				render.Row("sign-in-row", 0, nil,
					render.Spacer("sign-in-left"),
					card,
					render.Spacer("sign-in-right"),
				),
				render.Spacer("sign-in-bottom"),
			),
		)

		respond(w, r, form.Build(screen))
	}
}

// submitLogin answers `update_session`, which is the single point where this protocol touches
// authorisation (§12.4): the client replaces its pair and carries the access half as a bearer.
//
// No idempotency key. §16.5 asks for one from a submit that changes state, and signing in changes
// none: repeating it issues another pair, which is what a repeat of a sign-in should do. Demanding a
// key here would also mean a client that lost its answer could never simply ask again.
func submitLogin(members domain.Members, sessions *auth.Sessions) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var request submitRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			http.Error(w, `{"error":"the body is not a form submission"}`, http.StatusBadRequest)
			return
		}

		member, err := members.Authenticate(r.Context(), request.text("email"), request.text("password"))
		if err != nil {
			if errors.Is(err, domain.ErrBadCredentials) {
				// 401 on a public endpoint, which is not a contradiction: §16.8 says the absence of
				// authorisation and the code 401 are independent, and a sign-in form answering a
				// wrong pair is the example it gives.
				writeJSON(w, http.StatusUnauthorized, map[string]string{
					"error":   "That password does not match this email.",
					"fieldId": "password",
				})
				return
			}
			fail(w, err)
			return
		}

		pair, err := sessions.Issue(member)
		if err != nil {
			fail(w, err)
			return
		}

		writeJSON(w, http.StatusOK, map[string]any{
			"type":         "update_session",
			"accessToken":  pair.Access,
			"refreshToken": pair.Refresh,
		})
	}
}

// requireSession guards the KOMPOT surface.
//
// Deliberately not the OAuth middleware. An MCP token is bound by audience to the MCP endpoint, and
// accepting one here would spend a credential issued for something else — the confused deputy the
// audience rule exists to prevent, arriving through our own front door.
func requireSession(sessions *auth.Sessions, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		header := r.Header.Get("Authorization")
		const prefix = "Bearer "
		if len(header) <= len(prefix) || header[:len(prefix)] != prefix {
			challenge(w)
			return
		}

		principal, err := sessions.Verify(header[len(prefix):])
		if err != nil {
			challenge(w)
			return
		}

		next.ServeHTTP(w, r.WithContext(auth.WithPrincipal(r.Context(), principal)))
	})
}

func challenge(w http.ResponseWriter) {
	// Bearer with no metadata pointer: an anonymous caller is told where it is, not where to go —
	// the page knows its provider already, from /auth/config.
	w.Header().Set("WWW-Authenticate", `Bearer realm="tacku"`)
	unauthenticated(w)
}

// refuseWithReason is challenge for a caller that presented something, and says what was wrong.
//
// Everything else about a refused request is identical whether the token expired, names another
// audience, or was signed by a provider this server does not know — three different mistakes with
// three different fixes, and one of them cost a day of guessing. RFC 6750 §3.1 is where the shape
// comes from: `error` for a program, `error_description` for whoever is reading a network tab.
func refuseWithReason(w http.ResponseWriter, refusal error) {
	if refusal == nil {
		challenge(w)
		return
	}

	// Quotes and backslashes would end the header value early, and a truncated header is a worse
	// answer than a plain one.
	description := strings.NewReplacer(`"`, "'", `\`, "/", "\n", " ").Replace(refusal.Error())
	if len(description) > 200 {
		description = description[:200]
	}

	w.Header().Set(
		"WWW-Authenticate",
		fmt.Sprintf(`Bearer realm="tacku", error="invalid_token", error_description="%s"`, description),
	)

	// And in the body, because that is where a person looks. The header is where the specification
	// puts it and where a client library reads it; a developer with a failing request open in front
	// of them sees `{"error":"unauthenticated"}` and has no reason to suspect a header carries the
	// answer. Asking somebody to know that is asking them to know what I know.
	writeJSON(w, http.StatusUnauthorized, map[string]string{
		"error":  "unauthenticated",
		"reason": description,
	})
}

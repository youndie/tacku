package httpsrv

import (
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/youndie/tacku/server/internal/render"
)

// page serves the browser client: the product's human surface.
//
// Off unless a directory is named. A server with no page is the state this product was in for its
// whole life and it is a legitimate one — the conformance walk, the agents and the desktop
// instrument all work without it — so the absence is a configuration rather than a failure.
//
// The files come from a directory rather than from the binary. Embedding would put thirteen
// megabytes of WebAssembly into the repository and into every image that does not serve a page, and
// would tie a rebuild of the server to a rebuild of the client for no reason: the two halves are
// already versioned by the wire between them.
func page(dir string) (http.Handler, error) {
	if dir == "" {
		return nil, nil
	}

	// Checked at startup rather than at the first request. A misspelled path is a mistake somebody
	// makes while deploying, and the moment to say so is while they are watching.
	index := filepath.Join(dir, "index.html")
	if _, err := os.Stat(index); err != nil {
		return nil, err
	}

	files := http.FileServer(http.Dir(dir))

	// The addresses the page itself can be standing at. Until it kept its position in the browser
	// there were none — a deeplink was `app://…` and never a path — and the comment here said so.
	// Now a person can reload one, or send one to somebody, and the server has to answer with the
	// page rather than with the 404 that a path with no file behind it earns.
	//
	// Named rather than "anything that is not a file": a typo is still a mistake and still says so,
	// which was the point of the old rule and is worth keeping. The names come from the graph, so
	// there is no second list to drift.
	exact, prefixes := render.ClientPaths()
	known := make(map[string]bool, len(exact))
	for _, path := range exact {
		known[path] = true
	}

	standing := func(path string) bool {
		// A last segment with an extension is a file, whatever it looks like otherwise. Without
		// this, a page served at /task/TAC-2 that asks for a relative `tacku.js` is handed this very
		// page — the browser parses HTML as JavaScript and the application never starts, which
		// arrives as a blank window and no error anybody can act on. It happened exactly that way.
		if strings.Contains(path[strings.LastIndex(path, "/")+1:], ".") {
			return false
		}
		if known[path] {
			return true
		}
		for _, prefix := range prefixes {
			if strings.HasPrefix(path, prefix) && len(path) > len(prefix) {
				return true
			}
		}
		return false
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// The API is mounted on its own prefixes and this handler never sees those; what does reach
		// here is the page, its bundle, its fonts, the addresses the page keeps, and typos.
		if standing(r.URL.Path) {
			http.ServeFile(w, r, index)
			return
		}
		if strings.HasSuffix(r.URL.Path, "/") || r.URL.Path == "" {
			if !precompressed(w, r, dir, "index.html") {
				http.ServeFile(w, r, index)
			}
			return
		}
		if precompressed(w, r, dir, strings.TrimPrefix(path.Clean(r.URL.Path), "/")) {
			return
		}
		files.ServeHTTP(w, r)
	}), nil
}

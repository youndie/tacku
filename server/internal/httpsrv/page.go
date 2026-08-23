package httpsrv

import (
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
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

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Everything the page asks for is a file it was shipped with. There are no client-side
		// routes to fall back for: a deeplink in this product is `app://…` and never a path, so a
		// request for a path that does not exist is a mistake and answered as one.
		//
		// The API is mounted on its own prefixes and this handler never sees those; what does reach
		// here is the page, its bundle, its fonts, and typos.
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

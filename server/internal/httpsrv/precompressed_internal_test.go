package httpsrv

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

// What a caller gets for a file that was compressed before the build ended.
//
// The interesting cases are all refusals: an answer compressed for somebody who did not ask for it
// is unreadable, and an answer whose `Content-Type` describes the compression rather than the file
// is a WebAssembly module a browser will not instantiate. Both look like a working server from the
// outside — a 200 with a body — which is why each is asserted rather than assumed.
func TestPrecompressed(t *testing.T) {
	dir := t.TempDir()
	write := func(name, body string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write("app.wasm", "the original")
	write("app.wasm.gz", "gzipped")
	write("app.wasm.br", "brotlied")
	write("alone.wasm", "never compressed")

	ask := func(t *testing.T, name, accept string, headers ...[2]string) *http.Response {
		t.Helper()
		request := httptest.NewRequest(http.MethodGet, "/"+name, nil)
		if accept != "" {
			request.Header.Set("Accept-Encoding", accept)
		}
		for _, header := range headers {
			request.Header.Set(header[0], header[1])
		}
		recorder := httptest.NewRecorder()
		if !precompressed(recorder, request, dir, name) {
			recorder.WriteHeader(http.StatusTeapot) // "did nothing", distinguishable from any answer
		}
		return recorder.Result()
	}

	t.Run("brotli wins when both are offered", func(t *testing.T) {
		answer := ask(t, "app.wasm", "gzip, br")

		if got := answer.Header.Get("Content-Encoding"); got != "br" {
			t.Fatalf("encoding is %q, and brotli is the smaller of the two", got)
		}
		// The type of what was compressed, not of the compression: a browser told `application/gzip`
		// refuses to instantiate the module.
		if got := answer.Header.Get("Content-Type"); got != "application/wasm" {
			t.Fatalf("content type is %q", got)
		}
		if got := answer.Header.Get("Vary"); got != "Accept-Encoding" {
			t.Fatalf("Vary is %q: a shared cache would hand this body to a client that asked for none", got)
		}
		// Without it the answer goes out chunked and a browser downloads megabytes with no idea
		// how many are left.
		if got := answer.Header.Get("Content-Length"); got != "8" {
			t.Fatalf("content length is %q, and the compressed file is 8 bytes", got)
		}
	})

	t.Run("gzip when that is all the caller takes", func(t *testing.T) {
		if got := ask(t, "app.wasm", "gzip").Header.Get("Content-Encoding"); got != "gzip" {
			t.Fatalf("encoding is %q", got)
		}
	})

	t.Run("nothing for a caller that asked for nothing", func(t *testing.T) {
		if code := ask(t, "app.wasm", "").StatusCode; code != http.StatusTeapot {
			t.Fatalf("something was served (%d) to a caller that named no encoding", code)
		}
	})

	t.Run("refusing an encoding is not the same as accepting it", func(t *testing.T) {
		if code := ask(t, "app.wasm", "gzip;q=0").StatusCode; code != http.StatusTeapot {
			t.Fatalf("gzip was served (%d) to a caller that said q=0 — that is a refusal", code)
		}
	})

	t.Run("a file with no compressed sibling is left alone", func(t *testing.T) {
		if code := ask(t, "alone.wasm", "gzip, br").StatusCode; code != http.StatusTeapot {
			t.Fatalf("something was served (%d) for a file that was never compressed", code)
		}
	})

	t.Run("a range request is left alone", func(t *testing.T) {
		// A range is a range over the bytes the caller expects. A slice of a differently encoded
		// file is a corrupt answer wearing a 206.
		answer := ask(t, "app.wasm", "gzip, br", [2]string{"Range", "bytes=0-3"})
		if answer.StatusCode != http.StatusTeapot {
			t.Fatalf("a compressed body was served (%d) for a range request", answer.StatusCode)
		}
	})
}

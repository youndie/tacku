package httpsrv

import (
	"mime"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
)

// The encodings this server can hand out, best first.
//
// Both are produced at build time and neither is produced here. Compressing on the way out would
// spend a core per request on a file that never changes, and would spend it again for every reader —
// while an offline compressor can afford its slowest setting once. Measured on this product's own
// bundle: 8.65 MB of WebAssembly becomes 3.32 MB gzipped and 2.61 MB with brotli.
var encodings = []struct {
	name   string
	suffix string
}{
	{name: "br", suffix: ".br"},
	{name: "gzip", suffix: ".gz"},
}

// precompressed serves a file that was compressed before it ever left the build, or nothing.
//
// It answers only when three things hold: the caller said it accepts the encoding, the compressed
// sibling exists, and the request is not a range request. The last one is not fussiness — a range
// is a range over the bytes the caller expects, and handing back a slice of a differently encoded
// file is a corrupt answer that looks like a working one.
//
// Returns false when it did nothing, and the caller then serves the file as it always did. An image
// built without the compression step therefore works, slowly and correctly, rather than serving
// nothing.
func precompressed(w http.ResponseWriter, r *http.Request, dir, name string) bool {
	if r.Header.Get("Range") != "" {
		return false
	}

	accepted := r.Header.Get("Accept-Encoding")
	for _, encoding := range encodings {
		if !accepts(accepted, encoding.name) {
			continue
		}

		file := filepath.Join(dir, filepath.FromSlash(name)+encoding.suffix)
		info, err := os.Stat(file)
		if err != nil || info.IsDir() {
			continue
		}

		body, err := os.Open(file)
		if err != nil {
			continue
		}
		defer func() { _ = body.Close() }()

		// The type of what was compressed, not of the compressed file: `application/wasm` gzipped
		// is still `application/wasm`, and a browser that is told `application/gzip` will not
		// instantiate it. This is the mistake the whole feature is one line away from.
		if kind := mime.TypeByExtension(path.Ext(name)); kind != "" {
			w.Header().Set("Content-Type", kind)
		}
		w.Header().Set("Content-Encoding", encoding.name)
		// Without it a shared cache hands a compressed body to a client that asked for none.
		w.Header().Add("Vary", "Accept-Encoding")
		// Named, because ServeContent declines to compute it once an encoding is set and answers
		// chunked instead — a browser then downloads several megabytes with no idea how many are
		// left. Exact here: a range is refused above, so this is the whole body.
		w.Header().Set("Content-Length", strconv.FormatInt(info.Size(), 10))
		// The uncompressed file's tag would be wrong for these bytes, and ServeContent computes
		// none of its own; leaving it absent is honest.
		http.ServeContent(w, r, "", info.ModTime(), body)
		return true
	}

	return false
}

// accepts says whether a header names an encoding without refusing it.
//
// Deliberately small, and deliberately not a quality-value parser: what it must not do is match
// `gzip;q=0` — a caller saying "anything but gzip" — and that is the one case a substring search
// gets wrong. Everything finer than that is a preference, and the preference this server follows is
// its own, best first.
func accepts(header, encoding string) bool {
	for part := range strings.SplitSeq(header, ",") {
		fields := strings.Split(strings.TrimSpace(part), ";")
		if !strings.EqualFold(strings.TrimSpace(fields[0]), encoding) {
			continue
		}
		for _, parameter := range fields[1:] {
			parameter = strings.ReplaceAll(strings.TrimSpace(parameter), " ", "")
			if parameter == "q=0" || strings.HasPrefix(parameter, "q=0.0") {
				return false
			}
		}
		return true
	}
	return false
}

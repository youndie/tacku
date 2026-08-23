#!/usr/bin/env bash
# Compress the page's assets once, at build time, beside the originals.
#
# The server hands out whichever of the three the caller accepts and never compresses anything
# itself: doing it per request spends a core on a file that never changes, and spends it again for
# every reader, while a compressor working offline can afford its slowest setting once. Measured on
# this product's own bundle: 8.65 MB of WebAssembly becomes 3.32 MB gzipped and 2.61 MB with brotli.
#
# Both encodings, not one. Brotli is a fifth smaller on the bundle and gzip is what everything
# understands — and the originals stay, because the server falls back to them for a caller that
# takes neither.
set -euo pipefail

dir="${1:?usage: precompress.sh <directory>}"
[ -d "$dir" ] || { echo "precompress: no such directory: $dir" >&2; exit 1; }

# What is worth compressing. The rest of what a page carries — png, woff2, anything already
# compressed — comes out bigger, and a "compressed" file larger than its original is a slower page
# with more moving parts.
#
# Named one extension at a time rather than as one regular expression: `find -regex` means different
# things on the two systems this runs on, and the version that worked on the runner silently matched
# nothing on a mac.
compressible=(-name '*.wasm' -o -name '*.js' -o -name '*.mjs' -o -name '*.html'
              -o -name '*.css' -o -name '*.json' -o -name '*.svg' -o -name '*.ttf'
              -o -name '*.txt' -o -name '*.map')

if ! command -v brotli >/dev/null 2>&1; then
    echo "precompress: brotli is not installed — install it or the page ships without it" >&2
    exit 1
fi

count=0
while IFS= read -r file; do
    gzip -9 -k -f -- "$file"
    brotli -q 11 -f -- "$file"
    count=$((count + 1))
done < <(find "$dir" -type f \( "${compressible[@]}" \))

# A run that compressed nothing is a run that found nothing, and it must not read as success: the
# directory would be a build that produced no page, or a filter that no longer matches its files.
if [ "$count" -eq 0 ]; then
    echo "precompress: nothing matched in $dir — the page is empty or the filter is wrong" >&2
    exit 1
fi

# Printed rather than counted silently, because the number is the only thing that distinguishes
# "compressed everything" from "compressed the one file it could still find".
echo "precompress: $count files, each as .gz and .br"

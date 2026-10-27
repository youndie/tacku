# ---------- Stage 1 : build ----------
# Pinned to amd64 on purpose: the image is built on whatever runner is free, and a binary linked
# for the builder's architecture would fail at exec time with nothing but "exec format error" to
# explain it.
FROM --platform=linux/amd64 golang:1.27-bookworm AS build

WORKDIR /src
COPY server/go.mod server/go.sum ./
RUN go mod download

COPY server/ ./

# CGO off, and this is the whole reason the store is modernc.org/sqlite rather than the usual
# binding. It was checked by a probe before the choice was made (probes/sqlite-nocgo), not after:
# a pure-Go SQLite means the binary carries its database engine and needs nothing from the image
# it lands in.
#
# Stripped and trimmed: the paths of the machine that built it are not information a deployed
# binary should carry, and the symbol table is not read by anything here.
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags "-s -w" -o /out/tacku ./cmd/tacku

# ---------- Stage 2 : runtime ----------
# distroless/static, which is what a CGO-free binary can live in: no libc, no shell, no package
# manager. `nonroot` rather than root because nothing here needs to be root and the volume is
# mounted for that user.
#
# The certificates matter and their absence is quiet: without them every outbound HTTPS call fails
# as an unexplained refusal, and this server makes one — fetching the JWKS of the issuer that signs
# agent tokens. distroless/static carries them, which is why it is this image rather than `scratch`.
FROM gcr.io/distroless/static-debian12:nonroot
WORKDIR /app

COPY --from=build /out/tacku /app/tacku

# The page, built before this ran rather than inside it.
#
# It is a Kotlin/Wasm bundle, and building it here would mean a JDK, a Gradle distribution and a
# Node toolchain downloaded on every image build — where the workflow that builds this already keeps
# a warm Gradle cache from the check job. So the bundle is produced beside the context and copied.
#
# The cost of that choice is that `docker build` without a prior Gradle build fails on this line.
# That is the intended half: the alternative — an optional copy — would produce an image that starts,
# serves the API and has no product in it. The server refuses to start when this directory carries no
# index.html, so a bundle that is empty rather than absent is caught too, at startup, while somebody
# is watching the rollout.
COPY client/web/build/dist/wasmJs/productionExecutable /app/web

# The database lives here, on a volume. Named rather than left to the working directory so that the
# mount point is visible in the image and a chart cannot quietly disagree with it.
VOLUME ["/data"]
EXPOSE 8080

ENTRYPOINT ["/app/tacku"]
CMD ["serve", "-db", "/data/tacku.db", "-addr", ":8080", "-web", "/app/web"]

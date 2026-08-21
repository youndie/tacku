# One gate, and CI runs exactly this target.
#
# A local check set that differs from the CI one turns "green here, red there" into the normal
# state of affairs, and then neither is read. So: whatever is not in `make check` is not a gate,
# and whatever is in it runs the same way in both places.
#
# REPOS points at the directory holding this checkout: code_anchors.py resolves a path by looking
# for it in every repository under it. Anchors into the kompot checkout live elsewhere and are
# expected not to resolve — that report is advisory, which is why it is not in the gate.
#
# The gate covers three things now: the documentation tree, the Kotlin half (which generates the
# spec and guards it against drift) and the Go half (which consumes it). They are one target on
# purpose — a half nobody calls rots unseen. When they arrive, their checks join `gate`
# rather than getting a target of their own — a half nobody calls rots unseen.

DOCS ?= docs
BACKLOG ?= backlog.md
REPOS ?= ..
PY ?= python3

.PHONY: check gate docs server client spec tck probe shots format report probes fix help

help:
	@echo "make check   - the gate: blocking checks, exactly what CI runs"
	@echo "make spec    - regenerate the committed KOMPOT spec of this build"
	@echo "make format  - apply ktlint and gofmt in place"
	@echo "make tck     - start the server and walk it with the conformance kit"
	@echo "make probe   - start the server and decode every screen with the toolkit's parsers"
	@echo "make shots   - re-record the screenshot goldens; review the diff as carefully as code"
	@echo "make probes  - re-run the research probes; a probe that stops building is a changed fact"
	@echo "make report  - non-blocking reports: BDD coverage, code anchors"
	@echo "make fix     - regenerate the backlog index, fill in missing coverage-map lines"

check: gate report

# Blocking. Any of these failing means the documentation is internally inconsistent, which is a
# defect in the documentation rather than a matter of opinion.
gate: docs client server

docs:
	$(PY) scripts/backlog_index.py --check --docs $(DOCS) --backlog $(BACKLOG)
	$(PY) scripts/docs_check.py --docs $(DOCS) --backlog $(BACKLOG)
	$(PY) scripts/coverage_map.py --check --docs $(DOCS)

# Guards the committed spec against the generator: a kompot upgrade that changes the wire must not
# leave the Go server validating against a contract that no longer exists.
# ktlintCheck alongside the test for the same reason gofmt sits next to go test on the other half:
# a formatter enforced on one language of a two-language repository is a rule that gets argued about
# in the other.
#
# Pixel comparison runs only where the goldens were recorded, which is why viddikVerify is absent
# under CI. The harness claims its goldens are portable across operating systems; recorded on macOS
# they failed on Linux, and the claim was one this project said out loud it would test by running
# rather than by reading. Deciding to compare pixels on one machine is the answer the backlog item
# named in advance: not "configure it", but pick where the goldens live.
#
# What still runs everywhere is the half that catches the real traps — that the harness ran at all,
# and that it drew something. Neither depends on which machine drew it.
VIDDIK_VERIFY := $(if $(CI),,:app:viddikVerify)

client:
	cd client && ./gradlew --quiet ktlintCheck :spec-gen:test :tck:test :app:test $(VIDDIK_VERIFY)

server:
	cd server && gofmt -l . | tee /dev/stderr | (! read)
	cd server && go vet ./...
	@# -count=1 rather than the cache. The spec tests read files outside their package, and Go's
	@# test cache keys on package inputs: a regenerated schema leaves them green without rerunning.
	cd server && go test -count=1 ./...

# Not in the gate, because it needs a listening server rather than a working tree — and because a
# red conformance run is a finding about the server, which somebody reads, rather than a broken
# build. It starts a throwaway server, walks it and stops it whatever happens.
#
# devauth mints one token and serves the key set behind it. A walk with no token proves almost
# nothing — every endpoint answers 401 and each check reports that same fact in its own words — so
# the fixture exists to make the findings be about the server.
tck:
	@# Killed by port rather than by name. `go run` executes a binary it built in a temporary
	@# directory, so pkill on the package path matches the parent and leaves the server listening —
	@# and a previous devauth still holding the port serves the key set of a key that no longer
	@# signs anything, which arrives as an unexplained 401 on every request.
	@lsof -ti:8477 -ti:8478 2>/dev/null | xargs kill -9 2>/dev/null || true
	@rm -f /tmp/tacku-tck.db /tmp/tacku-tck.token
	@# Seeded, because several checks reach their interesting paths only once an operation can
	@# succeed: against an empty workspace the idempotency check watched a create fail for want of
	@# a board, and a failed attempt is not recorded, so the conflict it wanted could never happen.
	@cd server && go run ./cmd/tacku seed -db /tmp/tacku-tck.db
	@cd server && go run ./cmd/devauth -addr :8478 > /tmp/tacku-tck.token 2>/dev/null & sleep 4
	@cd server && TACKU_RESOURCE=http://localhost:8477 \
		TACKU_ISSUER=http://localhost:8478 TACKU_JWKS_URL=http://localhost:8478/jwks \
		TACKU_SESSION_KEY=a-key-of-at-least-thirty-two-characters \
		go run ./cmd/tacku serve -db /tmp/tacku-tck.db -addr :8477 >/dev/null 2>&1 & sleep 5
	@cd client && ./gradlew --quiet :tck:tck -Ptarget=http://localhost:8477 --console=plain; \
		status=$$?; \
		lsof -ti:8477 -ti:8478 2>/dev/null | xargs kill -9 2>/dev/null || true; \
		rm -f /tmp/tacku-tck.token; exit $$status

shots:
	cd client && ./gradlew --quiet :app:viddikRecord

# The client as a measuring instrument: a response can satisfy the schema and still not decode, and
# only the code that will actually draw the screen can say so.
probe:
	@lsof -ti:8477 -ti:8478 2>/dev/null | xargs kill -9 2>/dev/null || true
	@rm -f /tmp/tacku-probe.db
	@cd server && go run ./cmd/tacku seed -db /tmp/tacku-probe.db
	@cd server && go run ./cmd/devauth -addr :8478 >/dev/null 2>&1 & sleep 4
	@cd server && TACKU_RESOURCE=http://localhost:8477 \
		TACKU_ISSUER=http://localhost:8478 TACKU_JWKS_URL=http://localhost:8478/jwks \
		TACKU_SESSION_KEY=a-key-of-at-least-thirty-two-characters \
		go run ./cmd/tacku serve -db /tmp/tacku-probe.db -addr :8477 >/dev/null 2>&1 & sleep 5
	@cd client && ./gradlew --quiet :app:probe --console=plain; \
		status=$$?; \
		lsof -ti:8477 -ti:8478 2>/dev/null | xargs kill -9 2>/dev/null || true; \
		exit $$status

# Not in the gate: it rewrites committed files. Run it when the wire types change, then review the
# diff of spec/ as carefully as the code.
spec:
	cd client && TACKU_SPEC_RECORD=true ./gradlew --quiet :spec-gen:test

# Deliberately outside the gate. A probe pins the versions a fact was verified against, so it goes
# red when the dependency moves — which is information about the fact, not a broken build. Read the
# failure, amend the research, then re-pin.
probes:
	cd probes/mcp-mux && CGO_ENABLED=0 go run .
	cd probes/sqlite-nocgo && CGO_ENABLED=0 go run .

# Non-blocking, on purpose. Demanding a percentage of automated scenarios is meaningless while
# acceptance is manual, and an anchor goes stale because of a refactor in somebody else's
# repository rather than because of an edit here.
report:
	$(PY) scripts/bdd_report.py --docs $(DOCS) --repos $(REPOS)
	$(PY) scripts/code_anchors.py --docs $(DOCS) --repos $(REPOS)

format:
	cd client && ./gradlew --quiet :spec-gen:ktlintFormat
	cd server && gofmt -w .

fix:
	$(PY) scripts/backlog_index.py --docs $(DOCS) --backlog $(BACKLOG)
	$(PY) scripts/coverage_map.py --fix --docs $(DOCS)

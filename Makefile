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

.PHONY: measure check gate docs server client spec tck probe shots format report probes fix help

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
	$(PY) scripts/questions_check.py
	$(PY) scripts/no_private_names.py
	$(PY) scripts/no_reflective_decode.py
	$(PY) scripts/no_fontless_text_style.py
	$(PY) scripts/reports_check.py

# Guards the committed spec against the generator: a kompot upgrade that changes the wire must not
# leave the Go server validating against a contract that no longer exists.
# ktlintCheck alongside the test for the same reason gofmt sits next to go test on the other half:
# a formatter enforced on one language of a two-language repository is a rule that gets argued about
# in the other.
#
# viddikVerify compares pixels. The goldens are portable because the harness draws in a font it
# carries rather than one the machine happens to have — see the note in Screenshots.kt, which is
# where the portability is actually earned.
client:
	cd client && ./gradlew --quiet ktlintCheck :spec-gen:test :tck:test :app:test :app:viddikVerify

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
	@# Built with the instrument's door, which is what a stand is: a release build serves no sign-in
	@# form, and this walk signs in through one. Without the tag the server starts and answers
	@# `unauthenticated` to every screen — a stand that is up and useless.
	@cd server && TACKU_RESOURCE=http://localhost:8477 \
		TACKU_ISSUER=http://localhost:8478 TACKU_JWKS_URL=http://localhost:8478/jwks \
		TACKU_SESSION_KEY=a-key-of-at-least-thirty-two-characters \
		go run -tags debugdoor ./cmd/tacku serve -db /tmp/tacku-tck.db -addr :8477 >/dev/null 2>&1 & sleep 5
	@cd client && ./gradlew --quiet :tck:tck -Ptarget=http://localhost:8477 --console=plain; \
		status=$$?; \
		lsof -ti:8477 -ti:8478 2>/dev/null | xargs kill -9 2>/dev/null || true; \
		rm -f /tmp/tacku-tck.token; exit $$status

# Rewrites the goldens. The gate compares them; only this rewrites them.
# Both numbers the backlog is waiting on, from a real database. Not in the gate: it answers a
# question about people, and there are none yet — what it does today is refuse to divide, which is
# the behaviour worth having ready before there is data rather than after.
measure:
	cd server && go run ./cmd/tacku measure -db $(DB)

DB ?= tacku.db

shots:
	cd client && ./gradlew --quiet :app:viddikRecord

# Сверка с макетом: значения токенов числами, словарь и токены — структурой. Вне гейта, потому что
# требует экспортированного макета, а не рабочего дерева, и потому что расхождение здесь — находка,
# которую читают, а не поломка сборки.
#
#   make design DESIGN=~/Downloads/tacku\ Design\ Spec.dc.html
design:
	python3 scripts/design_check.py --design "$(DESIGN)"

# The bodies those shots are taken from, from a seeded server. Deliberate rather than automatic:
# see the header of the script.
screens:
	./scripts/screens.sh

# Собрать страницу и поднять сервер, который её отдаёт. Продакшн-поверхность продукта, в отличие от
# десктопного клиента — тот прибор.
web:
	cd client && ./gradlew --quiet :web:wasmJsBrowserDistribution
	@echo "готово: client/web/build/dist/wasmJs/productionExecutable"
	@echo "отдать её:  go run ./cmd/tacku serve -web ../client/web/build/dist/wasmJs/productionExecutable" 

# The client as a measuring instrument: a response can satisfy the schema and still not decode, and
# only the code that will actually draw the screen can say so.
probe:
	@lsof -ti:8477 -ti:8478 2>/dev/null | xargs kill -9 2>/dev/null || true
	@rm -f /tmp/tacku-probe.db
	@cd server && go run ./cmd/tacku seed -db /tmp/tacku-probe.db
	@cd server && go run ./cmd/devauth -addr :8478 >/dev/null 2>&1 & sleep 4
	@# Built with the instrument's door, which is what a stand is: a release build serves no sign-in
	@# form, and this walk signs in through one. Without the tag the server starts and answers
	@# `unauthenticated` to every screen — a stand that is up and useless.
	@cd server && TACKU_RESOURCE=http://localhost:8477 \
		TACKU_ISSUER=http://localhost:8478 TACKU_JWKS_URL=http://localhost:8478/jwks \
		TACKU_SESSION_KEY=a-key-of-at-least-thirty-two-characters \
		go run -tags debugdoor ./cmd/tacku serve -db /tmp/tacku-probe.db -addr :8477 >/dev/null 2>&1 & sleep 5
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
	cd probes/mcp-elicitation && CGO_ENABLED=0 go run .

# Non-blocking, on purpose. Demanding a percentage of automated scenarios is meaningless while
# acceptance is manual, and an anchor goes stale because of a refactor in somebody else's
# repository rather than because of an edit here.
report:
	$(PY) scripts/bdd_report.py --docs $(DOCS) --repos $(REPOS)
	$(PY) scripts/code_anchors.py --docs $(DOCS) --repos $(REPOS)

# Every module the gate checks, and that list is the point: `make check` runs `ktlintCheck` across
# the client, while this used to format one module of it. The other two were formatted by hand or
# not at all, and the difference showed up as a red gate after a change that had just been
# "formatted".
format:
	cd client && ./gradlew --quiet ktlintFormat
	cd server && gofmt -w .

fix:
	$(PY) scripts/backlog_index.py --docs $(DOCS) --backlog $(BACKLOG)
	$(PY) scripts/coverage_map.py --fix --docs $(DOCS)

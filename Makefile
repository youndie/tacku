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

.PHONY: check gate docs server client spec report probes fix help

help:
	@echo "make check   - the gate: blocking checks, exactly what CI runs"
	@echo "make spec    - regenerate the committed KOMPOT spec of this build"
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
client:
	cd client && ./gradlew --quiet :spec-gen:test

server:
	cd server && gofmt -l . | tee /dev/stderr | (! read)
	cd server && go vet ./...
	cd server && go test ./...

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

fix:
	$(PY) scripts/backlog_index.py --docs $(DOCS) --backlog $(BACKLOG)
	$(PY) scripts/coverage_map.py --fix --docs $(DOCS)

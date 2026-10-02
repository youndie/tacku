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
#
# The documentation checks are docs-bootstrap's: this file is its templates/Makefile merged into
# this repository's own, next to .github/workflows/check.yaml copied from
# templates/workflow-check.yaml.
#
#   make check    the gate and the reports - exactly what CI runs
#   make fix      regenerate the backlog index, append missing coverage-map lines
#
# ONE VERSION OF THE CHECKS, WRITTEN DOWN ONCE: the `uses: youndie/docs-bootstrap@<ref>` line in
# .github/workflows/check.yaml. CI runs the checks at that ref because the runner resolves the line.
# This file reads the same line and fetches the same ref into .docs-bootstrap/, a directory that
# ignores itself, so `make check` here runs what CI runs - the same scripts, the same guard, the same
# flags. Renovate bumps the line, and the next `make check` fetches what CI already moved to.
#
# WHY THE SCRIPTS ARE NOT COPIED IN. A copied check runs, but at the version of the day it was copied,
# and a fix upstream never arrives: across one portfolio 18 copies of backlog_index.py were found in
# three versions, eleven of them without the guard that makes `--check` fail when the backlog has
# gone missing - a guard that existed upstream the whole time.
#
# WHY THE VERSION IS NOT ALSO WRITTEN HERE. A version pinned in the workflow and again in this file is
# two pins, and two pins drift: one is bumped, the other is found months later, and "green here, red
# there" comes back with nobody able to say which side is right. So this file holds none; if the
# workflow names two different refs, it refuses to choose.
#
# WHAT LIVES HERE is what is this repository's own: where the tree is, how the backlog is kept, and
# checks of its own under `gate`. How the documents are checked - including the guard that fails the
# gate when docs/ or the backlog is not there - is in check.mk at the pinned version, and changes
# arrive with a bump instead of with a re-copy.
#
# ONLY A GOAL THAT RUNS THE CHECKS LOADS THEM. A project adds targets of its own below this head - a
# chart, a stand, a release - and make reads every included file, fetching the ones that are
# missing, before it runs any goal at all. Included unconditionally, check.mk made each of those
# targets, `make` alone and even `make -n` read the pin and download it on a fresh clone, and fail
# offline. So it is included only when a goal asked for - on the command line, or the default goal
# when there is none - is in DOCS_BOOTSTRAP_GOALS or is one of check.mk's own `docs-` targets; every
# other goal runs without docs-bootstrap and without the network. A goal of the project's own that
# leads to the checks (`ci: check build`) is added to DOCS_BOOTSTRAP_GOALS, above the line that says
# nothing below is meant to be edited; one that is not added stops on a message naming that
# variable.
#
# OVERRIDES. `DOCS_BOOTSTRAP=<dir>` runs the checks from a directory instead of the pinned ref: a
# clone of docs-bootstrap you are changing, or - offline, or without GitHub Actions - a committed
# copy of its check.mk, scripts/ and .claude-plugin/. That last one is the copy route again, with its
# drift; it is the fallback, not the default.

DOCS ?= docs
BACKLOG ?= backlog.md
# How the backlog is kept (docs-bootstrap SKILL.md, step 7): `files` - one file per item in
# $(DOCS)/backlog/ and the generated index in $(BACKLOG); `milestones` - one hand-kept file at
# $(BACKLOG), usually BACKLOG.md; `none` - no backlog, yet.
BACKLOG_FORM ?= files
REPOS ?= ..
PY ?= python3

# Where the pin is, and what it names.
DOCS_BOOTSTRAP_PIN ?= .github/workflows/check.yaml
DOCS_BOOTSTRAP_REPO ?= youndie/docs-bootstrap
DOCS_BOOTSTRAP_CACHE ?= .docs-bootstrap
# The revision of this file. check.mk says so when a newer docs-bootstrap expects a newer one.
DOCS_BOOTSTRAP_SHIM := 2

# The goals that load the checks - and so read the pin and, on a fresh clone, fetch it. check.mk's
# `docs-` targets load them by themselves. A goal of this repository's own that runs one of these
# goes here too, e.g. for `ci: check build`:
#	DOCS_BOOTSTRAP_GOALS += ci
DOCS_BOOTSTRAP_GOALS := check gate report fix
# `docs` is the documentation third of `gate`: docs-bootstrap's `docs-gate` and this repository's
# own guards below it, runnable alone.
DOCS_BOOTSTRAP_GOALS += docs

.DEFAULT_GOAL := help
.PHONY: measure check gate docs server client spec tck probe shots format report probes fix help

help:
	@echo "make check   - the gate and the reports: exactly what CI runs"
	@echo "make gate    - the blocking half alone: documents, client, server"
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

# docs-gate is docs-bootstrap's half at the pinned version: the guard that fails when docs/ or the
# backlog is not there, the backlog index, the documents, the coverage map. The lines below it are
# this repository's own.
docs: docs-gate
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
	@lsof -ti:8477 -ti:8478 -ti:8479 2>/dev/null | xargs kill -9 2>/dev/null || true
	@rm -f /tmp/tacku-probe.db
	@cd server && go run ./cmd/tacku seed -db /tmp/tacku-probe.db
	@cd server && go run ./cmd/devauth -addr :8478 >/dev/null 2>&1 & sleep 4
	@# A stand-in for the forge, so the walk meets the read-only view over another repository's
	@# backlog. Without a source that board is absent from the graph and its card is never pressed —
	@# which is how a client that asked for the wrong shape of body reached a person first.
	@python3 scripts/docs_stub.py --root scripts/fixtures/docs-source --addr 127.0.0.1:8479 >/dev/null 2>&1 & sleep 1
	@# Built with the instrument's door, which is what a stand is: a release build serves no sign-in
	@# form, and this walk signs in through one. Without the tag the server starts and answers
	@# `unauthenticated` to every screen — a stand that is up and useless.
	@cd server && TACKU_RESOURCE=http://localhost:8477 \
		TACKU_ISSUER=http://localhost:8478 TACKU_JWKS_URL=http://localhost:8478/jwks \
		TACKU_SESSION_KEY=a-key-of-at-least-thirty-two-characters \
		TACKU_DOCS_API=http://127.0.0.1:8479 \
		TACKU_DOCS_SOURCES='[{"key":"example","title":"A lending system","repo":"example/docs","root":"backlog"}]' \
		go run -tags debugdoor ./cmd/tacku serve -db /tmp/tacku-probe.db -addr :8477 >/dev/null 2>&1 & sleep 5
	@cd client && ./gradlew --quiet :app:probe --console=plain; \
		status=$$?; \
		lsof -ti:8477 -ti:8478 -ti:8479 2>/dev/null | xargs kill -9 2>/dev/null || true; \
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
report: docs-report

# Every module the gate checks, and that list is the point: `make check` runs `ktlintCheck` across
# the client, while this used to format one module of it. The other two were formatted by hand or
# not at all, and the difference showed up as a red gate after a change that had just been
# "formatted".
format:
	cd client && ./gradlew --quiet ktlintFormat
	cd server && gofmt -w .

fix: docs-fix

# -- where the checks come from. Nothing below is meant to be edited. ------------------------------

# The goals this run was asked for: the command line's, or the default goal when it names none.
DOCS_BOOTSTRAP_ASKED := $(or $(MAKECMDGOALS),$(.DEFAULT_GOAL))

ifneq ($(filter $(DOCS_BOOTSTRAP_GOALS) docs-%,$(DOCS_BOOTSTRAP_ASKED)),)

ifndef DOCS_BOOTSTRAP
DOCS_BOOTSTRAP_REF := $(sort $(shell sed -n -E 's|^[[:space:]]*(-[[:space:]]*)?uses:[[:space:]]*"?$(DOCS_BOOTSTRAP_REPO)@([^"[:space:]]+).*|\2|p' $(DOCS_BOOTSTRAP_PIN) 2>/dev/null))
ifeq ($(words $(DOCS_BOOTSTRAP_REF)),0)
$(error no `uses: $(DOCS_BOOTSTRAP_REPO)@<ref>` in $(DOCS_BOOTSTRAP_PIN). That line is the version of the checks, for CI and for this file alike - copy templates/workflow-check.yaml, or run with DOCS_BOOTSTRAP=<a local copy>)
endif
ifneq ($(words $(DOCS_BOOTSTRAP_REF)),1)
$(error $(DOCS_BOOTSTRAP_PIN) pins $(DOCS_BOOTSTRAP_REPO) at more than one ref: $(DOCS_BOOTSTRAP_REF). One version of the checks, one ref - make every uses: line name the same one)
endif
DOCS_BOOTSTRAP := $(DOCS_BOOTSTRAP_CACHE)/$(DOCS_BOOTSTRAP_REF)
else ifeq ($(wildcard $(DOCS_BOOTSTRAP)/check.mk),)
$(error DOCS_BOOTSTRAP=$(DOCS_BOOTSTRAP) holds no check.mk)
endif

include $(DOCS_BOOTSTRAP)/check.mk

else

# Not loaded, so no `docs-` target exists in this run. A goal that reaches one anyway is missing from
# DOCS_BOOTSTRAP_GOALS, and make's own "No rule to make target" would not say so.
docs-%:
	@echo "$@ is a target of docs-bootstrap's check.mk, which this run did not load: '$(DOCS_BOOTSTRAP_ASKED)' is not in DOCS_BOOTSTRAP_GOALS ($(strip $(DOCS_BOOTSTRAP_GOALS))). Add the goal that leads to $@ to DOCS_BOOTSTRAP_GOALS in the Makefile." >&2; exit 2

endif

# The fetch. A tarball of the ref rather than a clone: a tag, a branch and a commit SHA (what
# Renovate writes when it pins digests) are all one URL, and no history is needed. Unpacked next to
# its final place and moved in only once complete, so an interrupted fetch never leaves a directory
# that looks like a version. GNU make 3.81 - the one macOS ships - announces the missing file
# ("check.mk: No such file or directory") just before fetching it; that line is not the error.
$(DOCS_BOOTSTRAP_CACHE)/%/check.mk:
	@echo "docs-bootstrap: fetching $(DOCS_BOOTSTRAP_REPO)@$* - the ref $(DOCS_BOOTSTRAP_PIN) pins"
	@rm -rf "$(@D).part" && mkdir -p "$(@D).part"
	@curl -fsSL --retry 2 -o "$(@D).part/src.tar.gz" "https://codeload.github.com/$(DOCS_BOOTSTRAP_REPO)/tar.gz/$*" || { rm -rf "$(@D).part"; echo "could not fetch $(DOCS_BOOTSTRAP_REPO)@$* - offline, or a ref that does not exist? DOCS_BOOTSTRAP=<dir> runs a local copy instead" >&2; exit 1; }
	@tar -xzf "$(@D).part/src.tar.gz" -C "$(@D).part" --strip-components=1 && rm -f "$(@D).part/src.tar.gz"
	@test -f "$(@D).part/check.mk" || { echo "$(DOCS_BOOTSTRAP_REPO)@$* has no check.mk - versions before 0.3.0 cannot be pinned this way" >&2; rm -rf "$(@D).part"; exit 1; }
	@rm -rf "$(@D)" && mv "$(@D).part" "$(@D)"
	@echo '*' > "$(DOCS_BOOTSTRAP_CACHE)/.gitignore"

#!/usr/bin/env bash
# Refreshes the frozen screen bodies the screenshots are taken from.
#
# The pictures are of real responses, not of fragments somebody typed — that is the whole point of
# them — but they are of responses recorded once. A golden compared against a live server would go
# red for a clock as readily as for a defect, and half of what it caught would be the date.
#
# So the bodies are refreshed deliberately, by running this, when the server's own output changes.
# That the server still produces what these say is the Go half's job and it has its own tests.
set -euo pipefail

root="$(cd "$(dirname "$0")/.." && pwd)"
out="$root/client/app/src/test/screens"
db=/tmp/tacku-screens.db
# The moment the corpus is stamped at. Changing it rewrites every picture carrying a time, so it is
# a decision rather than a default: pick one and leave it.
stamp=2026-08-23T09:00:00Z
# Overridable, because the refusal above is only useful if there is somewhere else to go: the usual
# ports are where a stand for looking at the product lives.
port=${TACKU_SCREENS_PORT:-8477}
authport=${TACKU_SCREENS_AUTHPORT:-8478}
# The stand-in for the forge. The read-only view over a backlog kept in a repository is a screen like
# any other and is photographed like one — from a body a real server produced — and this is what
# saves that from needing a real repository, a credential and a network.
docsport=${TACKU_SCREENS_DOCSPORT:-8479}

# Only what this script started.
#
# It used to kill whatever held the two ports, which is a script reaching outside its own work: it
# took down a stand somebody was looking at, twice, and the symptom on their side was a white window
# with no explanation. If a port is already busy the honest thing is to refuse, not to clear it.
started=""

cleanup() {
  for pid in $started; do
    kill "$pid" 2>/dev/null || true
  done
  rm -f "$db"
  rm -rf "$bin"
}
# On EXIT rather than at the end: a failure halfway through would otherwise leave a server holding
# the port, and the next run would talk to it and record whatever that one happened to serve.
trap cleanup EXIT

for busy in $port $authport $docsport; do
  if lsof -ti:"$busy" >/dev/null 2>&1; then
    echo "port $busy is already in use — stop what is on it, or it will be recorded instead of a fresh server" >&2
    exit 1
  fi
done
rm -f "$db"
cd "$root/server"

# Built and then run, rather than run through `go run`.
#
# `go run` compiles to a temporary binary and starts it as a child; killing the process it left in
# `$!` kills the wrapper and leaves the server holding the port. Two runs in a row therefore refused
# to start, correctly, blaming a port that this very script had left busy. Building first makes the
# pid in `$!` the server's own.
bin=$(mktemp -d)
go build -o "$bin/tacku" ./cmd/tacku
go build -o "$bin/devauth" ./cmd/devauth
go build -tags debugdoor -o "$bin/tacku-door" ./cmd/tacku
# A fixed instant, so two refreshes of the corpus differ only where the server changed. Without it
# the journal stamps itself with the wall clock and every refresh rewrote two goldens by the minute
# — a diff somebody has to triage each time, which is where a real change gets waved through.
"$bin/tacku" seed -db "$db" -at "$stamp" >/dev/null
"$bin/devauth" -addr :$authport >/dev/null 2>&1 &
started="$started $!"
python3 "$root/scripts/docs_stub.py" --root "$root/scripts/fixtures/docs-source" --addr 127.0.0.1:$docsport >/dev/null 2>&1 &
started="$started $!"
sleep 4
# The instrument's door, which is what a stand is: a release build serves no sign-in form, and this
# script signs in through one.
#
# The comment is above the command and not inside it, which is not style. It used to stand between
# two continuation lines, and a `#` there ends the command: the shell ran the assignments as a bare
# assignment list and started the server with none of them. Nothing failed — the door this build
# carries needs no provider, and a missing session key is generated — so the recording had been
# taken from a server configured by accident. Checked by asking the shell rather than by reading it.
TACKU_RESOURCE=http://localhost:$port \
  TACKU_ISSUER=http://localhost:$authport \
  TACKU_JWKS_URL=http://localhost:$authport/jwks \
  TACKU_SESSION_KEY=a-key-of-at-least-thirty-two-characters \
  TACKU_DOCS_API=http://127.0.0.1:$docsport \
  TACKU_DOCS_REPO=example/docs \
  TACKU_DOCS_ROOT=backlog \
  TACKU_DOCS_TOKEN=a-fixture-needs-none \
  "$bin/tacku-door" serve -db "$db" -addr :$port >/dev/null 2>&1 &
started="$started $!"
sleep 6

# The stand's own credentials, printed by `seed` — not anybody's.
token=$(curl -sS -X POST -H "Content-Type: application/json" -H "Idempotency-Key: screens-$$" \
  -d '{"formId":"sign-in","values":{"email":{"type":"text_value","text":"anna@tacku.team"},"password":{"type":"text_value","text":"conformance-stand"}}}' \
  "http://localhost:$port/submit/sign-in" | python3 -c 'import json,sys; print(json.load(sys.stdin)["accessToken"])')

fetch() {
  local name=$1 path=$2
  local status
  status=$(curl -sS -o "$out/$name.json.raw" -w '%{http_code}' -H "Authorization: Bearer $token" "http://localhost:$port/$path")
  if [ "$status" != "200" ]; then
    echo "$name: $status from /$path" >&2
    exit 1
  fi
  python3 -c "
import json,sys
d=json.load(open('$out/$name.json.raw'))
with open('$out/$name.json','w') as f:
    json.dump(d,f,indent=1,ensure_ascii=False)
    f.write('\n')"
  rm -f "$out/$name.json.raw"
  echo "$name"
}

fetch sign-in forms/sign-in
fetch catch-up screens/catch-up
fetch board screens/board
fetch my-tasks forms/my-tasks
fetch new-task forms/new-task
fetch new-board forms/new-board
fetch docs-board screens/docs-board

# The task screen needs a task, and which one it is comes from the board rather than from a constant
# that would go stale the first time the seed changed.
task=$(python3 -c "
import json,re
print(re.findall(r'app://task/([A-Z]+-[0-9]+)', open('$out/board.json').read())[0])")
fetch task "forms/task/$task"

echo "screens refreshed from a seeded server; re-record the goldens with: make shots"

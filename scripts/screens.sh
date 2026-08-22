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
port=8477
authport=8478

cleanup() {
  lsof -ti:$port -ti:$authport 2>/dev/null | xargs kill -9 2>/dev/null || true
  rm -f "$db"
}
# On EXIT rather than at the end: a failure halfway through would otherwise leave a server holding
# the port, and the next run would talk to it and record whatever that one happened to serve.
trap cleanup EXIT

cleanup
cd "$root/server"
go run ./cmd/tacku seed -db "$db" >/dev/null
go run ./cmd/devauth -addr :$authport >/dev/null 2>&1 &
sleep 4
TACKU_RESOURCE=http://localhost:$port \
  TACKU_ISSUER=http://localhost:$authport \
  TACKU_JWKS_URL=http://localhost:$authport/jwks \
  TACKU_SESSION_KEY=a-key-of-at-least-thirty-two-characters \
  go run ./cmd/tacku serve -db "$db" -addr :$port >/dev/null 2>&1 &
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

# The task screen needs a task, and which one it is comes from the board rather than from a constant
# that would go stale the first time the seed changed.
task=$(python3 -c "
import json,re
print(re.findall(r'app://task/([A-Z]+-[0-9]+)', open('$out/board.json').read())[0])")
fetch task "forms/task/$task"

echo "screens refreshed from a seeded server; re-record the goldens with: make shots"

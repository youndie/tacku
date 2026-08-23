#!/usr/bin/env bash
# Ask the agent surface a real question with a real token, and say what happened.
#
# It exists because nothing else answers this. Every other check — the walk, the tests, the probes —
# either brings its own stand or asks the deployed server something that needs no token, and a
# surface nobody has ever authenticated against looks exactly like a working one: it answers 401 to
# everybody, which is also what it does when it is broken.
#
# Nothing secret is printed. The client's secret is read from the environment and used once; the
# token never leaves this script; what comes out is the shape of the token's claims and the answer
# the server gave.
#
# The resource is **asked of the server**, not passed in. The first version took it as an argument
# and then compared the token's audience against that same argument — which is true for whatever you
# type, and said "fits: yes" about a token the server refused. A comparison whose two sides come
# from the same place answers nothing.
set -euo pipefail

# No apostrophes in these messages, and that is not style. Inside ${var:?word} bash treats a single
# quote as opening a quoted string, so one of them swallows the rest of the file. The first version
# of this script had two — in "provider's" and "server's" — which cancelled each other out, and
# `bash -n` passed on a file that was correct by accident.
: "${SERVER:?set SERVER to the address of this server, e.g. https://tacku.example}"
: "${CLIENT_ID:?set CLIENT_ID to the agent client}"
: "${CLIENT_SECRET:?set CLIENT_SECRET to the secret of that client}"

server="${SERVER%/}"

metadata="$(curl -fsS --max-time 20 "$server/.well-known/oauth-protected-resource")"
resource="$(printf '%s' "$metadata" | python3 -c 'import json,sys; print(json.load(sys.stdin)["resource"])')"
issuer="$(printf '%s' "$metadata" | python3 -c 'import json,sys; print(json.load(sys.stdin)["authorization_servers"][0])')"

echo "  server names its resource $resource"
echo "  and its provider          $issuer"

token_endpoint="$(curl -fsS --max-time 20 "$issuer/.well-known/openid-configuration" |
    python3 -c 'import json,sys; print(json.load(sys.stdin)["token_endpoint"])')"

token="$(curl -fsS --max-time 20 -X POST "$token_endpoint" \
    -d grant_type=client_credentials \
    --data-urlencode "client_id=$CLIENT_ID" \
    --data-urlencode "client_secret=$CLIENT_SECRET" \
    --data-urlencode "resource=$resource" |
    python3 -c 'import json,sys; print(json.load(sys.stdin)["access_token"])')"

# The claims, not the token. `aud` is the whole question: a token without it is refused by this
# server, and that refusal is indistinguishable from a wrong secret unless you look here.
python3 - "$token" "$resource" <<'PY'
import base64, json, sys

payload = sys.argv[1].split(".")[1]
claims = json.loads(base64.urlsafe_b64decode(payload + "=" * (-len(payload) % 4)))
audience = claims.get("aud")
print(f"  azp   {claims.get('azp')}")
print(f"  aud   {audience if audience is not None else 'MISSING — this server will refuse it'}")
print(f"  scope {claims.get('scope', '(none)')}")
if audience is not None:
    wanted = sys.argv[2]
    fits = wanted in (audience if isinstance(audience, list) else [audience])
    print(f"  fits  {'yes' if fits else f'no — it names something other than {wanted}'}")
PY

status="$(curl -s -o /tmp/tacku-agent-answer.json -w '%{http_code}' --max-time 30 \
    -X POST "$server/mcp" \
    -H "Authorization: Bearer $token" \
    -H "Content-Type: application/json" \
    -H "Accept: application/json, text/event-stream" \
    -d '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"check-agent","version":"0"}}}')"

echo "  mcp   initialize → $status"
if [ "$status" = "200" ]; then
    echo "  the agent surface answered a real token. This is the check nothing else makes."
else
    echo "  refused. The answer is in /tmp/tacku-agent-answer.json; the claims above say whether the"
    echo "  audience is the reason."
    exit 1
fi

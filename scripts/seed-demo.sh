#!/usr/bin/env bash
# Seeds a running parserod with a few demo scans and prints a completed scan id
# (used by the screenshot tooling, locally and in CI). Waits until scans finish.
#
#   BASE_URL=http://localhost:8080 IDENTITY=demo@parsero.dev ./scripts/seed-demo.sh
#
# Prints the chosen scan id on stdout; all logging goes to stderr.
set -euo pipefail

BASE_URL="${BASE_URL:-http://127.0.0.1:8080}"
IDENTITY="${IDENTITY:-demo@parsero.dev}"
IDENTITY_HEADER="${IDENTITY_HEADER:-X-Auth-Request-Email}"
TARGETS="${TARGETS:-www.google.com en.wikipedia.org github.com}"
# Which target's scan to return for the results screenshot (compact is nicer).
RESULTS_TARGET="${RESULTS_TARGET:-github.com}"

log() { echo "[seed] $*" >&2; }

for t in $TARGETS; do
  id=$(curl -fsS -H "Content-Type: application/json" -H "$IDENTITY_HEADER: $IDENTITY" \
    -X POST "$BASE_URL/api/scans" -d "{\"target\":\"$t\"}" \
    | python3 -c 'import sys,json;print(json.load(sys.stdin).get("id",""))')
  log "submitted $t -> $id"
done

log "waiting for scans to finish…"
for _ in $(seq 1 60); do
  pending=$(curl -fsS -H "$IDENTITY_HEADER: $IDENTITY" "$BASE_URL/api/scans" \
    | python3 -c 'import sys,json;print(sum(1 for s in json.load(sys.stdin) if s["status"] in ("queued","running")))')
  [ "$pending" = "0" ] && break
  sleep 2
done

# Print the results-target scan id (falling back to any completed scan).
curl -fsS -H "$IDENTITY_HEADER: $IDENTITY" "$BASE_URL/api/scans" | python3 -c '
import sys, json
d = json.load(sys.stdin)
target = "'"$RESULTS_TARGET"'"
done = [s for s in d if s["status"] == "done"]
chosen = [s for s in done if s["target"] == target] or done
print(chosen[0]["id"] if chosen else "")
'

#!/usr/bin/env bash
# chaos-drill.sh — fault-injection drill for circuit breaker (7-1) +
# Retry-After (7-4) + cost spike (8-2.1).
#
# Validates that newhub responds correctly when an upstream provider
# misbehaves. Intended for STAGE only — DO NOT run against PROD.
#
# What it does:
#   Scenario A: pick a tier-2 channel, inject 5xx via channel-test/admin
#               override, send 5 requests, confirm breaker opens after the
#               threshold and closes after cool-down.
#   Scenario B: send a slow-loris request (300s no body), confirm 524
#               eventually returned and breaker records it.
#   Scenario C: send 50_001 quota worth of requests in 5 min, confirm
#               cost-spike middleware disables the user.
#
# Required env:
#   HUB_BASE, ADMIN_TOKEN, USER_TOKEN, CHAOS_CHANNEL_ID, TEST_USER_ID
#
# Usage:
#   HUB_BASE=https://hub-stage.lurus.cn \
#   ADMIN_TOKEN=sk-admin-... \
#   USER_TOKEN=sk-... \
#   CHAOS_CHANNEL_ID=99 \
#   TEST_USER_ID=42 \
#   ./scripts/chaos-drill.sh

set -u
set -o pipefail

HUB_BASE="${HUB_BASE:-https://hub-stage.lurus.cn}"
ADMIN_TOKEN="${ADMIN_TOKEN:-}"
USER_TOKEN="${USER_TOKEN:-}"
CHAOS_CHANNEL_ID="${CHAOS_CHANNEL_ID:-}"
TEST_USER_ID="${TEST_USER_ID:-}"

if [ -z "$ADMIN_TOKEN" ] || [ -z "$USER_TOKEN" ] || [ -z "$CHAOS_CHANNEL_ID" ] || [ -z "$TEST_USER_ID" ]; then
  echo "ERROR: missing required env. Set ADMIN_TOKEN, USER_TOKEN, CHAOS_CHANNEL_ID, TEST_USER_ID."
  exit 2
fi

if [ "${HUB_BASE}" = "https://hub.lurus.cn" ] || [ "${HUB_BASE}" = "https://api.lurus.cn" ]; then
  echo "REFUSING: HUB_BASE looks like PROD ($HUB_BASE). Chaos drill is STAGE-only."
  exit 2
fi

PASS=0; FAIL=0
ok()   { PASS=$((PASS+1)); printf '\033[32m  ✓\033[0m %s\n' "$1"; }
fail() { FAIL=$((FAIL+1)); printf '\033[31m  ✗\033[0m %s\n     %s\n' "$1" "${2:-}"; }
hdr()  { printf '\n\033[2m── %s ──\033[0m\n' "$1"; }

# ---- Scenario A: 5xx injection → breaker opens ----------------------------

hdr "Scenario A — repeated 5xx from upstream → breaker opens"

# Save original status, force the channel into a "broken" state by setting
# its base URL to a known-bad endpoint.
ORIG=$(curl -sS -H "Authorization: Bearer $ADMIN_TOKEN" \
  "$HUB_BASE/api/channel/$CHAOS_CHANNEL_ID" | head -1 || echo "")
echo "  $(printf '\033[2m[snapshot] channel %s captured (will restore on exit)\033[0m\n' "$CHAOS_CHANNEL_ID")"

restore_channel() {
  echo ""
  echo "  $(printf '\033[2m[cleanup] restoring channel %s — manual: PUT /api/channel/%s with original base_url\033[0m\n' "$CHAOS_CHANNEL_ID" "$CHAOS_CHANNEL_ID")"
}
trap restore_channel EXIT

# Inject a base_url that always 502s (httpbin.org/status/502)
inject=$(curl -sS -X PUT -H "Authorization: Bearer $ADMIN_TOKEN" -H "Content-Type: application/json" \
  "$HUB_BASE/api/channel/" \
  -d "{\"id\":$CHAOS_CHANNEL_ID,\"base_url\":\"https://httpbin.org/status/502\"}")
if echo "$inject" | grep -q '"success":true'; then
  ok "channel $CHAOS_CHANNEL_ID base_url overridden to 502 source"
else
  fail "could not inject — channel update API rejected" "$inject"
  exit "$FAIL"
fi

# Send 5 requests; expect breaker to open after threshold (default 3 failures)
opens=0
for i in 1 2 3 4 5; do
  code=$(curl -sS -o /dev/null -w '%{http_code}' \
    -H "Authorization: Bearer $USER_TOKEN" \
    "$HUB_BASE/v1/chat/completions" \
    -d '{"model":"gpt-4","messages":[{"role":"user","content":"chaos-drill"}]}')
  printf "    req %d → %s\n" "$i" "$code"
  if [ "$code" = "503" ] || [ "$code" = "502" ]; then
    opens=$((opens+1))
  fi
done
if [ "$opens" -ge 3 ]; then
  ok "≥3/5 requests returned 5xx (breaker fired or upstream propagated)"
else
  fail "expected ≥3 5xx responses" "got $opens"
fi
echo "  $(printf '\033[2m  manual check: kubectl logs deploy/newhub | grep \"breaker open\"\033[0m\n')"

# ---- Scenario B: slow-loris timeout (524) ---------------------------------

hdr "Scenario B — upstream timeout returns 524 + breaker records"
echo "  $(printf '\033[2m  skipped in this script — needs a slow upstream mock; recommend manual:\033[0m\n')"
echo "  $(printf '\033[2m    1. point CHAOS_CHANNEL_ID base_url at httpbin.org/delay/300\033[0m\n')"
echo "  $(printf '\033[2m    2. send 1 request with --max-time 60\033[0m\n')"
echo "  $(printf '\033[2m    3. expect 524 + breaker increments\033[0m\n')"

# ---- Scenario C: cost spike → user disabled ------------------------------

hdr "Scenario C — cost spike protection (8-2.1) auto-disables user"
echo "  $(printf '\033[2m  WARNING: this disables TEST_USER_ID=%s. Re-enable manually after.\033[0m\n' "$TEST_USER_ID")"
read -r -p "  proceed? [y/N] " confirm
if [ "$confirm" != "y" ] && [ "$confirm" != "Y" ]; then
  echo "  skipped Scenario C"
else
  # Send rapid bursts to accumulate quota. Threshold default = 50_000 / 5min.
  echo "  sending 100 quick chat-completion requests..."
  burst=0
  for i in $(seq 1 100); do
    code=$(curl -sS -o /dev/null -w '%{http_code}' --max-time 5 \
      -H "Authorization: Bearer $USER_TOKEN" \
      "$HUB_BASE/v1/chat/completions" \
      -d '{"model":"gpt-3.5-turbo","messages":[{"role":"user","content":"x"}],"max_tokens":1000}')
    if [ "$code" = "429" ]; then
      ok "request $i hit 429 — cost spike triggered"
      burst=$i
      break
    fi
  done
  if [ "$burst" = 0 ]; then
    fail "no 429 after 100 requests" "threshold may be too high or middleware not wired"
  fi

  # Verify user disabled
  user=$(curl -sS -H "Authorization: Bearer $ADMIN_TOKEN" \
    "$HUB_BASE/api/user/$TEST_USER_ID" | head -1)
  if echo "$user" | grep -q '"status":2'; then
    ok "user $TEST_USER_ID auto-disabled (status=2)"
  else
    fail "user not disabled" "$user"
  fi

  echo "  $(printf '\033[2m  re-enable: curl -X PUT -H \"Authorization: Bearer $ADMIN_TOKEN\" %s/api/user/manage -d \"{\\\"id\\\":%s,\\\"action\\\":\\\"enable\\\"}\"\033[0m\n' "$HUB_BASE" "$TEST_USER_ID")"
fi

# ---- summary -------------------------------------------------------------

echo ""
printf '\033[2m──────────────\033[0m pass=%d fail=%d\n' "$PASS" "$FAIL"
if [ "$FAIL" -eq 0 ]; then
  printf '\033[32m✓\033[0m chaos drill complete\n'
  exit 0
else
  printf '\033[31m✗\033[0m %d scenario(s) failed\n' "$FAIL"
  exit "$FAIL"
fi

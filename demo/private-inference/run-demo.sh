#!/usr/bin/env bash
# One-click runner for the private-inference-routing demo. Git Bash (MSYS2)
# compatible. Brings up: demo Postgres (docker) -> newhub backend (go run,
# waits for auto-migrate) -> seeds (tenant + private channel + console-viewer
# admin) -> mock on-prem OpenAI-compatible endpoint -> one real
# /v1/chat/completions relay call as the privacy-demo tenant, then prints the
# mock endpoint's request log as proof the call was routed to the private
# on-prem endpoint and never left the host.
#
# Usage:
#   bash demo/private-inference/run-demo.sh          # start everything + fire proof call
#   bash demo/private-inference/run-demo.sh stop      # stop backend+mock (docker container is left running)
#
# Re-run-safe: every step checks "already up" before acting, and both seed
# files are idempotent SQL (ON CONFLICT DO NOTHING / DO UPDATE).
#
# What this intentionally does NOT touch: repo-root .env / .env.dev (used by
# other local dev work) — everything here reads demo/private-inference/.env.demo
# instead, and the demo Postgres runs in its own docker container on its own
# port so it never collides with docker-compose.dev.yml's dev stack.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
cd "$REPO_ROOT"

DEMO_DIR="demo/private-inference"
ENV_FILE="$DEMO_DIR/.env.demo"
PG_CONTAINER="newhub-pgdemo"
PG_PORT=5435
BACKEND_PORT=8099
MOCK_PORT=11434
DEMO_TOKEN="privacydemo00000000000000000000000000onprem01"
LOG_DIR="${TMPDIR:-/tmp}/newhub-private-inference-demo"
mkdir -p "$LOG_DIR"

step() { printf '\n=== %s ===\n' "$1"; }

# --- teardown mode -----------------------------------------------------------
# NOTE: we deliberately do NOT kill by the bash `$!` PID recorded at start —
# under Git Bash/MSYS2, `$!` after `nohup cmd &` is the MSYS2 job-control
# PID, not the real Windows PID of the spawned executable (go run's actual
# compiled server, or bun.exe), so `taskkill //PID $!` silently no-ops. Look
# up the real PID by the port it's listening on instead.
kill_by_port() {
  local port="$1"
  local pid
  pid="$(netstat -ano | grep "LISTENING" | grep -E ":${port}[[:space:]]" | awk '{print $NF}' | sort -u | head -n1)"
  if [ -n "${pid:-}" ]; then
    taskkill //F //PID "$pid" >/dev/null 2>&1 && echo "killed pid $pid (port $port)" || echo "could not kill pid $pid (port $port)"
  else
    echo "nothing listening on port $port"
  fi
}

if [ "${1:-}" = "stop" ]; then
  step "stopping backend + mock endpoint (docker container left running)"
  kill_by_port "$BACKEND_PORT"
  kill_by_port "$MOCK_PORT"
  rm -f "$LOG_DIR/backend.pid" "$LOG_DIR/mock.pid"
  echo "Postgres container '$PG_CONTAINER' left running (docker stop $PG_CONTAINER to fully tear down)."
  exit 0
fi

# --- 0) prerequisites --------------------------------------------------------
step "0/5 prerequisites"
missing=""
for cmd in docker go bun curl; do
  command -v "$cmd" >/dev/null 2>&1 || missing="$missing $cmd"
done
if [ -n "$missing" ]; then
  echo "!! missing required tool(s):$missing"
  echo "   install them first (README.md Prerequisites): docker (demo PG), go (backend),"
  echo "   bun (mock endpoint), curl (proof call)."
  exit 1
fi
echo "docker/go/bun/curl all present"

# --- 1) demo Postgres (docker) -----------------------------------------------
step "1/5 Postgres (docker container '$PG_CONTAINER', port $PG_PORT)"

pg_host_port_bound() {
  docker inspect "$PG_CONTAINER" --format '{{json .NetworkSettings.Ports}}' 2>/dev/null \
    | grep -q "\"HostPort\":\"${PG_PORT}\""
}

if docker ps -a --format '{{.Names}}' | grep -qx "$PG_CONTAINER"; then
  docker start "$PG_CONTAINER" >/dev/null
  echo "started existing container"
else
  docker run -d --name "$PG_CONTAINER" \
    -e POSTGRES_USER=postgres -e POSTGRES_PASSWORD=postgres -e POSTGRES_DB=newhub \
    -p "${PG_PORT}:5432" postgres:16-alpine >/dev/null
  echo "created new container"
fi

# Known Docker-Desktop-on-Windows flake: `docker start` on a container that
# has been stopped for a while can come up with HostConfig.PortBindings
# correct but NetworkSettings.Ports (the actual live port-forward proxy)
# empty — i.e. the container is healthy but nothing on the host can reach
# it. A full stop+start (docker restart) reliably re-establishes the proxy;
# `docker start` alone does not. Observed and fixed 2026-07-06.
sleep 1
if ! pg_host_port_bound; then
  echo "host port $PG_PORT not bound after start — docker restart to re-establish the port-forward proxy"
  docker restart "$PG_CONTAINER" >/dev/null
  sleep 2
fi

echo -n "waiting for postgres to accept connections (in-container)"
for _ in $(seq 1 30); do
  if docker exec "$PG_CONTAINER" pg_isready -U postgres -d newhub >/dev/null 2>&1; then echo " ok"; break; fi
  echo -n "."; sleep 1
done
# The above only proves postgres is ready INSIDE the container's network
# namespace. Gate on the actual host-visible port too (bash's /dev/tcp probe;
# supported by Git Bash/MSYS2) — this is what go run will actually dial.
echo -n "waiting for postgres to accept connections (host :$PG_PORT)"
host_ok=false
for _ in $(seq 1 20); do
  if (exec 3<>"/dev/tcp/127.0.0.1/${PG_PORT}") 2>/dev/null; then exec 3>&- 2>/dev/null; host_ok=true; echo " ok"; break; fi
  echo -n "."; sleep 1
done
if [ "$host_ok" != true ]; then
  echo
  echo "!! host port $PG_PORT still unreachable after docker restart — is something else bound to it, or is Docker Desktop's NAT stuck? 'docker inspect $PG_CONTAINER' NetworkSettings.Ports:"
  docker inspect "$PG_CONTAINER" --format '{{json .NetworkSettings.Ports}}' || true
  exit 1
fi

# --- 2) backend (go run ./cmd/server) ----------------------------------------
step "2/5 newhub backend (go run ./cmd/server, port $BACKEND_PORT)"
if curl -s -o /dev/null "http://localhost:${BACKEND_PORT}/api/status"; then
  echo "already running on :$BACKEND_PORT"
else
  set -a
  # shellcheck disable=SC1090
  source "$ENV_FILE"
  set +a
  nohup go run ./cmd/server > "$LOG_DIR/backend.log" 2>&1 &
  bg_pid=$!
  echo "$bg_pid" > "$LOG_DIR/backend.pid"
  echo -n "waiting for backend boot + auto-migrate (first run also compiles — can take a minute)"
  ok=false
  for _ in $(seq 1 120); do
    if curl -s -o /dev/null "http://localhost:${BACKEND_PORT}/api/status"; then ok=true; echo " ok"; break; fi
    # go run wraps the actual server as a child process, so the wrapper pid
    # can still look "alive" in some shells after the child fatals — check
    # for the crash marker in the log too, and fail fast instead of always
    # spinning the full timeout.
    if grep -q "^\[FATAL\]\|^exit status" "$LOG_DIR/backend.log" 2>/dev/null; then
      echo
      echo "!! backend process exited early — tail of $LOG_DIR/backend.log:"
      tail -n 40 "$LOG_DIR/backend.log" || true
      exit 1
    fi
    echo -n "."; sleep 1
  done
  if [ "$ok" != true ]; then
    echo
    echo "!! backend did not come up within 120s — tail of $LOG_DIR/backend.log:"
    tail -n 40 "$LOG_DIR/backend.log" || true
    exit 1
  fi
fi

# --- 3) apply seeds (idempotent) ---------------------------------------------
step "3/5 apply seeds"
docker exec -i "$PG_CONTAINER" psql -U postgres -d newhub -f - < "$DEMO_DIR/seed-tenant-private-endpoint.sql"
docker exec -i "$PG_CONTAINER" psql -U postgres -d newhub -f - < "$DEMO_DIR/seed-console-viewer.sql"

# --- 4) mock on-prem endpoint (bun) ------------------------------------------
step "4/5 mock private-inference endpoint (bun, port $MOCK_PORT)"
if curl -s "http://127.0.0.1:${MOCK_PORT}/v1/models" >/dev/null 2>&1; then
  echo "already running on :$MOCK_PORT"
else
  nohup bun "$DEMO_DIR/mock-openai-endpoint.mjs" > "$LOG_DIR/mock.log" 2>&1 &
  echo $! > "$LOG_DIR/mock.pid"
  # Poll for the port to actually bind before the step-5 proof call fires —
  # a bare `sleep 1` races a slow bun startup and yields a flaky connection-refused.
  echo -n "waiting for mock endpoint to bind :$MOCK_PORT"
  mock_ok=false
  for _ in $(seq 1 30); do
    if curl -s "http://127.0.0.1:${MOCK_PORT}/v1/models" >/dev/null 2>&1; then mock_ok=true; echo " ok"; break; fi
    if grep -qiE "^error|Error:" "$LOG_DIR/mock.log" 2>/dev/null; then
      echo; echo "!! mock endpoint failed to start — tail of $LOG_DIR/mock.log:"; tail -n 20 "$LOG_DIR/mock.log" || true; exit 1
    fi
    echo -n "."; sleep 1
  done
  if [ "$mock_ok" != true ]; then
    echo; echo "!! mock endpoint did not bind :$MOCK_PORT within 30s — tail of $LOG_DIR/mock.log:"; tail -n 20 "$LOG_DIR/mock.log" || true; exit 1
  fi
fi

# --- 5) the proof: one real relay call as tenant privacy-demo ----------------
step "5/5 relay proof — POST /v1/chat/completions as tenant privacy-demo"
curl -s "http://localhost:${BACKEND_PORT}/v1/chat/completions" \
  -H "Authorization: Bearer ${DEMO_TOKEN}" \
  -H "Content-Type: application/json" \
  -d '{"model":"qwen2.5-7b-instruct","messages":[{"role":"user","content":"where does this request go?"}]}' \
  | tee "$LOG_DIR/relay-response.json"
echo

echo
echo "--- mock endpoint request log (proof the private channel was hit) ---"
tail -n 20 "$LOG_DIR/mock.log"
echo "-----------------------------------------------------------------------"

cat <<EOF

Done.
  backend log:  $LOG_DIR/backend.log   (pid file: $LOG_DIR/backend.pid)
  mock log:     $LOG_DIR/mock.log      (pid file: $LOG_DIR/mock.pid)
  relay reply:  $LOG_DIR/relay-response.json

Optional console screenshot (see README.md "Console screenshot" section —
must run with node, not bun, on this stack; a bun-vs-node Playwright launch
incompatibility was found and documented there):
  BACKEND_URL=http://localhost:${BACKEND_PORT} \\
  E2E_BRIDGE_TOKEN="\$(grep '^E2E_BRIDGE_TOKEN=' $ENV_FILE | cut -d= -f2-)" \\
  BRIDGE_USER_ID=9101 TENANT_SLUG=privacy-demo \\
  node $DEMO_DIR/shot-console.mjs

Stop the backend+mock (docker container is left running for the next run):
  bash $DEMO_DIR/run-demo.sh stop
EOF

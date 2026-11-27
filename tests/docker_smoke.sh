#!/usr/bin/env bash
# P2-012 live Docker evidence: a deterministic, self-cleaning smoke run on
# host port 9092. It exercises the browser registration and password login
# forms, creates an API token through the web UI, authenticates against the
# Linkding-compatibility profile endpoint with `Authorization: Token`, then
# restarts the real Compose service and checks persistence/invalidation.
#
# Usage:
#   tests/docker_smoke.sh
#   LYNK_SMOKE_PORT=9092 tests/docker_smoke.sh
#
# Requires docker, docker compose and curl. All state is isolated under a
# dedicated Compose project name; the trap removes containers, the named
# volume, the scratch directory and the locally built image on exit.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
PROJECT="${LYNK_SMOKE_PROJECT:-lynk-p2-012-smoke}"
PORT="${LYNK_SMOKE_PORT:-9092}"
IMAGE="${LYNK_SMOKE_IMAGE:-lynk-p2-012-smoke}"
TAG="${LYNK_SMOKE_TAG:-smoke}"
BASE="http://127.0.0.1:${PORT}"
COMPOSE=(docker compose -f "$ROOT/compose.yaml" -p "$PROJECT")

WORK="$(mktemp -d /tmp/opencode/lynk-p2-012-smoke.XXXXXX)"

fail() {
  echo "FAIL: $*" >&2
  exit 1
}

cleanup() {
  "${COMPOSE[@]}" down -v --remove-orphans >/dev/null 2>&1 || true
  docker image rm "${IMAGE}:${TAG}" >/dev/null 2>&1 || true
  rm -rf "$WORK"
}
trap cleanup EXIT

status_of() { tr -d '\r' <"$1" | awk 'NR == 1 { print $2 }'; }
location_of() { tr -d '\r' <"$1" | awk 'tolower($1) == "location:" { print $2 }'; }
csrf_from() {
  grep -o 'name="csrf_token" value="[^"]*"' "$1" | head -n 1 | sed 's/.*value="//; s/"$//' || true
}

expect_status() {
  local file="$1" want="$2" label="$3" got
  got="$(status_of "$file")"
  if [[ "$got" != "$want" ]]; then
    cat "$file" >&2
    fail "$label: HTTP $got, want $want"
  fi
  echo "  ${label}: HTTP $got"
}

expect_location() {
  local file="$1" want="$2" label="$3" got
  got="$(location_of "$file")"
  if [[ "$got" != "$want" ]]; then
    cat "$file" >&2
    fail "$label: Location $got, want $want"
  fi
  echo "  ${label}: Location $got"
}

wait_health() {
  local label="$1" i
  for i in $(seq 1 90); do
    if curl -fsS --max-time 3 "$BASE/health" >"$WORK/health.json" 2>/dev/null; then
      grep -q '"status":"ok"' "$WORK/health.json" || fail "$label: unexpected health body: $(cat "$WORK/health.json")"
      echo "  ${label}: GET /health -> $(tr -d '\n' <"$WORK/health.json")"
      return 0
    fi
    sleep 1
  done
  "${COMPOSE[@]}" ps >&2 || true
  "${COMPOSE[@]}" logs --tail=100 >&2 || true
  fail "$label: /health did not become ready within 90s"
}

command -v docker >/dev/null 2>&1 || fail "docker not found in PATH"
docker compose version >/dev/null 2>&1 || fail "docker compose not found"
command -v curl >/dev/null 2>&1 || fail "curl not found in PATH"
[[ "$PORT" =~ ^[0-9]+$ ]] || fail "LYNK_SMOKE_PORT must be numeric"

echo "==> docker-smoke: resetting project $PROJECT on port $PORT"
"${COMPOSE[@]}" down -v --remove-orphans >/dev/null 2>&1 || true

export LYNK_PORT="$PORT"
export LYNK_ENV=development # curl cookie jars need plain-HTTP session cookies
export DOCKER_IMAGE="$IMAGE"
export DOCKER_TAG="$TAG"
export LYNK_LOGIN_RATE_LIMIT_MAX=20
export LYNK_LOGIN_RATE_LIMIT_WINDOW=1m

echo "==> docker-smoke: docker compose config"
docker compose -f "$ROOT/compose.yaml" -p "$PROJECT" config >"$WORK/compose.config"
grep -q "published: \"${PORT}\"" "$WORK/compose.config" || fail "compose config did not publish port $PORT"
grep -q 'target: 9090' "$WORK/compose.config" || fail "compose config did not target container port 9090"

echo "==> docker-smoke: docker compose up -d --build"
"${COMPOSE[@]}" up -d --build
"${COMPOSE[@]}" ps
wait_health "initial health"

echo "==> docker-smoke: register first admin through the browser form"
curl -fsS --max-time 10 -c "$WORK/register.jar" "$BASE/register" >"$WORK/register.html"
reg_csrf="$(csrf_from "$WORK/register.html")"
[[ -n "$reg_csrf" ]] || fail "registration CSRF token missing from GET /register"
curl -sS --max-time 10 -D "$WORK/register.headers" -o "$WORK/register.body" \
  -b "$WORK/register.jar" -c "$WORK/register.jar" -X POST \
  --data-urlencode "csrf_token=$reg_csrf" \
  --data-urlencode "username=smoke-admin" \
  --data-urlencode 'password=SmokePassw0rd!' \
  "$BASE/register"
expect_status "$WORK/register.headers" 303 "registration POST"
expect_location "$WORK/register.headers" "/" "registration POST"
grep -q 'lynk_session' "$WORK/register.jar" || fail "registration did not set lynk_session"

echo "==> docker-smoke: password login with a fresh cookie jar"
curl -fsS --max-time 10 -c "$WORK/login.jar" "$BASE/login" >"$WORK/login.html"
login_csrf="$(csrf_from "$WORK/login.html")"
[[ -n "$login_csrf" ]] || fail "login CSRF token missing from GET /login"
curl -sS --max-time 10 -D "$WORK/login.headers" -o "$WORK/login.body" \
  -b "$WORK/login.jar" -c "$WORK/login.jar" -X POST \
  --data-urlencode "csrf_token=$login_csrf" \
  --data-urlencode "username=smoke-admin" \
  --data-urlencode 'password=SmokePassw0rd!' \
  "$BASE/login"
expect_status "$WORK/login.headers" 303 "password login POST"
expect_location "$WORK/login.headers" "/" "password login POST"
grep -q 'lynk_session' "$WORK/login.jar" || fail "password login did not set lynk_session"

echo "==> docker-smoke: create API token in /settings/tokens"
curl -fsS --max-time 10 -b "$WORK/login.jar" -c "$WORK/login.jar" \
  "$BASE/settings/tokens" >"$WORK/tokens.html"
token_csrf="$(csrf_from "$WORK/tokens.html")"
[[ -n "$token_csrf" ]] || fail "session CSRF token missing from GET /settings/tokens"
curl -sS --max-time 10 -D "$WORK/token-create.headers" -o "$WORK/token-create.body" \
  -b "$WORK/login.jar" -c "$WORK/login.jar" -X POST \
  --data-urlencode "csrf_token=$token_csrf" \
  --data-urlencode "name=docker-smoke-token" \
  "$BASE/settings/tokens"
expect_status "$WORK/token-create.headers" 303 "token create POST"
expect_location "$WORK/token-create.headers" "/settings/tokens" "token create POST"
curl -fsS --max-time 10 -b "$WORK/login.jar" -c "$WORK/login.jar" \
  "$BASE/settings/tokens" >"$WORK/tokens-new.html"
raw_token="$(grep -o '<pre[^>]*>[0-9a-f]\{40\}</pre>' "$WORK/tokens-new.html" | grep -o '[0-9a-f]\{40\}' | head -n 1 || true)"
[[ "$raw_token" =~ ^[0-9a-f]{40}$ ]] || fail "one-time token was not displayed"
if grep -q "$raw_token" "$WORK/token-create.headers"; then
  fail "token create Location/headers leaked the raw token"
fi
echo "  one-time token displayed: ${raw_token:0:8}... (40 hex chars, prefix only logged)"

echo "==> docker-smoke: GET /api/user/profile/ with Authorization: Token"
code="$(curl -sS --max-time 10 -o "$WORK/profile.json" -w '%{http_code}' \
  -H "Authorization: Token $raw_token" "$BASE/api/user/profile/")"
[[ "$code" == "200" ]] || fail "authorized API profile = HTTP $code, want 200: $(cat "$WORK/profile.json")"
grep -q '"theme"' "$WORK/profile.json" || fail "profile body is not a Linkding profile: $(cat "$WORK/profile.json")"
echo "  authorized profile: HTTP $code $(tr -d '\n' <"$WORK/profile.json")"
code="$(curl -sS --max-time 10 -o "$WORK/profile-unauth.json" -w '%{http_code}' "$BASE/api/user/profile/")"
[[ "$code" == "401" ]] || fail "unauthenticated API profile = HTTP $code, want 401"
echo "  unauthenticated profile: HTTP $code"

echo "==> docker-smoke: docker compose restart"
"${COMPOSE[@]}" restart
"${COMPOSE[@]}" ps
wait_health "health after restart"

code="$(curl -sS --max-time 10 -o "$WORK/profile-after.json" -w '%{http_code}' \
  -H "Authorization: Token $raw_token" "$BASE/api/user/profile/")"
[[ "$code" == "200" ]] || fail "token after restart = HTTP $code, want 200: $(cat "$WORK/profile-after.json")"
echo "  token after restart: HTTP $code $(tr -d '\n' <"$WORK/profile-after.json")"

curl -sS --max-time 10 -D "$WORK/stale-session.headers" -o /dev/null -b "$WORK/login.jar" "$BASE/"
expect_status "$WORK/stale-session.headers" 303 "pre-restart session cookie"
expect_location "$WORK/stale-session.headers" "/login" "pre-restart session cookie"

echo "==> docker-smoke: password login after restart"
curl -fsS --max-time 10 -c "$WORK/relogin.jar" "$BASE/login" >"$WORK/relogin.html"
relogin_csrf="$(csrf_from "$WORK/relogin.html")"
[[ -n "$relogin_csrf" ]] || fail "login CSRF missing after restart"
curl -sS --max-time 10 -D "$WORK/relogin.headers" -o "$WORK/relogin.body" \
  -b "$WORK/relogin.jar" -c "$WORK/relogin.jar" -X POST \
  --data-urlencode "csrf_token=$relogin_csrf" \
  --data-urlencode "username=smoke-admin" \
  --data-urlencode 'password=SmokePassw0rd!' \
  "$BASE/login"
expect_status "$WORK/relogin.headers" 303 "password login after restart"
expect_location "$WORK/relogin.headers" "/" "password login after restart"

echo "==> docker-smoke: PASS (project $PROJECT, port $PORT); trap removes containers/volume/image"

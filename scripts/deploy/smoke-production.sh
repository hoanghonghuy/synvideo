#!/usr/bin/env bash
set -euo pipefail

WEB_ORIGIN="${WEB_ORIGIN:-}"
API_ORIGIN="${API_ORIGIN:-}"
DISALLOWED_ORIGIN="${DISALLOWED_ORIGIN:-https://disallowed.example}"

if [[ -z "${WEB_ORIGIN}" || -z "${API_ORIGIN}" ]]; then
  echo "WEB_ORIGIN and API_ORIGIN are required (e.g. https://app.example and https://api.example)." >&2
  exit 1
fi

pass() {
  echo "PASS: $*"
}

fail() {
  echo "FAIL: $*" >&2
  exit 1
}

request_status() {
  local method="$1"
  local url="$2"
  shift 2
  curl -sS -o /tmp/synvideo-smoke-body.txt -w "%{http_code}" -X "${method}" "$@" "${url}"
}

echo "SynVideo production smoke against WEB_ORIGIN=${WEB_ORIGIN} API_ORIGIN=${API_ORIGIN}"

web_status="$(request_status GET "${WEB_ORIGIN}/")"
if [[ "${web_status}" != "200" ]]; then
  fail "web root expected 200, got ${web_status}"
fi
pass "web serves index (${web_status})"

health_status="$(request_status GET "${API_ORIGIN}/api/v1/healthz")"
if [[ "${health_status}" != "200" ]]; then
  fail "API healthz expected 200, got ${health_status}"
fi
pass "API liveness (${health_status})"

ready_status="$(request_status GET "${API_ORIGIN}/api/v1/readyz")"
if [[ "${ready_status}" != "200" ]]; then
  fail "API readyz expected 200, got ${ready_status} (body: $(cat /tmp/synvideo-smoke-body.txt))"
fi
pass "API readiness (${ready_status})"

toolchain_status="$(request_status GET "${API_ORIGIN}/api/v1/runtime/toolchain")"
if [[ "${toolchain_status}" != "200" ]]; then
  fail "runtime toolchain expected 200, got ${toolchain_status}"
fi
if ! grep -q 'ffmpeg_version' /tmp/synvideo-smoke-body.txt || ! grep -q 'ffprobe_version' /tmp/synvideo-smoke-body.txt; then
  fail "runtime toolchain response missing ffmpeg/ffprobe versions"
fi
pass "FFmpeg/FFprobe observability"

allowed_cors="$(curl -sS -o /tmp/synvideo-smoke-body.txt -w "%{http_code}" \
  -H "Origin: ${WEB_ORIGIN}" \
  -H "Access-Control-Request-Method: GET" \
  -X OPTIONS \
  "${API_ORIGIN}/api/v1/healthz")"
if [[ "${allowed_cors}" != "204" ]]; then
  fail "allowed-origin preflight expected 204, got ${allowed_cors}"
fi
if ! curl -sSI -H "Origin: ${WEB_ORIGIN}" "${API_ORIGIN}/api/v1/healthz" | grep -qi "access-control-allow-origin: ${WEB_ORIGIN}"; then
  fail "allowed-origin response missing Access-Control-Allow-Origin"
fi
pass "CORS allows configured web origin"

disallowed_cors="$(curl -sS -o /tmp/synvideo-smoke-body.txt -w "%{http_code}" \
  -H "Origin: ${DISALLOWED_ORIGIN}" \
  -H "Access-Control-Request-Method: GET" \
  -X OPTIONS \
  "${API_ORIGIN}/api/v1/healthz")"
if [[ "${disallowed_cors}" != "403" ]]; then
  fail "disallowed-origin preflight expected 403, got ${disallowed_cors}"
fi
pass "CORS rejects disallowed origin (${disallowed_cors})"

echo "All production smoke checks passed."

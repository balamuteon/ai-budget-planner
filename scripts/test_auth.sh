#!/usr/bin/env bash
set -euo pipefail

BASE_URL="${BASE_URL:-http://localhost:8080}"
EMAIL="${AUTH_EMAIL:-user@example.com}"
PASSWORD="${AUTH_PASSWORD:-password123}"
NAME="${AUTH_NAME:-Ivan}"

json_field() {
  local key="$1"
  if command -v jq >/dev/null 2>&1; then
    jq -r "$key"
    return
  fi

  if command -v python3 >/dev/null 2>&1; then
    python3 - "$key" <<'PY'
import json
import sys

key = sys.argv[1].lstrip(".")
try:
    data = json.load(sys.stdin)
    for part in key.split("."):
        data = data[part]
    if data is None:
        sys.exit(2)
    print(data)
except Exception:
    sys.exit(2)
PY
    return
  fi

  echo "jq or python3 is required to parse JSON" >&2
  exit 2
}

request() {
  local method="$1"
  local url="$2"
  local payload="${3:-}"

  if [[ -n "$payload" ]]; then
    curl -s -w '\n%{http_code}' -X "$method" \
      -H "Content-Type: application/json" \
      -d "$payload" \
      "$url"
  else
    curl -s -w '\n%{http_code}' -X "$method" "$url"
  fi
}

payload=$(printf '{"email":"%s","password":"%s","name":"%s"}' "$EMAIL" "$PASSWORD" "$NAME")

register_resp=$(request POST "$BASE_URL/api/v1/auth/register" "$payload")
register_status=$(echo "$register_resp" | tail -n1)
register_body=$(echo "$register_resp" | sed '$d')

case "$register_status" in
  201)
    auth_body="$register_body"
    echo "registered: $EMAIL"
    ;;
  409)
    login_payload=$(printf '{"email":"%s","password":"%s"}' "$EMAIL" "$PASSWORD")
    login_resp=$(request POST "$BASE_URL/api/v1/auth/login" "$login_payload")
    login_status=$(echo "$login_resp" | tail -n1)
    login_body=$(echo "$login_resp" | sed '$d')
    if [[ "$login_status" != "200" ]]; then
      echo "login failed (status $login_status): $login_body" >&2
      exit 1
    fi
    auth_body="$login_body"
    echo "logged in: $EMAIL"
    ;;
  *)
    echo "register failed (status $register_status): $register_body" >&2
    exit 1
    ;;
 esac

access_token=$(echo "$auth_body" | json_field .access_token)
refresh_token=$(echo "$auth_body" | json_field .refresh_token)

if [[ -z "$access_token" || "$access_token" == "null" ]]; then
  echo "missing access_token" >&2
  exit 1
fi

me_resp=$(curl -s -w '\n%{http_code}' \
  -H "Authorization: Bearer $access_token" \
  "$BASE_URL/api/v1/auth/me")
me_status=$(echo "$me_resp" | tail -n1)
me_body=$(echo "$me_resp" | sed '$d')

if [[ "$me_status" != "200" ]]; then
  echo "me failed (status $me_status): $me_body" >&2
  exit 1
fi

me_email=$(echo "$me_body" | json_field .user.email)

cat <<SUMMARY
ok
- email: $me_email
- access_token: ${access_token:0:16}...
- refresh_token: ${refresh_token:0:16}...
SUMMARY

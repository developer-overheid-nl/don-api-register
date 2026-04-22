#!/usr/bin/env bash
set -euo pipefail

SOURCE_BASE_URL="${SOURCE_BASE_URL:-https://api.developer.overheid.nl/api-register}"
TARGET_BASE_URL="${TARGET_BASE_URL:-http://localhost:1337}"
PER_PAGE="${PER_PAGE:-100}"
SLEEP_SECONDS="${SLEEP_SECONDS:-1}"
OUT="${OUT:-sync-errors.json}"

SOURCE_API_KEY="${SOURCE_API_KEY:-}"
SOURCE_TOKEN="${SOURCE_TOKEN:-}"
TARGET_API_KEY="${TARGET_API_KEY:-}"
TARGET_TOKEN="${TARGET_TOKEN:-}"
declare -a SOURCE_AUTH_ARGS=()
declare -a TARGET_AUTH_ARGS=()

if [[ $# -gt 0 ]]; then
  TARGET_BASE_URL="$1"
fi

SOURCE_BASE_URL="${SOURCE_BASE_URL%/}"
TARGET_BASE_URL="${TARGET_BASE_URL%/}"

org_success=0
org_failed=0
api_success=0
api_failed=0

require_cmd() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "Vereiste command ontbreekt: $1" >&2
    exit 1
  fi
}

require_cmd curl
require_cmd jq

echo '[]' > "$OUT"

build_source_auth_args() {
  SOURCE_AUTH_ARGS=()
  if [[ -n "$SOURCE_TOKEN" ]]; then
    SOURCE_AUTH_ARGS+=(-H "Authorization: Bearer ${SOURCE_TOKEN}")
  fi
  if [[ -n "$SOURCE_API_KEY" ]]; then
    SOURCE_AUTH_ARGS+=(-H "X-API-Key: ${SOURCE_API_KEY}")
  fi
}

build_target_auth_args() {
  TARGET_AUTH_ARGS=()
  if [[ -n "$TARGET_TOKEN" ]]; then
    TARGET_AUTH_ARGS+=(-H "Authorization: Bearer ${TARGET_TOKEN}")
  fi
  if [[ -n "$TARGET_API_KEY" ]]; then
    TARGET_AUTH_ARGS+=(-H "X-API-Key: ${TARGET_API_KEY}")
  fi
}

prompt_source_auth() {
  if [[ ! -t 0 ]]; then
    echo "Bron-auth ontbreekt of is ongeldig. Zet SOURCE_API_KEY en eventueel SOURCE_TOKEN." >&2
    exit 1
  fi

  echo
  read -r -s -p "Bron X-API-Key: " SOURCE_API_KEY
  echo
  read -r -s -p "Bron Bearer token (optioneel, enter om over te slaan): " SOURCE_TOKEN
  echo
}

prompt_target_auth() {
  if [[ ! -t 0 ]]; then
    echo "Doel-auth ontbreekt of is ongeldig. Zet TARGET_API_KEY en/of TARGET_TOKEN." >&2
    exit 1
  fi

  echo
  read -r -s -p "Doel X-API-Key (optioneel, enter om over te slaan): " TARGET_API_KEY
  echo
  read -r -s -p "Doel Bearer token (optioneel, enter om over te slaan): " TARGET_TOKEN
  echo
}

request_source_once() {
  local url="$1"
  local body_file="$2"
  local headers_file="$3"
  local http_code
  local -a curl_args

  build_source_auth_args
  curl_args=(
    -sS
    -D "$headers_file"
    -o "$body_file"
    -w '%{http_code}'
    "$url"
    -H 'Accept: application/json'
  )
  if ((${#SOURCE_AUTH_ARGS[@]} > 0)); then
    curl_args+=("${SOURCE_AUTH_ARGS[@]}")
  fi
  http_code=$(curl "${curl_args[@]}")

  printf '%s' "$http_code"
}

request_target_once() {
  local url="$1"
  local payload="$2"
  local body_file="$3"
  local http_code
  local -a curl_args

  build_target_auth_args
  curl_args=(
    -sS
    -o "$body_file"
    -w '%{http_code}'
    -X POST "$url"
    -H 'Accept: application/json'
    -H 'Content-Type: application/json'
    -d "$payload"
  )
  if ((${#TARGET_AUTH_ARGS[@]} > 0)); then
    curl_args+=("${TARGET_AUTH_ARGS[@]}")
  fi
  http_code=$(curl "${curl_args[@]}")

  printf '%s' "$http_code"
}

append_error() {
  local phase="$1"
  local descriptor="$2"
  local request_json="$3"
  local status="$4"
  local body_file="$5"

  if jq -e . >/dev/null 2>&1 < "$body_file"; then
    jq \
      --arg phase "$phase" \
      --arg descriptor "$descriptor" \
      --argjson request "$request_json" \
      --argjson status "$status" \
      --slurpfile body "$body_file" \
      '. + [{phase:$phase, descriptor:$descriptor, request:$request, status:$status, body:$body[0]}]' \
      "$OUT" > "${OUT}.tmp"
  else
    jq \
      --arg phase "$phase" \
      --arg descriptor "$descriptor" \
      --argjson request "$request_json" \
      --arg status "$status" \
      --rawfile body "$body_file" \
      '. + [{
        phase:$phase,
        descriptor:$descriptor,
        request:$request,
        status:(if ($status | test("^[0-9]+$")) then ($status|tonumber) else $status end),
        body:$body
      }]' \
      "$OUT" > "${OUT}.tmp"
  fi

  mv "${OUT}.tmp" "$OUT"
}

extract_next_page() {
  local headers_file="$1"
  local next_link

  next_link=$(
    tr -d '\r' < "$headers_file" \
      | awk 'BEGIN{IGNORECASE=1} /^link:/ {sub(/^[^:]+:[[:space:]]*/, "", $0); print; exit}' \
      | tr ',' '\n' \
      | awk '/rel="next"/ {if (match($0, /<[^>]+>/)) print substr($0, RSTART + 1, RLENGTH - 2)}'
  )

  if [[ -z "$next_link" ]]; then
    return 0
  fi

  printf '%s\n' "$next_link" | sed -nE 's/.*[?&]page=([0-9]+).*/\1/p'
}

fetch_source_or_die() {
  local url="$1"
  local body_file="$2"
  local headers_file="$3"
  local context="$4"
  local http_code

  http_code="$(request_source_once "$url" "$body_file" "$headers_file")"
  if [[ "$http_code" == "401" ]]; then
    echo
    echo "401 ontvangen van bron voor ${context}."
    prompt_source_auth
    http_code="$(request_source_once "$url" "$body_file" "$headers_file")"
    if [[ "$http_code" == "401" ]]; then
      echo "Nog steeds 401 van de bron na opnieuw invoeren van credentials. Stop." >&2
      exit 1
    fi
  fi

  if [[ "$http_code" != "200" ]]; then
    echo "GET ${url} gaf status ${http_code}. Stop." >&2
    cat "$body_file" >&2
    exit 1
  fi
}

post_target() {
  local path="$1"
  local phase="$2"
  local descriptor="$3"
  local payload="$4"
  local http_code
  local body_file

  body_file="$(mktemp)"
  http_code="$(request_target_once "${TARGET_BASE_URL}${path}" "$payload" "$body_file")"

  if [[ "$http_code" == "401" ]]; then
    echo
    echo "401 ontvangen van doel voor ${descriptor}."
    prompt_target_auth
    http_code="$(request_target_once "${TARGET_BASE_URL}${path}" "$payload" "$body_file")"
    if [[ "$http_code" == "401" ]]; then
      echo "Nog steeds 401 van het doel na opnieuw invoeren van credentials. Stop." >&2
      rm -f "$body_file"
      exit 1
    fi
  fi

  if [[ "$http_code" == "201" ]]; then
    if [[ "$phase" == "organisation" ]]; then
      org_success=$((org_success + 1))
    else
      api_success=$((api_success + 1))
    fi
  else
    append_error "$phase" "$descriptor" "$payload" "$http_code" "$body_file"
    if [[ "$phase" == "organisation" ]]; then
      org_failed=$((org_failed + 1))
    else
      api_failed=$((api_failed + 1))
    fi
  fi

  echo "POST [${phase}] ${descriptor} -> ${http_code}"
  rm -f "$body_file"

  if [[ "$SLEEP_SECONDS" != "0" ]]; then
    sleep "$SLEEP_SECONDS"
  fi
}

build_api_payload() {
  local item="$1"

  jq -c '
    (.contact // {}) as $contact
    | ($contact.email // "") as $email
    | ($contact.name // "") as $name
    | {
        oasUrl: .oasUrl,
        organisationUri: .organisation.uri,
        contact: {
          name: (
            if $name != "" or $email == "" then
              $name
            else
              (($email | split("@")[1] // "") | split(".")[0])
            end
          ),
          url: ($contact.url // ""),
          email: $email
        }
      }' <<<"$item"
}

sync_organisations() {
  local body_file
  local headers_file

  body_file="$(mktemp)"
  headers_file="$(mktemp)"

  fetch_source_or_die "${SOURCE_BASE_URL}/v1/organisations" "$body_file" "$headers_file" "organisations"
  echo "Organisations opgehaald uit bron."

  while read -r item; do
    local payload
    local uri

    payload="$(jq -c '{uri, label}' <<<"$item")"
    uri="$(jq -r '.uri' <<<"$item")"
    post_target "/v1/organisations" "organisation" "$uri" "$payload"
  done < <(jq -c '.[]' "$body_file")

  rm -f "$body_file" "$headers_file"
}

sync_apis() {
  local page=1

  while :; do
    local body_file
    local headers_file
    local next_page
    local page_url

    body_file="$(mktemp)"
    headers_file="$(mktemp)"
    page_url="${SOURCE_BASE_URL}/v1/apis?page=${page}&perPage=${PER_PAGE}"

    fetch_source_or_die "$page_url" "$body_file" "$headers_file" "apis pagina ${page}"
    echo "APIs opgehaald uit bron, pagina ${page}."

    while read -r item; do
      local oas_url
      local organisation_uri
      local payload
      local request_json
      local error_body

      oas_url="$(jq -r '.oasUrl // empty' <<<"$item")"
      organisation_uri="$(jq -r '.organisation.uri // empty' <<<"$item")"

      if [[ -z "$oas_url" || -z "$organisation_uri" ]]; then
        error_body="$(mktemp)"
        printf '%s\n' 'ontbrekende oasUrl of organisation.uri in bronresponse' > "$error_body"
        request_json="$(jq -c '.' <<<"$item")"
        append_error "api-build" "${oas_url:-missing-oasUrl}" "$request_json" "0" "$error_body"
        api_failed=$((api_failed + 1))
        rm -f "$error_body"
        echo "SKIP [api] ontbrekende verplichte velden"
        continue
      fi

      payload="$(build_api_payload "$item")"
      post_target "/v1/apis" "api" "$oas_url" "$payload"
    done < <(jq -c '.[]' "$body_file")

    next_page="$(extract_next_page "$headers_file")"
    rm -f "$body_file" "$headers_file"

    if [[ -z "$next_page" || "$next_page" -le "$page" ]]; then
      break
    fi
    page="$next_page"
  done
}

if [[ -z "$SOURCE_API_KEY" ]]; then
  prompt_source_auth
fi

echo "Bron: ${SOURCE_BASE_URL}"
echo "Doel: ${TARGET_BASE_URL}"
echo "Errors worden opgeslagen in: ${OUT}"

sync_organisations
sync_apis

echo
echo "Klaar."
echo "Organisations: succes=${org_success}, fouten=${org_failed}"
echo "APIs: succes=${api_success}, fouten=${api_failed}"
echo "Foutenbestand: ${OUT}"

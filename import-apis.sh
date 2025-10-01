#!/usr/bin/env bash
set -euo pipefail

JSON_FILE="bla.json"
OUT="responses.json"
BASE_URL="https://api.don.apps.digilab.network/api-register/v1/apis"

# Optioneel: init token uit env (TOKEN), anders pas vragen bij 401
TOKEN="${TOKEN:-}"

echo '[]' > "$OUT"

request_once() {
  local payload="$1"
  local tmp="$2"
  local code
  code=$(curl -sS -o "$tmp" -w '%{http_code}' \
    -X POST "$BASE_URL" \
    -H 'Content-Type: application/json' \
    -H "Authorization: Bearer ${TOKEN}" \
    -d "$payload")
  printf '%s' "$code"
}

jq -c '.[]' "$JSON_FILE" | while read -r item; do
  href=$(echo "$item" | jq -r '.href')
  organisationUri=$(echo "$item" | jq -r '.organisationUri')

  # contact.name auto-afleiden als alleen email aanwezig is
  email=$(echo "$item" | jq -r '.contact.email // empty')
  name=$(echo "$item" | jq -r '.contact.name // empty')

  if [[ -n "$email" && -z "$name" ]]; then
    # domein tussen @ en eerste .
    domain=$(echo "$email" | awk -F'[@.]' '{print $2}')
    contact=$(echo "$item" | jq --arg n "$domain" '.contact + {name:$n}')
  else
    contact=$(echo "$item" | jq -c '.contact')
  fi

  payload=$(jq -nc --arg o "$href" --arg org "$organisationUri" --argjson c "$contact" \
    '{oasUrl:$o, organisationUri:$org, contact:$c}')

  tmp="$(mktemp)"

  # Eerste poging
  http_code="$(request_once "$payload" "$tmp")"

  # Als 401: prompt om nieuwe token en retry ditzelfde item
  if [[ "$http_code" -eq 401 ]]; then
    echo
    echo "⚠️  401 Unauthorized ontvangen voor: $href"
    read -r -s -p "Voer nieuwe Bearer token in: " TOKEN
    echo
    # Retry (overschrijft $tmp)
    http_code="$(request_once "$payload" "$tmp")"
    # Als nog steeds 401 → hard stoppen, want je token klopt niet
    if [[ "$http_code" -eq 401 ]]; then
      echo "❌ Nog steeds 401 na het invoeren van een nieuwe token. Stop."
      rm -f "$tmp"
      exit 1
    fi
  fi

  # Alleen loggen/bewaren als geen 201
  if [[ "$http_code" -ne 201 ]]; then
    if jq -e . > /dev/null 2>&1 < "$tmp"; then
      jq --arg o "$href" --arg org "$organisationUri" --argjson status "$http_code" --slurpfile body "$tmp" \
         '. + [ { request:{ oasUrl:$o, organisationUri:$org }, status:$status, body:$body[0] } ]' \
         "$OUT" > "$OUT.tmp" && mv "$OUT.tmp" "$OUT"
    else
      jq --arg o "$href" --arg org "$organisationUri" --arg status "$http_code" --rawfile body "$tmp" \
         '. + [ { request:{ oasUrl:$o, organisationUri:$org }, status:($status|tonumber), body:$body } ]' \
         "$OUT" > "$OUT.tmp" && mv "$OUT.tmp" "$OUT"
    fi
  fi

  rm -f "$tmp"
  echo "POST $href -> $http_code"
  sleep 3
done

echo "Klaar. Responses (alleen fouten) staan in: $OUT"
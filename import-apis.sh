#!/usr/bin/env bash
set -euo pipefail

JSON_FILE="testApis.json"
OUT="responses.json"
BASE_URL="http://localhost:1337/v1/apis"

# Begin met een lege array
echo '[]' > "$OUT"

# Loop over alle items in testApis.json
jq -c '.[]' "$JSON_FILE" | while read -r item; do
  href=$(echo "$item" | jq -r '.href')
  organisationUri=$(echo "$item" | jq -r '.organisationUri')

  # Bouw payload veilig met jq (i.p.v. handmatig quoten)
  payload=$(jq -nc --arg o "$href" --arg org "$organisationUri" --arg email "developer.overheid@geonovum.nl" \
    '{oasUrl:$o, organisationUri:$org, contact:{email:$email}}')

  # Voer request uit; vang body en statuscode
  tmp="$(mktemp)"
  http_code=$(curl -sS -o "$tmp" -w '%{http_code}' \
    -X POST "$BASE_URL" \
    -H 'Content-Type: application/json' \
    -d "$payload")

  # Append entry aan het output-bestand
  if jq -e . > /dev/null 2>&1 < "$tmp"; then
    # Body is geldige JSON
    jq --arg o "$href" --arg org "$organisationUri" --argjson status "$http_code" --slurpfile body "$tmp" \
       '. + [ { request:{ oasUrl:$o, organisationUri:$org }, status:$status, body:$body[0] } ]' \
       "$OUT" > "$OUT.tmp" && mv "$OUT.tmp" "$OUT"
  else
    # Body is geen geldige JSON; sla op als string
    jq --arg o "$href" --arg org "$organisationUri" --arg status "$http_code" --rawfile body "$tmp" \
       '. + [ { request:{ oasUrl:$o, organisationUri:$org }, status:($status|tonumber), body:$body } ]' \
       "$OUT" > "$OUT.tmp" && mv "$OUT.tmp" "$OUT"
  fi

  rm -f "$tmp"
  echo "POST $href -> $http_code"
done

echo "Klaar. Responses staan in: $OUT"

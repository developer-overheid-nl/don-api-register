#!/bin/bash

JSON_FILE="testApis.json"

jq -c '.[]' "$JSON_FILE" | while read -r item; do
  href=$(echo "$item" | jq -r '.href')
  organisationUri=$(echo "$item" | jq -r '.organisationUri')

  echo "{\"oasUrl\": \"$href\", \"organisationUri\": \"$organisationUri\"},"

  curl -s -X POST "http://localhost:1337/v1/apis" \
    -H "Content-Type: application/json" \
    -H "Authorization: Bearer eyJhbGciOiJSUzI1NiIsInR5cCIgOiAiSldUIiwia2lkIiA6ICJjd2NaOVNuVEFLNkNwSkdCdXc2ZEhxYWh1bzNPbk40YlBMU1BEc0drblprIn0.eyJleHAiOjE3NTU2ODg4MDIsImlhdCI6MTc1NTY4ODUwMiwianRpIjoib25ydGNjOjU2OGNjOGQxLWRmNGYtMzRiMi04Y2FiLWYyZDA4Zjk4N2M5MCIsImlzcyI6Imh0dHBzOi8vYXV0aC5kb24uYXBwcy5kaWdpbGFiLm5ldHdvcmsvcmVhbG1zL2RvbiIsImF1ZCI6ImFjY291bnQiLCJzdWIiOiJlZGI1ZTdhOC1mMDAzLTQwZTktYWFjYi03YTY4Y2Y2ODA2OTIiLCJ0eXAiOiJCZWFyZXIiLCJhenAiOiJhbXN0ZXJkYW0tdHJ1c3RlZC1hcGkiLCJzaWQiOiI5Y2ZjMjc4Mi04ZWQ5LTQ5NTEtODgwNi1kNjczMDIwMmY5ZjYiLCJhY3IiOiIxIiwicmVhbG1fYWNjZXNzIjp7InJvbGVzIjpbImFwaXM6cmVhZCIsIm9mZmxpbmVfYWNjZXNzIiwidW1hX2F1dGhvcml6YXRpb24iLCJkZWZhdWx0LXJvbGVzLWRvbiJdfSwicmVzb3VyY2VfYWNjZXNzIjp7ImFtc3RlcmRhbS10cnVzdGVkLWFwaSI6eyJyb2xlcyI6WyJ1bWFfcHJvdGVjdGlvbiJdfSwiYWNjb3VudCI6eyJyb2xlcyI6WyJtYW5hZ2UtYWNjb3VudCIsIm1hbmFnZS1hY2NvdW50LWxpbmtzIiwidmlldy1wcm9maWxlIl19fSwic2NvcGUiOiJwcm9maWxlIHJhdGUtbGltaXQgZW1haWwgYXBpczp3cml0ZSIsImVtYWlsX3ZlcmlmaWVkIjpmYWxzZSwicmF0ZV9saW1pdCI6IiR7Y2xpZW50LmF0dHJpYnV0ZXMucmF0ZS1saW1pdH0iLCJjbGllbnRIb3N0IjoiMTAuNi4xMy44OSIsInByZWZlcnJlZF91c2VybmFtZSI6InNlcnZpY2UtYWNjb3VudC1hbXN0ZXJkYW0tdHJ1c3RlZC1hcGkiLCJjbGllbnRBZGRyZXNzIjoiMTAuNi4xMy44OSIsImNsaWVudF9pZCI6ImFtc3RlcmRhbS10cnVzdGVkLWFwaSJ9.FUrc4U8wel691D5vMbjPpS9sS6MP0RJudP3opYrh5pLe1ZvShfe8YQ4Hkk7frNQ6Q8AAaH3OVVpA3nK7TXQA186Rw67pLDXDT2Leq2I2BPj2_joqrZzRxMWed5ezbf-VHfqt0uTISatpgO1UacmFDuXOLolT6XXbsc_Eh7WpKLJFMT3p6arPCviDjVmkWsOSppPLv7jyzZOfzKLv2oUiOyvG6yYxO_gbKlzsGoPDGcxZgComKfAQRwcSvW7so-xu_Z9PAvMSTaWADOhef_VM0BNX7CKAq9TLygxwXvw5cr6R5Bk-J4mHdicqHvjPc2j93GrQzSGd9w8y1rP54In4KQ" \
    -d "{\"oasUrl\": \"$href\", \"organisationUri\": \"$organisationUri\", \"contact\": {\"email\":\"developer.overheid@geonovum.nl\"}}"

  echo
done

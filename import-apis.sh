#!/bin/bash

JSON_FILE="testApis.json"
RESULT_FILE="importResult.json"
TMP_FILE=$(mktemp)

echo "[" > "$TMP_FILE"

first=1

jq -c '.[]' "$JSON_FILE" | while read -r item; do
  href=$(echo "$item" | jq -r '.href')
  organisationUri=$(echo "$item" | jq -r '.organisationUri')

  echo "{\"oasUrl\": \"$href\", \"organisationUri\": \"$organisationUri\"},"

  response=$(curl -s -X POST "http://localhost:1338/v1/apis" \
    -H "Content-Type: application/json" \
    -H "Authorization: Bearer eyJhbGciOiJSUzI1NiIsInR5cCIgOiAiSldUIiwia2lkIiA6ICJjd2NaOVNuVEFLNkNwSkdCdXc2ZEhxYWh1bzNPbk40YlBMU1BEc0drblprIn0.eyJleHAiOjE3NTM3NzAzNDksImlhdCI6MTc1Mzc3MDA0OSwianRpIjoidHJydGNjOjYzOThlNzY3LTRkMDUtZmQ4MC04NzNmLTA5NzZhYTQ5OGIzMyIsImlzcyI6Imh0dHBzOi8vYXV0aC5kb24uYXBwcy5kaWdpbGFiLm5ldHdvcmsvcmVhbG1zL2RvbiIsImF1ZCI6WyJyZWFsbS1tYW5hZ2VtZW50IiwiYWNjb3VudCJdLCJzdWIiOiI3YTYyYzE3Ni1kMDYzLTQ0MmQtYWRhZS01NzcxZjhiN2E0MTUiLCJ0eXAiOiJCZWFyZXIiLCJhenAiOiJkb24tYWRtaW4tY2xpZW50IiwiYWNyIjoiMSIsImFsbG93ZWQtb3JpZ2lucyI6WyIvKiJdLCJyZWFsbV9hY2Nlc3MiOnsicm9sZXMiOlsib2ZmbGluZV9hY2Nlc3MiLCJ1bWFfYXV0aG9yaXphdGlvbiIsImRlZmF1bHQtcm9sZXMtZG9uIl19LCJyZXNvdXJjZV9hY2Nlc3MiOnsicmVhbG0tbWFuYWdlbWVudCI6eyJyb2xlcyI6WyJjcmVhdGUtY2xpZW50IiwibWFuYWdlLWNsaWVudHMiXX0sImRvbi1hZG1pbi1jbGllbnQiOnsicm9sZXMiOlsidW1hX3Byb3RlY3Rpb24iXX0sImFjY291bnQiOnsicm9sZXMiOlsibWFuYWdlLWFjY291bnQiLCJtYW5hZ2UtYWNjb3VudC1saW5rcyIsInZpZXctcHJvZmlsZSJdfX0sInNjb3BlIjoicHJvZmlsZSBlbWFpbCBhcGlzOndyaXRlIGFwaXM6cmVhZCIsImVtYWlsX3ZlcmlmaWVkIjpmYWxzZSwiY2xpZW50SG9zdCI6IjEwLjYuMTIuMTM4IiwicHJlZmVycmVkX3VzZXJuYW1lIjoic2VydmljZS1hY2NvdW50LWRvbi1hZG1pbi1jbGllbnQiLCJjbGllbnRBZGRyZXNzIjoiMTAuNi4xMi4xMzgiLCJjbGllbnRfaWQiOiJkb24tYWRtaW4tY2xpZW50In0.HreKnvxeZafcL557U_MC_TFarY8G50KwhvnqVEEUtktw7PqqvoQl24HAFmEAXX61twvmdZRMglnSJTAzuWjScYClFHWbF8F3G-nWFNZ3hSmvts_r78pI1UbE2A7dcjdYbxaUn_2kcxhGDkIKUWo5r5pgbQXTf1n2itVNX6bkFW2C7_A6fuA5O_SI8Bz2Bkf9iBaNNNpKmVfLmne76kNd7DwsRC7c3-njWObF-j8sc52a392RNmGpxWRLGQHimdegQAxl0jgPneQuTRJi3HhL3BJWaSdBcZlB34h2wV1mSSnkk5DZ81BBRpr5PjPNEu4dX7sBYBiVvmhJ4BHEEBp2YA" \
    -d "{\"oasUrl\": \"$href\", \"organisationUri\": \"$organisationUri\"}")

  if [ $first -eq 1 ]; then
    first=0
  else
    echo "," >> "$TMP_FILE"
  fi

  echo "$response" >> "$TMP_FILE"
done

echo "]" >> "$TMP_FILE"

mv "$TMP_FILE" "$RESULT_FILE"
echo "Resultaat opgeslagen in $RESULT_FILE"

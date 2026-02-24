# API registratie

API van het API register (apis.developer.overheid.nl)

## Overview

- API version: 1.0.0
- Build date: 2025-04-02
- Generator version: 7.7.0

## Lokaal draaien

1. Start de afhankelijkheden:

   ```bash
   docker compose up -d
   ```

2. Start de server:

   ```bash
   go run cmd/main.go
   ```

   De API luistert standaard op poort **1337**.

## Typesense integratie

Nieuwe APIs worden na een succesvolle POST ook naar Typesense gestuurd, zodat ze vindbaar zijn in de zoekfunctie. Stel hiervoor de volgende omgevingsvariabelen in:

- `TYPESENSE_ENDPOINT`: basis-URL van de Typesense cluster (bijv. `https://search.don.apps.digilab.network`).
- `TYPESENSE_API_KEY`: API key met schrijfrechten.
- `TYPESENSE_COLLECTION`: naam van de collectie (standaard `api_register`).
- `TYPESENSE_DETAIL_BASE_URL`: basis-URL voor detailpagina's in de frontend (bijv. `https://api-register.don.apps.digilab.network/apis`).
- `ENABLE_TYPESENSE`: zet op `false` om Typesense indexing volledig uit te schakelen (standaard `true`).

## Dagelijkse OAS-refresh

Bij het opstarten van de server wordt automatisch een aparte service gestart die direct een refresh-run uitvoert. Daarna draait de job iedere ochtend om **07:00** en haalt alle geregistreerde APIs opnieuw op. Zodra de OAS is gewijzigd, volgen exact dezelfde stappen als bij een POST: validatie, regeneratie van artifacts (Bruno, Postman en OAS-bestanden) en het opruimen van verouderde bestanden. Er zijn geen extra omgevingsvariabelen nodig.

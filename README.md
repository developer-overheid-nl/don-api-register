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


Wil je bestaande APIs eenmalig naar Typesense sturen? Gebruik dan:

```bash
go run ./cmd/tools/publish_typesense
```

Zorg dat de database- en Typesense-variabelen gezet zijn voordat je deze helper draait.

## Dagelijkse OAS-refresh

Bij het opstarten van de server wordt automatisch een aparte service gestart die iedere ochtend om **07:00** alle geregistreerde APIs opnieuw ophaalt. Zodra de OAS is gewijzigd, volgen exact dezelfde stappen als bij een POST: validatie, regeneratie van artifacts (Bruno, Postman en OAS-bestanden) en het opruimen van verouderde bestanden. Er zijn geen extra omgevingsvariabelen nodig; de job draait iedere 24 uur op het ingestelde tijdstip.

## Database en pgAdmin

De applicatie gebruikt PostgreSQL. De docker-compose start automatisch een Postgres container met bovenstaande credentials.

Voor het beheren van de database kun je optioneel [pgAdmin](https://www.pgadmin.org/) gebruiken:

```bash
docker run --rm -p 5050:80 \
  -e PGADMIN_DEFAULT_EMAIL=admin@example.com \
  -e PGADMIN_DEFAULT_PASSWORD=admin \
  dpage/pgadmin4
```

Navigeer naar `http://localhost:5050`, voeg een nieuwe server toe en gebruik de waarden:

- Host: `localhost`
- Port: `5432`
- Username: `don`
- Password: `don`
- Database: `don_v1`

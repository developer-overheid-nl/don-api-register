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

## Changelog (Changie)

Voor user-facing wijzigingen (fix/feature/breaking) verwachten we per PR een Changie-fragment in `.changes/unreleased`.

Eenmalig installeren:

```bash
go install github.com/miniscruff/changie@latest
```

Fragment aanmaken:

```bash
changie new
```

Normaal is een fragment niet nodig voor interne refactors zonder zichtbaar effect, docs-only wijzigingen en CI-only tweaks.

Bij een release kun je de fragments bundelen in `CHANGELOG.md`:

```bash
changie batch <version>
```

Dit gebeurt ook automatisch bij elke merge naar `main` via GitHub Actions:
`changie batch auto` en daarna `changie merge`, waarna automatisch een PR met de changelog-updates wordt aangemaakt.

## Deployen

De deployment van deze site verloopt via GitHub Actions en een aparte infra
repository.

### Benodigde variabelen en secrets

- Organization variable `INFRA_REPO`, bijvoorbeeld
  `developer-overheid-nl/don-infra`.
- Repository variable `KUSTOMIZE_PATH`, met als basispad bijvoorbeeld
  `apps/api/overlays/`.
- Secrets `RELEASE_PROCES_APP_ID` en `RELEASE_PROCES_APP_PRIVATE_KEY` voor het
  aanpassen van de infra repository.

### Deploy naar test

De testdeploy draait via
`.github/workflows/deploy-test.yml`.

- De workflow draait op pushes naar branches behalve `main`.
- Alleen commits met `[deploy-test]` in de commit message worden echt gedeployed.
- Er wordt een image gebouwd en gepusht naar
  `ghcr.io/<owner>/<repo>` met tags `test` en de commit SHA.
- Daarna wordt in `INFRA_REPO` het bestand
  `${KUSTOMIZE_PATH}test/kustomization.yaml` bijgewerkt naar de nieuwe image
  tag en direct gecommit.

Voorbeeld commit message:

```text
feat: pas content aan [deploy-test]
```

### Deploy naar productie

De productiedeploy draait via
`.github/workflows/deploy-prod.yml`.

- De workflow draait bij een push naar `main`.
- Er wordt in `INFRA_REPO` een release branch aangemaakt.
- In `${KUSTOMIZE_PATH}prod/kustomization.yaml` wordt de image tag bijgewerkt
  naar de commit SHA van deze repository.
- Daarna wordt automatisch een pull request in de infra repository geopend.
- De productie-uitrol gebeurt door die pull request te mergen.

### Contributies en deploy

Een contribution of pull request leidt niet automatisch tot een deployment.

- Een pull request triggert wel CI, waaronder de build en JSON-validatie.
- De build in `.github/workflows/go-ci.yml` bouwt voor een pull request een
  Docker image als controle, maar pusht dat image niet naar GHCR en past de
  infra repository niet aan.
- Er is dus geen automatische preview-omgeving per pull request.
- Een testdeploy gebeurt pas na een push naar een branch in deze repository met
  `[deploy-test]` in de commit message.
- Die testdeploy gebruikt repository- en organization-variables en secrets om
  ook `INFRA_REPO` aan te passen. Daardoor is dit pad in de praktijk bedoeld
  voor maintainers of contributors met een branch in deze repository.
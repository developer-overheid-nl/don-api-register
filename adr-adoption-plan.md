# Implementatieplan: ADR Adoptie Statistieken API

## 1. Context

### 1.1 Doel
API endpoints voor het ontsluiten van ADR adoptiestatistieken in een dashboard. Het dashboard toont per ADR versie de adoptiegraad per regel over tijd, met diverse filtermogelijkheden.

### 1.2 Database Schema
```
lint_results
├── id (PK)
├── api_id (FK naar apis tabel)
├── successes (boolean)
├── failures (bigint)
├── warnings (bigint)
├── created_at (timestamptz)
└── adr_version (text)

lint_messages
├── id (PK)
├── lint_result_id (FK)
├── line (bigint)
├── column (bigint)
├── severity (text) - "error" | "warning"
├── code (text) - bijv. "API-01", "API-05"
└── created_at (timestamptz)

lint_message_infos
├── id (PK)
├── lint_message_id (FK)
├── message (text)
└── path (text)
```

### 1.3 Belangrijke Data-Eigenschap
Lint resultaten worden **alleen aangemaakt bij API wijzigingen**, niet periodiek. Voor tijdreeksen moet daarom de **laatst bekende status** per API worden gebruikt (point-in-time queries).

### 1.4 Tabel- en Kolomnamen
GORM genereert tabelnamen automatisch (snake_case, meervoud) zonder `TableName()` overrides. De `search_path` is ingesteld op het schema, dus geen prefix nodig in queries.

| Go field | DB kolom | Tabel |
|----------|----------|-------|
| `LintResult.Successes` | `successes` | `lint_results` |
| `LintResult.AdrVersion` | `adr_version` | `lint_results` |
| `Organisation.Label` | `label` | `organisations` |
| `Api.OrganisationID` | `organisation_id` | `apis` |

---

## 2. API Specificatie

### 2.1 Endpoints Overzicht

| Endpoint | Methode | Doel |
|----------|---------|------|
| `/v1/statistics/summary` | GET | KPI's: totalen en overall adoptiegraad |
| `/v1/statistics/rules` | GET | Adoptie per ADR regel (snapshot) |
| `/v1/statistics/timeline` | GET | Adoptie over tijd per regel (voor grafieken) |
| `/v1/statistics/apis` | GET | Drill-down naar individuele API's |

### 2.2 Gemeenschappelijke Query Parameters

| Parameter | Type | Required | Beschrijving |
|-----------|------|----------|--------------|
| `adrVersion` | string | Ja | ADR versie (bijv. "1.0.0") |
| `startDate` | string (date) | Ja | Begin periode, ISO 8601 formaat (bijv. "2024-01-01") |
| `endDate` | string (date) | Ja | Eind periode, ISO 8601 formaat |
| `apiIds` | string | Nee | Comma-separated lijst van API ID's |
| `organisation` | string | Nee | Filter op organisatie URI |

---

## 3. Endpoint Specificaties

### 3.1 GET /v1/statistics/summary

**Beschrijving**: Geeft algemene KPI's terug voor de geselecteerde periode.

**Extra Parameters**: Geen

**Response Model**:
```json
{
  "adrVersion": "1.0.0",
  "period": {
    "start": "2024-01-01",
    "end": "2024-06-30"
  },
  "totalApis": 150,
  "compliantApis": 45,
  "overallAdoptionRate": 30.0,
  "totalLintRuns": 450
}
```

**Velden**:
- `totalApis`: Aantal unieke API's met minimaal één lint result in of vóór de periode
- `compliantApis`: Aantal API's waarvan de laatst bekende status `successes = true` is
- `overallAdoptionRate`: `(compliantApis / totalApis) * 100`, afgerond op 1 decimaal
- `totalLintRuns`: Totaal aantal lint runs binnen de periode

---

### 3.2 GET /v1/statistics/rules

**Beschrijving**: Geeft adoptiegraad per ADR regel terug (snapshot op `endDate`).

**Extra Parameters**:

| Parameter | Type | Required | Beschrijving |
|-----------|------|----------|--------------|
| `ruleCodes` | string | Nee | Comma-separated regel codes (bijv. "API-01,API-05") |
| `severity` | string | Nee | Filter op severity: "error" of "warning" |

**Response Model**:
```json
{
  "adrVersion": "1.0.0",
  "period": {
    "start": "2024-01-01",
    "end": "2024-06-30"
  },
  "totalApis": 150,
  "rules": [
    {
      "code": "API-01",
      "severity": "error",
      "violatingApis": 12,
      "compliantApis": 138,
      "adoptionRate": 92.0
    },
    {
      "code": "API-05",
      "severity": "error",
      "violatingApis": 45,
      "compliantApis": 105,
      "adoptionRate": 70.0
    }
  ]
}
```

**Berekening per regel**:
- Neem per API de laatst bekende lint result (op `endDate`)
- Tel voor die lint result of er een message is met `code = <regel>`
- `violatingApis`: Aantal API's met minimaal één violation voor deze regel
- `compliantApis`: `totalApis - violatingApis`
- `adoptionRate`: `(compliantApis / totalApis) * 100`

---

### 3.3 GET /v1/statistics/timeline

**Beschrijving**: Geeft adoptie over tijd terug per regel voor grafieken. Altijd per regel: zonder `ruleCodes` worden alle regels teruggegeven, met `ruleCodes` alleen de opgegeven regels.

**Extra Parameters**:

| Parameter | Type | Required | Beschrijving |
|-----------|------|----------|--------------|
| `granularity` | string | Nee | "day", "week", of "month" (default: "month") |
| `ruleCodes` | string | Nee | Comma-separated regel codes (zonder = alle regels) |

**Response Model**:
```json
{
  "adrVersion": "1.0.0",
  "granularity": "month",
  "series": [
    {
      "type": "rule",
      "ruleCode": "API-01",
      "dataPoints": [
        {
          "period": "2024-01",
          "totalApis": 120,
          "compliantApis": 100,
          "adoptionRate": 83.3
        },
        {
          "period": "2024-02",
          "totalApis": 125,
          "compliantApis": 110,
          "adoptionRate": 88.0
        }
      ]
    },
    {
      "type": "rule",
      "ruleCode": "API-05",
      "dataPoints": [
        {
          "period": "2024-01",
          "totalApis": 120,
          "compliantApis": 60,
          "adoptionRate": 50.0
        },
        {
          "period": "2024-02",
          "totalApis": 125,
          "compliantApis": 70,
          "adoptionRate": 56.0
        }
      ]
    }
  ]
}
```

**Period formaat per granularity**:
- `day`: "2024-01-15"
- `week`: "2024-W03" (ISO week)
- `month`: "2024-01"

---

### 3.4 GET /v1/statistics/apis

**Beschrijving**: Lijst van API's met hun compliance status (voor drill-down in dashboard).

**Extra Parameters**:

| Parameter | Type | Required | Beschrijving |
|-----------|------|----------|--------------|
| `compliant` | boolean | Nee | Filter op compliance status |
| `ruleCodes` | string | Nee | Filter API's die deze specifieke regels schenden |
| `page` | integer | Nee | Pagina nummer (default: 1) |
| `perPage` | integer | Nee | Items per pagina (default: 20, max: 100) |

**Response Model**:
```json
{
  "adrVersion": "1.0.0",
  "period": {
    "start": "2024-01-01",
    "end": "2024-06-30"
  },
  "apis": [
    {
      "apiId": "abc-123",
      "apiTitle": "Petstore API",
      "organisation": "Gemeente Amsterdam",
      "isCompliant": false,
      "totalViolations": 3,
      "totalWarnings": 5,
      "violatedRules": ["API-01", "API-05", "API-12"],
      "lastLintDate": "2024-06-28T14:30:00Z"
    }
  ]
}
```

**Response Headers** (bestaand patroon):
- `Total-Count`: Totaal aantal resultaten
- `Total-Pages`: Aantal pagina's
- `Per-Page`: Items per pagina
- `Current-Page`: Huidige pagina
- `Link`: RFC 5988 pagination links

---

## 4. SQL Queries

### 4.1 Point-in-Time Basis Query

De kern van alle queries: de laatst bekende lint result per API op een bepaalde datum.

```sql
-- Laatst bekende lint result per API op :end_date
WITH latest_results AS (
    SELECT DISTINCT ON (lr.api_id)
        lr.id,
        lr.api_id,
        lr.successes,
        lr.failures,
        lr.warnings,
        lr.created_at,
        lr.adr_version
    FROM lint_results lr
    WHERE lr.created_at <= :end_date
      AND lr.adr_version = :adr_version
      -- Optionele filters:
      -- AND lr.api_id IN (:api_ids)
      -- AND lr.api_id IN (SELECT id FROM apis WHERE organisation_id = :organisation)
    ORDER BY lr.api_id, lr.created_at DESC
)
SELECT * FROM latest_results;
```

### 4.2 Summary Query

```sql
WITH latest_results AS (
    SELECT DISTINCT ON (lr.api_id)
        lr.api_id,
        lr.successes
    FROM lint_results lr
    WHERE lr.created_at <= :end_date
      AND lr.adr_version = :adr_version
    ORDER BY lr.api_id, lr.created_at DESC
)
SELECT
    COUNT(*) AS total_apis,
    COUNT(*) FILTER (WHERE successes = true) AS compliant_apis,
    COALESCE(ROUND(
        (COUNT(*) FILTER (WHERE successes = true)::numeric / NULLIF(COUNT(*), 0)) * 100,
        1
    ), 0) AS adoption_rate
FROM latest_results;

-- Separaat: totaal aantal lint runs in de periode
SELECT COUNT(*) AS total_lint_runs
FROM lint_results
WHERE created_at BETWEEN :start_date AND :end_date
  AND adr_version = :adr_version;
```

### 4.3 Rules Query

```sql
WITH latest_results AS (
    SELECT DISTINCT ON (lr.api_id)
        lr.id AS lint_result_id,
        lr.api_id
    FROM lint_results lr
    WHERE lr.created_at <= :end_date
      AND lr.adr_version = :adr_version
    ORDER BY lr.api_id, lr.created_at DESC
),
total_count AS (
    SELECT COUNT(*) AS total_apis FROM latest_results
),
violations_per_rule AS (
    SELECT
        lm.code,
        lm.severity,
        COUNT(DISTINCT lr.api_id) AS violating_apis
    FROM latest_results lr
    JOIN lint_messages lm ON lm.lint_result_id = lr.lint_result_id
    WHERE 1=1
    -- Optioneel: AND lm.code IN (:rule_codes)
    -- Optioneel: AND lm.severity = :severity
    GROUP BY lm.code, lm.severity
)
SELECT
    v.code,
    v.severity,
    v.violating_apis,
    t.total_apis
FROM violations_per_rule v
CROSS JOIN total_count t
ORDER BY v.code;
```

### 4.4 Timeline Query (Per Regel)

Altijd per regel. Zonder `ruleCodes` filter worden eerst alle regels opgehaald via een aparte query op de laatst bekende lint results.

```sql
WITH date_series AS (
    SELECT generate_series(
        date_trunc(:granularity, :start_date::date),
        date_trunc(:granularity, :end_date::date),
        ('1 ' || :granularity)::interval
    )::date AS period_start
),
rules AS (
    SELECT unnest(ARRAY[:rule_codes]::text[]) AS code
),
period_rules AS (
    SELECT
        ds.period_start,
        r.code AS rule_code,
        CASE :granularity
            WHEN 'day' THEN ds.period_start
            WHEN 'week' THEN ds.period_start + interval '6 days'
            WHEN 'month' THEN (ds.period_start + interval '1 month' - interval '1 day')::date
        END AS period_end
    FROM date_series ds
    CROSS JOIN rules r
)
SELECT
    pr.rule_code,
    TO_CHAR(pr.period_start,
        CASE :granularity
            WHEN 'day' THEN 'YYYY-MM-DD'
            WHEN 'week' THEN 'IYYY-"W"IW'
            WHEN 'month' THEN 'YYYY-MM'
        END
    ) AS period,
    (
        SELECT COUNT(DISTINCT api_id)
        FROM lint_results
        WHERE created_at <= pr.period_end
          AND adr_version = :adr_version
    ) AS total_apis,
    (
        SELECT COUNT(DISTINCT sub.api_id)
        FROM (
            SELECT DISTINCT ON (api_id) id, api_id
            FROM lint_results
            WHERE created_at <= pr.period_end
              AND adr_version = :adr_version
            ORDER BY api_id, created_at DESC
        ) sub
        WHERE NOT EXISTS (
            SELECT 1 FROM lint_messages lm
            WHERE lm.lint_result_id = sub.id
              AND lm.code = pr.rule_code
        )
    ) AS compliant_apis
FROM period_rules pr
ORDER BY pr.rule_code, pr.period_start;
```

### 4.5 APIs Query

```sql
WITH latest_results AS (
    SELECT DISTINCT ON (lr.api_id)
        lr.id AS lint_result_id,
        lr.api_id,
        lr.successes,
        lr.failures,
        lr.warnings,
        lr.created_at
    FROM lint_results lr
    WHERE lr.created_at <= :end_date
      AND lr.adr_version = :adr_version
    ORDER BY lr.api_id, lr.created_at DESC
),
api_violations AS (
    SELECT
        lr.api_id,
        ARRAY_AGG(DISTINCT lm.code ORDER BY lm.code) AS violated_rules
    FROM latest_results lr
    JOIN lint_messages lm ON lm.lint_result_id = lr.lint_result_id
    WHERE lm.severity = 'error'
    GROUP BY lr.api_id
)
SELECT
    lr.api_id,
    a.title AS api_title,
    COALESCE(o.label, '') AS organisation,
    lr.successes AS is_compliant,
    lr.failures AS total_violations,
    lr.warnings AS total_warnings,
    COALESCE(av.violated_rules, ARRAY[]::text[]) AS violated_rules,
    lr.created_at AS last_lint_date
FROM latest_results lr
JOIN apis a ON a.id = lr.api_id
LEFT JOIN organisations o ON o.uri = a.organisation_id
LEFT JOIN api_violations av ON av.api_id = lr.api_id
-- Optionele filters:
-- WHERE lr.successes = :compliant
-- WHERE av.violated_rules && ARRAY[:rule_codes]::text[]
ORDER BY a.title
LIMIT :per_page
OFFSET (:page - 1) * :per_page;
```

---

## 5. Go Implementatie

### 5.1 Bestandsstructuur

```
pkg/api_client/
├── models/
│   └── adoption.go             # Response models + query param structs
├── repositories/
│   └── adoption_repository.go  # Database queries (raw SQL via db.Raw)
├── services/
│   └── adoption_service.go     # Business logic, date parsing, response assembly
├── handler/
│   └── statistics_handler.go   # HTTP handlers (StatisticsController)
└── routers.go                  # Route registratie (wijzigen)
```

### 5.2 Models + Params (models/adoption.go)

Params zitten in het `models` package (bestaand patroon, zie `list_apis_params.go`).

```go
package models

import "time"

// Gemeenschappelijk
type Period struct {
    Start string `json:"start"`
    End   string `json:"end"`
}

// Summary endpoint
type AdoptionSummary struct {
    AdrVersion          string  `json:"adrVersion"`
    Period              Period  `json:"period"`
    TotalApis           int     `json:"totalApis"`
    CompliantApis       int     `json:"compliantApis"`
    OverallAdoptionRate float64 `json:"overallAdoptionRate"`
    TotalLintRuns       int     `json:"totalLintRuns"`
}

// Rules endpoint
type AdoptionRules struct {
    AdrVersion string         `json:"adrVersion"`
    Period     Period         `json:"period"`
    TotalApis  int            `json:"totalApis"`
    Rules      []RuleAdoption `json:"rules"`
}

type RuleAdoption struct {
    Code          string  `json:"code"`
    Severity      string  `json:"severity"`
    ViolatingApis int     `json:"violatingApis"`
    CompliantApis int     `json:"compliantApis"`
    AdoptionRate  float64 `json:"adoptionRate"`
}

// Timeline endpoint
type AdoptionTimeline struct {
    AdrVersion  string           `json:"adrVersion"`
    Granularity string           `json:"granularity"`
    Series      []TimelineSeries `json:"series"`
}

type TimelineSeries struct {
    Type       string          `json:"type"`
    RuleCode   string          `json:"ruleCode,omitempty"`
    DataPoints []TimelinePoint `json:"dataPoints"`
}

type TimelinePoint struct {
    Period        string  `json:"period"`
    TotalApis     int     `json:"totalApis"`
    CompliantApis int     `json:"compliantApis"`
    AdoptionRate  float64 `json:"adoptionRate"`
}

// APIs endpoint
type AdoptionApis struct {
    AdrVersion string        `json:"adrVersion"`
    Period     Period        `json:"period"`
    Apis       []ApiAdoption `json:"apis"`
}

type ApiAdoption struct {
    ApiId           string    `json:"apiId"`
    ApiTitle        string    `json:"apiTitle"`
    Organisation    string    `json:"organisation"`
    IsCompliant     bool      `json:"isCompliant"`
    TotalViolations int       `json:"totalViolations"`
    TotalWarnings   int       `json:"totalWarnings"`
    ViolatedRules   []string  `json:"violatedRules"`
    LastLintDate    time.Time `json:"lastLintDate"`
}

// Query parameter structs
type AdoptionBaseParams struct {
    AdrVersion   string  `query:"adrVersion" binding:"required"`
    StartDate    string  `query:"startDate" binding:"required"`
    EndDate      string  `query:"endDate" binding:"required"`
    ApiIds       *string `query:"apiIds"`
    Organisation *string `query:"organisation"`
}

type AdoptionRulesParams struct {
    AdoptionBaseParams
    RuleCodes *string `query:"ruleCodes"`
    Severity  *string `query:"severity"`
}

type AdoptionTimelineParams struct {
    AdoptionBaseParams
    Granularity string  `query:"granularity"`
    RuleCodes   *string `query:"ruleCodes"`
}

type AdoptionApisParams struct {
    AdoptionBaseParams
    Compliant *bool   `query:"compliant"`
    RuleCodes *string `query:"ruleCodes"`
    Page      int     `query:"page"`
    PerPage   int     `query:"perPage"`
}
```

### 5.3 Repository (repositories/adoption_repository.go)

Gebruikt `db.Raw()` met parameterized queries. Geeft ruwe resultaat-structs terug (geen response models).

```go
type AdoptionRepository interface {
    GetSummary(ctx context.Context, params AdoptionQueryParams) (SummaryResult, int, error)
    GetRules(ctx context.Context, params AdoptionQueryParams) ([]RuleRow, int, error)
    GetTimeline(ctx context.Context, params TimelineQueryParams) ([]TimelineRow, error)
    GetApis(ctx context.Context, params ApisQueryParams) ([]ApiRow, int, error)
}
```

Dynamische SQL-opbouw: filters (`apiIds`, `organisation`, `ruleCodes`, `severity`, `compliant`) worden conditioneel toegepast via string builder + args slice.

### 5.4 Service (services/adoption_service.go)

```go
type AdoptionService struct { repo AdoptionRepository }
```

Verantwoordelijkheden:
- Parse date strings (`time.Parse("2006-01-02", ...)`)
- Split comma-separated params
- Validate granularity (default "month")
- Validate page/perPage defaults
- Repo aanroepen, response models assembleren
- `adoptionRate` berekenen: `math.Round(float64(compliant)/float64(total)*1000) / 10`

### 5.5 Handler (handler/statistics_handler.go)

```go
type StatisticsController struct { Service *services.AdoptionService }
```

Methodes volgen bestaand patroon (zie `handler/api_handler.go`):
- `GetSummary(ctx *gin.Context, p *models.AdoptionBaseParams) (*models.AdoptionSummary, error)`
- `GetRules(ctx *gin.Context, p *models.AdoptionRulesParams) (*models.AdoptionRules, error)`
- `GetTimeline(ctx *gin.Context, p *models.AdoptionTimelineParams) (*models.AdoptionTimeline, error)`
- `GetApis(ctx *gin.Context, p *models.AdoptionApisParams) (*models.AdoptionApis, error)` — zet pagination headers via `util.SetPaginationHeaders()`

### 5.6 Route Registratie (routers.go wijzigen)

`NewRouter` krijgt extra parameter `statsController *handler.StatisticsController`.

```go
statsGroup := f.Group("/v1/statistics", "Statistics", "ADR adoption statistics endpoints")

statsGroup.GET("/summary",
    []fizz.OperationOption{
        fizz.ID("getAdoptionSummary"),
        fizz.Summary("Get adoption summary"),
        fizz.Description("Returns overall adoption KPIs for the selected period"),
        apiVersionHeaderOption,
        badRequestResponse,
    },
    tonic.Handler(statsController.GetSummary, 200),
)
// ... analoog voor /rules, /timeline, /apis
```

### 5.7 DI Wiring (cmd/main.go wijzigen)

```go
adoptionRepo := repositories.NewAdoptionRepository(db)
adoptionService := services.NewAdoptionService(adoptionRepo)
statsController := handler.NewStatisticsController(adoptionService)

router := api.NewRouter(version, APIsAPIController, statsController)
```

---

## 6. Aandachtspunten

### 6.1 Performance Overwegingen
- De timeline queries met subqueries per periode kunnen zwaar zijn bij grote datasets
- Overweeg een index op `(adr_version, api_id, created_at DESC)` voor de `lint_results` tabel
- Bij performance problemen: overweeg materialized views of een snapshot tabel

---

## 7. Testen

### 7.1 Unit Tests
- Test repository methodes met mock database
- Test berekening van adoptieRate (edge cases: 0 API's, 100% compliant, etc.)

### 7.2 Integratie Tests
- Test point-in-time logica: API gevalideerd op dag X moet meetellen op dag X+n
- Test filters werken correct in combinatie
- Test pagination headers

### 7.3 Handmatig Testen
```bash
# Summary
curl "http://localhost:1337/v1/statistics/summary?adrVersion=1.0.0&startDate=2024-01-01&endDate=2024-06-30"

# Rules
curl "http://localhost:1337/v1/statistics/rules?adrVersion=1.0.0&startDate=2024-01-01&endDate=2024-06-30&severity=error"

# Timeline (alle regels)
curl "http://localhost:1337/v1/statistics/timeline?adrVersion=1.0.0&startDate=2024-01-01&endDate=2024-06-30&granularity=month"

# Timeline (specifieke regels)
curl "http://localhost:1337/v1/statistics/timeline?adrVersion=1.0.0&startDate=2024-01-01&endDate=2024-06-30&granularity=month&ruleCodes=API-01,API-05"

# APIs
curl "http://localhost:1337/v1/statistics/apis?adrVersion=1.0.0&startDate=2024-01-01&endDate=2024-06-30&compliant=false&page=1&perPage=20"
```

---

## 8. Checklist

- [x] `adr_version` veld toevoegen aan `LintResult` model
- [x] `models/adoption.go` aanmaken met response structs + param structs
- [x] `repositories/adoption_repository.go` aanmaken met queries
- [x] `services/adoption_service.go` aanmaken
- [x] `handler/statistics_handler.go` aanmaken
- [x] Routes registreren in `routers.go`
- [x] DI wiring in `cmd/main.go`
- [ ] `go build ./...` succesvol
- [ ] Database index toevoegen voor performance
- [ ] Unit tests schrijven
- [ ] Integratie tests schrijven
- [ ] OpenAPI spec genereren en valideren

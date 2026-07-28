# API Register Sorting Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add validated, deterministic ascending and descending sorting by title, ADR score, and registered API version to `GET /apis`.

**Architecture:** Parse the two query parameters into a focused `models.ApiSort` value in the service, then pass it to the repository. The repository applies the existing filters, sorts the entire filtered set in Go, and only then slices the requested page; semantic versions use `golang.org/x/mod/semver` after normalizing short numeric forms and a leading `v`.

**Tech Stack:** Go 1.26.5, Gin/Fizz/Tonic, GORM, SQLite/PostgreSQL, `golang.org/x/mod/semver`, Testify, OpenAPI 3.1 JSON.

## Global Constraints

- Public parameters are exactly `sortBy=title|adrScore|version` and `sortOrder=asc|desc`.
- Omitted values default independently to `title` and `asc`.
- Sort after filtering and before pagination.
- Title comparison is case-insensitive.
- ADR scores are numeric.
- API versions use semantic ordering and accept short numeric forms plus an optional `v` prefix.
- Missing ADR scores and missing or invalid versions remain last for both sort directions.
- Ties use case-insensitive title and then API id for deterministic results.
- Invalid sort values return HTTP 400 and identify the invalid query parameter.
- Sorting applies only to `GET /apis`; deprecated search and filter-count endpoints remain unchanged.

---

### Task 1: Sort request model and validation

**Files:**
- Create: `pkg/api_client/models/api_sort.go`
- Create: `pkg/api_client/models/api_sort_test.go`
- Modify: `pkg/api_client/models/list_apis_params.go`
- Modify: `pkg/api_client/services/api_service.go`
- Modify: `pkg/api_client/services/api_service_test.go`

**Interfaces:**
- Produces: `models.ApiSort`, `models.ApiSortField`, `models.ApiSortOrder`, `models.ParseApiSort(sortBy, sortOrder string) (models.ApiSort, error)`.
- Produces: `models.InvalidApiSortError` with `Parameter` and `Value` fields.
- Changes: `repositories.ApiRepository.GetApis(context.Context, int, int, *models.ApiFiltersParams, models.ApiSort)` and every test stub implementing that interface.

- [ ] **Step 1: Write failing model tests for defaults, valid values, and invalid parameters**

```go
func TestParseApiSort(t *testing.T) {
    got, err := ParseApiSort("", "")
    require.NoError(t, err)
    assert.Equal(t, ApiSort{Field: ApiSortTitle, Order: ApiSortAscending}, got)

    got, err = ParseApiSort("version", "desc")
    require.NoError(t, err)
    assert.Equal(t, ApiSort{Field: ApiSortVersion, Order: ApiSortDescending}, got)
}

func TestParseApiSortRejectsUnsupportedValues(t *testing.T) {
    _, err := ParseApiSort("createdAt", "asc")
    var invalid InvalidApiSortError
    require.ErrorAs(t, err, &invalid)
    assert.Equal(t, "sortBy", invalid.Parameter)

    _, err = ParseApiSort("title", "sideways")
    require.ErrorAs(t, err, &invalid)
    assert.Equal(t, "sortOrder", invalid.Parameter)
}
```

- [ ] **Step 2: Run the model tests and confirm RED**

Run: `go test ./pkg/api_client/models -run 'TestParseApiSort' -count=1`

Expected: compilation fails because the sort types and parser do not exist.

- [ ] **Step 3: Add the minimal sort types and parser**

```go
type ApiSortField string
type ApiSortOrder string

const (
    ApiSortTitle    ApiSortField = "title"
    ApiSortADRScore ApiSortField = "adrScore"
    ApiSortVersion  ApiSortField = "version"

    ApiSortAscending  ApiSortOrder = "asc"
    ApiSortDescending ApiSortOrder = "desc"
)

type ApiSort struct {
    Field ApiSortField
    Order ApiSortOrder
}

type InvalidApiSortError struct {
    Parameter string
    Value     string
}
```

`ParseApiSort` fills empty values with the defaults and accepts only the exact
contract values. Its error message lists the allowed values for the rejected
parameter.

- [ ] **Step 4: Add raw request fields and a failing service propagation test**

Add to `ListApisParams`:

```go
SortBy    string `query:"sortBy"`
SortOrder string `query:"sortOrder"`
```

Extend the repository stub callback and assert the parsed sort value:

```go
getApis: func(ctx context.Context, page, perPage int, p *models.ApiFiltersParams, sorting models.ApiSort) ([]models.Api, models.Pagination, error) {
    assert.Equal(t, models.ApiSortVersion, sorting.Field)
    assert.Equal(t, models.ApiSortDescending, sorting.Order)
    return nil, models.Pagination{}, nil
},
```

Also add a service test asserting that `sortBy=unknown` returns a
`problem.APIError` with status 400, `In: "query"`, and
`Location: "sortBy"`, without calling the repository.

- [ ] **Step 5: Run the service test and confirm RED**

Run: `go test ./pkg/api_client/services -run 'TestListApis_.*Sort' -count=1`

Expected: compilation or assertion failure because the service and repository
interface do not forward an `ApiSort` yet.

- [ ] **Step 6: Parse in the service and update the repository interface/stubs**

In `ListApis`, call `models.ParseApiSort(p.SortBy, p.SortOrder)` before the
repository. Map `InvalidApiSortError` to:

```go
problem.New(http.StatusBadRequest, "Request validation failed", problem.ErrorDetail{
    In:       "query",
    Location: invalid.Parameter,
    Code:     invalid.Parameter,
    Detail:   invalid.Error(),
})
```

Pass the parsed `models.ApiSort` as the final `GetApis` argument and update all
interface implementations in service, handler, and OAS tests to accept it.

- [ ] **Step 7: Run focused tests and confirm GREEN**

Run: `go test ./pkg/api_client/models ./pkg/api_client/services -run 'TestParseApiSort|TestListApis_.*Sort' -count=1`

Expected: PASS.

- [ ] **Step 8: Commit the validated request contract**

```bash
git add pkg/api_client/models/api_sort.go pkg/api_client/models/api_sort_test.go pkg/api_client/models/list_apis_params.go pkg/api_client/services/api_service.go pkg/api_client/services/api_service_test.go pkg/api_client/services/api_service_oas_test.go pkg/api_client/services/api_service_internal_test.go pkg/api_client/handler/api_handler_test.go pkg/api_client/repositories/api_repositorie.go
git commit -m "feat: validate API sort parameters"
```

### Task 2: Filtered and paginated API ordering

**Files:**
- Create: `pkg/api_client/repositories/api_sort.go`
- Create: `pkg/api_client/repositories/api_sort_test.go`
- Modify: `pkg/api_client/repositories/api_repositorie.go`
- Modify: `pkg/api_client/repositories/api_repositorie_test.go`
- Modify: `go.mod`
- Modify: `go.sum`

**Interfaces:**
- Consumes: `models.ApiSort{Field, Order}` from Task 1.
- Produces: unexported `sortApis([]models.Api, models.ApiSort)` and semantic-version normalization helpers owned by the repository package.
- Changes: `apiRepository.GetApis` sorts `filtered` immediately before computing and slicing the requested page.

- [ ] **Step 1: Write failing table tests for the three comparators**

Create API fixtures with ids that make incorrect tie-breaking visible and
assert complete id sequences for:

```go
tests := []struct {
    name string
    sort models.ApiSort
    want []string
}{
    {"title ascending", models.ApiSort{Field: models.ApiSortTitle, Order: models.ApiSortAscending}, []string{"alpha", "beta", "zulu"}},
    {"title descending", models.ApiSort{Field: models.ApiSortTitle, Order: models.ApiSortDescending}, []string{"zulu", "beta", "alpha"}},
    {"ADR ascending with nil last", models.ApiSort{Field: models.ApiSortADRScore, Order: models.ApiSortAscending}, []string{"score-10", "score-90", "score-missing"}},
    {"ADR descending with nil last", models.ApiSort{Field: models.ApiSortADRScore, Order: models.ApiSortDescending}, []string{"score-90", "score-10", "score-missing"}},
    {"version ascending", models.ApiSort{Field: models.ApiSortVersion, Order: models.ApiSortAscending}, []string{"v1-9", "v1-10", "v2", "version-invalid", "version-empty"}},
    {"version descending", models.ApiSort{Field: models.ApiSortVersion, Order: models.ApiSortDescending}, []string{"v2", "v1-10", "v1-9", "version-invalid", "version-empty"}},
}
```

Add focused cases for case-insensitive title ties, optional `v`, one/two/three
numeric components, prerelease ordering, and id tie-breaking.

- [ ] **Step 2: Run comparator tests and confirm RED**

Run: `go test ./pkg/api_client/repositories -run 'TestSortApis' -count=1`

Expected: compilation fails because `sortApis` does not exist.

- [ ] **Step 3: Add semantic-version dependency and minimal comparator code**

Run: `go get golang.org/x/mod/semver@latest`

Implement normalization by trimming whitespace, removing one case-insensitive
leading `v`, splitting the numeric core from prerelease/build suffixes, padding
one- and two-part numeric cores to three components, then validating the
result with `semver.IsValid`. Compare valid versions with `semver.Compare`.

`sortApis` must reverse only valid primary comparisons for descending order;
the valid-versus-missing decision is never reversed. When the primary values
tie or are both invalid, compare lower-cased titles ascending and ids ascending.

- [ ] **Step 4: Run comparator tests and confirm GREEN**

Run: `go test ./pkg/api_client/repositories -run 'TestSortApis' -count=1`

Expected: PASS.

- [ ] **Step 5: Write a failing repository test for filter → sort → paginate**

Insert at least four APIs in deliberately unsorted database order. Request a
filter that retains three APIs, `version desc`, page 1, perPage 2, and assert
that the response contains the two highest semantic versions while pagination
reports all three filtered records. Add a default request assertion proving
title ascending remains unchanged.

- [ ] **Step 6: Run the repository test and confirm RED**

Run: `go test ./pkg/api_client/repositories -run 'TestApiRepository_GetApisSortsBeforePagination|TestApiRepository_GetApisDefaultsToTitleAscending' -count=1`

Expected: FAIL because `GetApis` still uses fixed database title ordering and
does not sort the filtered set from `models.ApiSort`.

- [ ] **Step 7: Apply sorting between filtering and pagination**

Remove the fixed `applyApiOrdering` call from `GetApis`, call
`sortApis(filtered, sorting)` after the filter loop, and leave the existing
pagination calculation/slicing immediately after it. Keep `applyApiOrdering`
for the deprecated search endpoint so that endpoint remains unchanged.

- [ ] **Step 8: Run repository and related package tests**

Run: `go test ./pkg/api_client/repositories ./pkg/api_client/services ./pkg/api_client/handler -count=1`

Expected: PASS.

- [ ] **Step 9: Commit ordering behavior**

```bash
git add go.mod go.sum pkg/api_client/repositories/api_sort.go pkg/api_client/repositories/api_sort_test.go pkg/api_client/repositories/api_repositorie.go pkg/api_client/repositories/api_repositorie_test.go
git commit -m "feat: sort filtered APIs before pagination"
```

### Task 3: HTTP contract, pagination links, and OpenAPI

**Files:**
- Modify: `pkg/api_client/integration_test.go`
- Modify: `pkg/api_client/handler/api_handler_test.go`
- Modify: `pkg/api_client/routers.go`
- Modify: `api/openapi.json`
- Modify: `api/spec_test.go`

**Interfaces:**
- Consumes: `GET /apis` query parsing and repository ordering from Tasks 1-2.
- Produces: documented `SortBy` and `SortOrder` OpenAPI components referenced only by `GET /apis`.

- [ ] **Step 1: Write failing HTTP tests**

Add integration coverage that creates multiple versions and requests:

```text
/v1/apis?sortBy=version&sortOrder=desc&page=1&perPage=2
```

Assert the response order and that the `Link` header retains
`sortBy=version&sortOrder=desc`. Add invalid `sortBy` and `sortOrder` requests,
assert status 400, content type `application/problem+json`, and error locations
matching the query parameter names.

- [ ] **Step 2: Run HTTP tests and confirm RED**

Run: `go test ./pkg/api_client -run 'TestListApis_.*Sort' -count=1`

Expected: FAIL until the complete HTTP behavior and error response are wired.

- [ ] **Step 3: Finish HTTP behavior and router description**

Ensure `ListApisParams` binds both query fields and update the `/apis` router
description to mention filtering, searching, and sorting. No pagination helper
change is needed: the shared helper clones all request query parameters before
replacing only `page` and `perPage`.

- [ ] **Step 4: Add failing embedded-spec assertions**

In `api/spec_test.go`, assert that `GET /apis` references both parameters and
that their component schemas contain the exact enums and defaults:

```go
assert.Equal(t, []any{"title", "adrScore", "version"}, sortBySchema["enum"])
assert.Equal(t, "title", sortBySchema["default"])
assert.Equal(t, []any{"asc", "desc"}, sortOrderSchema["enum"])
assert.Equal(t, "asc", sortOrderSchema["default"])
```

- [ ] **Step 5: Run the spec test and confirm RED**

Run: `go test ./api -run 'TestListApisDocumentsSorting' -count=1`

Expected: FAIL because the OpenAPI parameters do not exist.

- [ ] **Step 6: Update OpenAPI JSON**

Add reusable query parameter components named `SortBy` and `SortOrder` with
lowerCamelCase names, string schemas, enums, defaults, descriptions, and
examples. Reference both from `GET /apis` after the pagination parameters.

- [ ] **Step 7: Run focused HTTP and specification tests**

Run: `go test ./api ./pkg/api_client -run 'TestListApis.*Sort|TestListApisDocumentsSorting' -count=1`

Expected: PASS.

- [ ] **Step 8: Validate formatting, the full test suite, and OpenAPI**

Run:

```bash
gofmt -w pkg/api_client/models/api_sort.go pkg/api_client/models/api_sort_test.go pkg/api_client/models/list_apis_params.go pkg/api_client/services/api_service.go pkg/api_client/services/api_service_test.go pkg/api_client/services/api_service_oas_test.go pkg/api_client/services/api_service_internal_test.go pkg/api_client/handler/api_handler_test.go pkg/api_client/repositories/api_sort.go pkg/api_client/repositories/api_sort_test.go pkg/api_client/repositories/api_repositorie.go pkg/api_client/repositories/api_repositorie_test.go pkg/api_client/integration_test.go api/spec_test.go
go test ./... -count=1
npx @developer-overheid-nl/don-checker@latest validate --ruleset adr-21 --input api/openapi.json
git diff --check
```

Expected: Go tests pass, the checker reports no errors (document any existing
warnings without introducing new ones), and `git diff --check` is silent.

- [ ] **Step 9: Commit endpoint documentation and end-to-end coverage**

```bash
git add api/openapi.json api/spec_test.go pkg/api_client/integration_test.go pkg/api_client/handler/api_handler_test.go pkg/api_client/routers.go docs/superpowers/plans/2026-07-28-api-register-sorting.md
git commit -m "docs: expose API sorting contract"
```

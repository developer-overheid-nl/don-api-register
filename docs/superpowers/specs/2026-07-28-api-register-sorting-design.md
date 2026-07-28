# API Register Sorting Design

## Goal

Add deterministic sorting to `GET /apis` so clients can order filtered API
results by title, ADR score, or registered API version in ascending or
descending direction.

## Scope

Sorting applies to `GET /apis`, including requests that also use the existing
filters or the `q` search parameter. The deprecated `GET /apis/_search` and the
filter-count endpoint are unchanged.

## API contract

The endpoint accepts two optional lowerCamelCase query parameters:

- `sortBy`: `title`, `adrScore`, or `version`;
- `sortOrder`: `asc` or `desc`.

When either value is omitted, it defaults independently: `sortBy=title` and
`sortOrder=asc`. This preserves the current title-ascending default.

Unsupported values produce HTTP `400` using the API's existing problem
response format. The response identifies `sortBy` or `sortOrder` as the
invalid query parameter. The OpenAPI specification documents both parameters,
their enum values, defaults, descriptions, and examples.

Example requests:

```text
GET /apis?sortBy=title&sortOrder=desc
GET /apis?sortBy=adrScore&sortOrder=asc
GET /apis?sortBy=version&sortOrder=desc
```

## Ordering semantics

Sorting is stable and deterministic. It uses the selected field first, then a
case-insensitive title comparison, and finally the API id as a unique
tie-breaker.

### Title

Titles are ordered case-insensitively. Title ties are resolved by API id.

### ADR score

ADR scores are ordered numerically. A missing ADR score always follows APIs
with a score, for both ascending and descending order.

### API version

The registered API version (`Api.Version`, exposed as
`ApiSummary.Lifecycle.Version`) is ordered semantically rather than
lexicographically. Numeric dotted versions are normalized for comparison so
that short versions such as `1` and `1.9`, full versions such as `1.10.0`, and
the same forms with a leading `v` are supported. Consequently, ascending order
places `1.9` before `1.10`.

Empty versions and values that cannot be interpreted as semantic versions are
treated as invalid and always follow valid versions, for both ascending and
descending order. Invalid values are ordered deterministically using the
normal tie-breakers.

## Architecture and data flow

The request model gains `sortBy` and `sortOrder`. The service normalizes the
defaults, validates the allowed values, and passes a focused sort configuration
to the repository.

The repository follows this sequence:

1. load the candidate APIs and required associations;
2. apply existing SQL-capable and in-memory filters;
3. sort the complete filtered result set in Go;
4. calculate pagination metadata and slice the requested page.

Sorting in Go is preferred because the repository already materializes the
candidate set for existing in-memory filters, and semantic version ordering is
not portable across the supported SQLite and PostgreSQL databases. A small,
focused sorting module owns normalization and comparison. No database migration
or stored sort key is introduced.

Pagination links retain all incoming query parameters, including `sortBy` and
`sortOrder`.

## Error handling

Validation happens before the repository is called. Invalid sort parameters do
not silently fall back to defaults. Data-quality issues in ADR score or version
values do not fail the request; those records are placed at the end as defined
above.

## Testing

Tests cover:

- ascending and descending ordering for title, ADR score, and API version;
- case-insensitive title ordering and deterministic tie-breakers;
- numeric ADR ordering with missing values last in both directions;
- semantic version cases including `1.9`, `1.10`, full three-part versions,
  and a leading `v`;
- empty and invalid versions last in both directions;
- sorting after filtering and before pagination;
- the unchanged title-ascending default;
- invalid `sortBy` and `sortOrder` responses;
- query-parameter preservation in pagination links;
- OpenAPI enum, default, and description coverage;
- the complete Go test suite and OpenAPI validation already used by the
  repository.

## Non-goals

- Multi-column sorting selected by clients;
- sorting the deprecated search endpoint;
- sorting filter-count groups;
- database schema changes or denormalized version fields.

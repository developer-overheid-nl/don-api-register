package services

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	httpclient "github.com/developer-overheid-nl/don-api-register/pkg/api_client/helpers/httpclient"
	openapi "github.com/developer-overheid-nl/don-api-register/pkg/api_client/helpers/openapi"
	toolslint "github.com/developer-overheid-nl/don-api-register/pkg/api_client/helpers/tools"
	"github.com/developer-overheid-nl/don-api-register/pkg/api_client/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/time/rate"
)

func TestServicePureHelpers(t *testing.T) {
	assert.Equal(t, models.OASStatusUnknown, classifyOASStatus(assert.AnError))
	assert.Equal(t, models.OASStatusUnreachable, classifyOASStatus(errString("kan OAS niet ophalen")))
	assert.Equal(t, models.OASStatusInvalid, classifyOASStatus(errString("invalid OAS (parse)")))
	assert.Equal(t, models.OASStatusValid, classifyOASStatus(nil))

	assert.Equal(t, "unknown", scorePtrValue(nil))
	score := 42
	assert.Equal(t, "42", scorePtrValue(&score))

	orgID := " https://example.org/org "
	assert.Equal(t, "https://example.org/org", deriveOrganisationURI(&models.Api{OrganisationID: &orgID}))
	assert.Equal(t, "https://example.org/fallback", deriveOrganisationURI(&models.Api{Organisation: &models.Organisation{Uri: " https://example.org/fallback "}}))
	assert.Empty(t, deriveOrganisationURI(nil))

	assert.False(t, hasOASUpdateInput(nil))
	assert.True(t, hasOASUpdateInput(&models.UpdateApiInput{OasBody: "{}"}))
	assert.True(t, hasOASDocumentChange(&models.Api{OasUri: "https://old.example.org/openapi.json"}, &models.UpdateApiInput{OasUrl: "https://new.example.org/openapi.json"}))
	assert.False(t, hasOASDocumentChange(&models.Api{OasUri: "https://same.example.org/openapi.json"}, &models.UpdateApiInput{OasUrl: " https://same.example.org/openapi.json "}))

	assert.Equal(t, "application/json", formatContentType("json"))
	assert.Equal(t, "application/yaml", formatContentType("yml"))
	assert.Equal(t, "application/octet-stream", formatContentType("txt"))
	assert.Equal(t, "oas-3.1-converted.json", oasFilename("3.1", "converted", "json"))

	short, full, ok := targetVersion(&openapi.OASResult{Minor: 0})
	assert.True(t, ok)
	assert.Equal(t, "3.1", short)
	assert.Equal(t, "3.1.0", full)

	short, full, ok = targetVersion(&openapi.OASResult{Minor: 2})
	assert.False(t, ok)
	assert.Empty(t, short)
	assert.Empty(t, full)
}

func TestLifecycleHelpers(t *testing.T) {
	sunset := models.NewOptionalString("2027-01-01")
	deprecated := models.NewOptionalString(" 2026-01-01 ")
	body := &models.UpdateApiInput{Sunset: sunset, Deprecated: deprecated}

	require.NoError(t, validateLifecycleOverrides(body))

	api := &models.Api{}
	applyLifecycleOverrides(api, body)
	assert.Equal(t, "2027-01-01", api.Sunset)
	assert.Equal(t, "2026-01-01", api.Deprecated)

	nullSunset := models.NewNullString()
	applyLifecycleOverrides(api, &models.UpdateApiInput{Sunset: nullSunset})
	assert.Empty(t, api.Sunset)

	invalid := models.NewOptionalString("01-01-2027")
	err := validateLifecycleOverrides(&models.UpdateApiInput{Sunset: invalid})
	require.Error(t, err)
	apiErr, ok := err.(interface {
		error
	})
	require.True(t, ok)
	assert.Contains(t, apiErr.Error(), "Request validation failed")
}

func TestFeedEventHelpers(t *testing.T) {
	title, description := feedEventText(models.ApiFeedEventLifecycleChanged, "active", "deprecated")
	assert.Equal(t, "Lifecycle gewijzigd", title)
	assert.Contains(t, description, "active")
	assert.Contains(t, description, "deprecated")

	title, description = feedEventText("custom", "old", "new")
	assert.Equal(t, "custom", title)
	assert.Empty(t, description)

	before := models.Api{Deprecated: "2026-01-01"}
	after := models.Api{Deprecated: "2027-01-01", Sunset: "2028-01-01"}
	assert.Equal(t, "deprecated, deprecated=2027-01-01, sunset=2028-01-01", lifecycleFeedValue(after, before, "deprecated"))
}

func TestDetectOASFormatAndPrettyJSON(t *testing.T) {
	tests := []struct {
		name        string
		raw         []byte
		contentType string
		want        string
	}{
		{name: "json content type", raw: []byte(`openapi: 3.1.0`), contentType: "application/json", want: "json"},
		{name: "yaml content type", raw: []byte(`{"openapi":"3.1.0"}`), contentType: "application/x-yaml", want: "yaml"},
		{name: "json body", raw: []byte(` {"openapi":"3.1.0"}`), want: "json"},
		{name: "yaml body", raw: []byte("openapi: 3.1.0\ninfo:\n  title: T"), want: "yaml"},
		{name: "yaml document marker", raw: []byte("---\nopenapi: 3.1.0"), want: "yaml"},
	}

	for _, tc := range tests {
		current := tc
		t.Run(current.name, func(t *testing.T) {
			got, err := detectOASFormat(current.raw, current.contentType)
			require.NoError(t, err)
			assert.Equal(t, current.want, got)
		})
	}

	_, err := detectOASFormat([]byte("   "), "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "leeg document")

	_, err = detectOASFormat([]byte("not an oas"), "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "onbekend formaat")

	pretty, err := prettyJSON([]byte(`{"b":2,"a":1}`))
	require.NoError(t, err)
	assert.Contains(t, string(pretty), "\n")

	pretty, err = toPrettyJSON([]byte("openapi: 3.1.0\ninfo:\n  title: T\n  version: 1\npaths: {}\n"), "yaml")
	require.NoError(t, err)
	assert.True(t, json.Valid(pretty))

	_, err = toPrettyJSON([]byte("x"), "txt")
	require.Error(t, err)
}

func TestUpdateOpenAPIVersion(t *testing.T) {
	out, err := updateOpenAPIVersion([]byte(`{"openapi":"3.1.0","webhooks":{},"jsonSchemaDialect":"https://example.org","$self":"x","info":{"title":"T"}}`), "3.0.3")

	require.NoError(t, err)
	assert.JSONEq(t, `{"openapi":"3.0.3","info":{"title":"T"}}`, string(out))

	_, err = updateOpenAPIVersion([]byte(`{`), "3.1.0")
	require.Error(t, err)
}

func TestRecordFeedEvents(t *testing.T) {
	repo := &recordingRepo{}
	service := NewAPIsAPIService(repo)
	ctx := context.Background()

	service.recordADRScoreChange(ctx, nil, intPtrForServiceTest(80), "api-1")
	service.recordOASHashChange(ctx, "api-1", "old", "new")
	service.recordOASUnavailable(ctx, "api-1", models.OASMetadata{}, models.OASMetadata{Status: models.OASStatusUnreachable})

	require.Len(t, repo.events, 3)
	assert.Equal(t, models.ApiFeedEventADRScoreChanged, repo.events[0].Type)
	assert.Equal(t, models.ApiFeedEventOASHashChanged, repo.events[1].Type)
	assert.Equal(t, models.ApiFeedEventOASUnavailable, repo.events[2].Type)

	service.recordADRScoreChange(ctx, intPtrForServiceTest(80), intPtrForServiceTest(80), "api-1")
	service.recordOASHashChange(ctx, "api-1", "same", " same ")
	service.recordOASUnavailable(ctx, "api-1", models.OASMetadata{Status: models.OASStatusUnreachable}, models.OASMetadata{Status: models.OASStatusUnreachable})
	assert.Len(t, repo.events, 3)
}

type errString string

func (e errString) Error() string { return string(e) }

func intPtrForServiceTest(v int) *int { return &v }

type recordingRepo struct {
	events []models.ApiFeedEvent
}

func (r *recordingRepo) SaveApiFeedEvent(ctx context.Context, event *models.ApiFeedEvent) error {
	r.events = append(r.events, *event)
	return nil
}

func (r *recordingRepo) GetApis(context.Context, int, int, *models.ApiFiltersParams) ([]models.Api, models.Pagination, error) {
	return nil, models.Pagination{}, nil
}
func (r *recordingRepo) SearchApis(context.Context, int, int, *string, string) ([]models.Api, models.Pagination, error) {
	return nil, models.Pagination{}, nil
}
func (r *recordingRepo) GetApiFilterCounts(context.Context, *models.ApiFiltersParams) (*models.ApiFilterCounts, error) {
	return nil, nil
}
func (r *recordingRepo) GetApiByID(context.Context, string) (*models.Api, error) { return nil, nil }
func (r *recordingRepo) GetLintResults(context.Context, string) ([]models.LintResult, error) {
	return nil, nil
}
func (r *recordingRepo) ListLintResults(context.Context) ([]models.LintResult, error) {
	return nil, nil
}
func (r *recordingRepo) FindByOasUrl(context.Context, string) (*models.Api, error) { return nil, nil }
func (r *recordingRepo) FindOrganisationByURI(context.Context, string) (*models.Organisation, error) {
	return nil, nil
}
func (r *recordingRepo) SaveOrganisatie(*models.Organisation) error { return nil }
func (r *recordingRepo) GetOrganisations(context.Context) ([]models.Organisation, int, error) {
	return nil, 0, nil
}
func (r *recordingRepo) Save(*models.Api) error                      { return nil }
func (r *recordingRepo) UpdateApi(context.Context, models.Api) error { return nil }
func (r *recordingRepo) UpdateOASMetadata(context.Context, string, models.OASMetadata) error {
	return nil
}
func (r *recordingRepo) SaveServer(models.Server) error                           { return nil }
func (r *recordingRepo) AllApis(context.Context) ([]models.Api, error)            { return nil, nil }
func (r *recordingRepo) SaveLintResult(context.Context, *models.LintResult) error { return nil }
func (r *recordingRepo) SaveArtifact(context.Context, *models.ApiArtifact) error  { return nil }
func (r *recordingRepo) GetOasArtifact(context.Context, string, string, string) (*models.ApiArtifact, error) {
	return nil, nil
}
func (r *recordingRepo) HasArtifactOfKind(context.Context, string, string) (bool, error) {
	return false, nil
}
func (r *recordingRepo) GetArtifact(context.Context, string, string) (*models.ApiArtifact, error) {
	return nil, nil
}
func (r *recordingRepo) DeleteArtifactsByKind(context.Context, string, string, []string) error {
	return nil
}
func (r *recordingRepo) ListApiFeedEvents(context.Context, string, int) ([]models.ApiFeedEvent, error) {
	return nil, nil
}

func TestRetiredLifecycleDate(t *testing.T) {
	assert.Equal(t, "2026-07-07", retiredLifecycleDate(time.Date(2026, 7, 8, 12, 0, 0, 0, time.UTC)))
}

func TestLintAllApisPersistsLintResultAndUpdatesApi(t *testing.T) {
	createdAt := time.Date(2026, 7, 8, 12, 0, 0, 0, time.UTC)
	toolsServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/oas/bundle":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{
			  "openapi": "3.1.0",
			  "info": {"title": "Linted API", "version": "1.0.0"},
			  "paths": {}
			}`))
		case "/oas/validate":
			require.NoError(t, json.NewEncoder(w).Encode(toolslint.LintResultDTO{
				ID:             "lint-1",
				Successes:      false,
				Failures:       1,
				Warnings:       1,
				Score:          73,
				CreatedAt:      createdAt,
				RulesetVersion: "2026.07",
				Messages: []toolslint.LintMessageDTO{
					{
						ID:        "msg-error",
						Code:      "adr-error",
						Severity:  "error",
						CreatedAt: createdAt,
						Infos: []toolslint.LintMessageInfoDTO{{
							ID:      "info-error",
							Message: "error detail",
							Path:    "$.info",
						}},
					},
					{
						Code:      "adr-warning",
						Severity:  "warning",
						CreatedAt: createdAt,
					},
				},
			}))
		default:
			t.Fatalf("unexpected tools path %s", r.URL.Path)
		}
	}))
	t.Cleanup(toolsServer.Close)
	t.Setenv("TOOLS_API_ENDPOINT", toolsServer.URL)

	prevClient := httpclient.HTTPClient
	httpclient.HTTPClient = toolsServer.Client()
	t.Cleanup(func() { httpclient.HTTPClient = prevClient })

	repo := &lintRecordingRepo{
		api: models.Api{
			Id:      "api-lint",
			OasUri:  "https://example.org/openapi.json",
			OasHash: "old-hash",
		},
	}
	service := NewAPIsAPIService(repo)
	service.limiter = rate.NewLimiter(rate.Inf, 1)

	err := service.LintAllApis(context.Background())

	require.NoError(t, err)
	require.Len(t, repo.savedResults, 1)
	assert.Equal(t, "lint-1", repo.savedResults[0].ID)
	assert.Equal(t, "api-lint", repo.savedResults[0].ApiID)
	require.Len(t, repo.savedResults[0].Messages, 2)
	assert.Equal(t, "2026.07", repo.savedResults[0].Messages[0].RulesetVersion)
	assert.NotEmpty(t, repo.savedResults[0].Messages[1].ID)
	require.NotNil(t, repo.updatedApi.AdrScore)
	assert.Equal(t, 73, *repo.updatedApi.AdrScore)
	assert.NotEqual(t, "old-hash", repo.updatedApi.OasHash)
	require.Len(t, repo.events, 2)
	assert.Equal(t, models.ApiFeedEventADRScoreChanged, repo.events[0].Type)
	assert.Equal(t, models.ApiFeedEventOASHashChanged, repo.events[1].Type)
}

type lintRecordingRepo struct {
	recordingRepo
	api          models.Api
	updatedApi   models.Api
	savedResults []models.LintResult
}

func (r *lintRecordingRepo) AllApis(context.Context) ([]models.Api, error) {
	return []models.Api{r.api}, nil
}

func (r *lintRecordingRepo) GetApiByID(context.Context, string) (*models.Api, error) {
	api := r.api
	return &api, nil
}

func (r *lintRecordingRepo) SaveLintResult(ctx context.Context, result *models.LintResult) error {
	r.savedResults = append(r.savedResults, *result)
	return nil
}

func (r *lintRecordingRepo) UpdateApi(ctx context.Context, api models.Api) error {
	r.updatedApi = api
	r.api = api
	return nil
}

package services

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"testing"

	openapihelper "github.com/developer-overheid-nl/don-api-register/pkg/api_client/helpers/openapi"
	"github.com/developer-overheid-nl/don-api-register/pkg/api_client/helpers/tools"
	"github.com/developer-overheid-nl/don-api-register/pkg/api_client/models"
	"github.com/developer-overheid-nl/don-api-register/pkg/api_client/testutil"
	commonlogging "github.com/developer-overheid-nl/don-register-common/logging"
	"github.com/pb33f/libopenapi"
	v3 "github.com/pb33f/libopenapi/datamodel/high/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type artifactRepoStub struct {
	saved   []*models.ApiArtifact
	apis    []models.Api
	updates []models.Api
}

func (a *artifactRepoStub) GetApis(ctx context.Context, page, perPage int, p *models.ApiFiltersParams, sorting models.ApiSort) ([]models.Api, models.Pagination, error) {
	return nil, models.Pagination{}, nil
}
func (a *artifactRepoStub) SearchApis(ctx context.Context, page, perPage int, organisation *string, query string) ([]models.Api, models.Pagination, error) {
	return nil, models.Pagination{}, nil
}
func (a *artifactRepoStub) GetApiByID(ctx context.Context, id string) (*models.Api, error) {
	return nil, nil
}
func (a *artifactRepoStub) Save(api *models.Api) error { return nil }
func (a *artifactRepoStub) UpdateApi(ctx context.Context, api models.Api) error {
	a.updates = append(a.updates, api)
	return nil
}
func (a *artifactRepoStub) UpdateOASMetadata(ctx context.Context, apiID string, oas models.OASMetadata) error {
	a.updates = append(a.updates, models.Api{Id: apiID, OAS: oas})
	return nil
}
func (a *artifactRepoStub) FindByOasUrl(ctx context.Context, oasUrl string) (*models.Api, error) {
	return nil, nil
}
func (a *artifactRepoStub) SaveServer(server models.Server) error          { return nil }
func (a *artifactRepoStub) SaveOrganisatie(org *models.Organisation) error { return nil }
func (a *artifactRepoStub) AllApis(ctx context.Context) ([]models.Api, error) {
	return a.apis, nil
}
func (a *artifactRepoStub) SaveLintResult(ctx context.Context, res *models.LintResult) error {
	return nil
}
func (a *artifactRepoStub) GetLintResults(ctx context.Context, apiID string) ([]models.LintResult, error) {
	return nil, nil
}
func (a *artifactRepoStub) ListLintResults(ctx context.Context) ([]models.LintResult, error) {
	return nil, nil
}
func (a *artifactRepoStub) GetOrganisations(ctx context.Context) ([]models.Organisation, int, error) {
	return nil, 0, nil
}
func (a *artifactRepoStub) FindOrganisationByURI(ctx context.Context, uri string) (*models.Organisation, error) {
	return nil, nil
}
func (a *artifactRepoStub) SaveArtifact(ctx context.Context, art *models.ApiArtifact) error {
	copy := *art
	copy.Data = append([]byte(nil), art.Data...)
	a.saved = append(a.saved, &copy)
	return nil
}
func (a *artifactRepoStub) GetOasArtifact(ctx context.Context, apiID, version, format string) (*models.ApiArtifact, error) {
	for _, art := range a.saved {
		if art.ApiID == apiID && art.Version == version && art.Format == format {
			return art, nil
		}
	}
	return nil, nil
}
func (a *artifactRepoStub) GetArtifact(ctx context.Context, apiID, kind string) (*models.ApiArtifact, error) {
	return nil, nil
}
func (a *artifactRepoStub) HasArtifactOfKind(ctx context.Context, apiID, kind string) (bool, error) {
	for _, art := range a.saved {
		if art.ApiID == apiID && art.Kind == kind {
			return true, nil
		}
	}
	return false, nil
}
func (a *artifactRepoStub) DeleteArtifactsByKind(ctx context.Context, apiID, kind string, keep []string) error {
	keepSet := make(map[string]struct{}, len(keep))
	for _, id := range keep {
		keepSet[id] = struct{}{}
	}
	filtered := a.saved[:0]
	for _, art := range a.saved {
		if art.ApiID == apiID && art.Kind == kind {
			if _, ok := keepSet[art.ID]; ok {
				filtered = append(filtered, art)
				continue
			}
			continue
		}
		filtered = append(filtered, art)
	}
	a.saved = filtered
	return nil
}
func (a *artifactRepoStub) GetApiFilterCounts(ctx context.Context, p *models.ApiFiltersParams) (*models.ApiFilterCounts, error) {
	return &models.ApiFilterCounts{}, nil
}
func (a *artifactRepoStub) SaveApiFeedEvent(ctx context.Context, event *models.ApiFeedEvent) error {
	return nil
}
func (a *artifactRepoStub) ListApiFeedEvents(ctx context.Context, apiID string, limit int) ([]models.ApiFeedEvent, error) {
	return nil, nil
}

func TestDetachedOASResultDoesNotRetainParsedSpec(t *testing.T) {
	spec := &openapihelper.OASResult{
		Spec:          &v3.Document{},
		Raw:           []byte(`{"openapi":"3.0.3"}`),
		CanonicalJSON: []byte(`{"openapi":"3.0.3","paths":{}}`),
		Hash:          "hash",
	}

	detached := detachedOASResult(spec)

	require.NotNil(t, detached)
	assert.Nil(t, detached.Spec)
	assert.Equal(t, spec.Raw, detached.Raw)
	assert.Equal(t, spec.CanonicalJSON, detached.CanonicalJSON)
	assert.Equal(t, "hash", detached.Hash)
}

func TestRenderCanonicalJSONUsesDetachedCanonicalBytes(t *testing.T) {
	canonical := []byte(`{"openapi":"3.0.3","info":{"title":"Canonical"},"paths":{}}`)
	res := &openapihelper.OASResult{
		Raw:           []byte(`{"openapi":"3.0.3","info":{"title":"Raw"},"paths":{}}`),
		CanonicalJSON: canonical,
	}

	got, err := renderCanonicalJSON(res, "json")
	require.NoError(t, err)
	assert.JSONEq(t, string(canonical), string(got))
}

func TestRefreshChangedApisLogsStructuredProgress(t *testing.T) {
	t.Setenv("TOOLS_API_ENDPOINT", "")
	spec := `{"openapi":"3.0.3","info":{"title":"Progress","version":"1.0.0"},"paths":{}}`
	server := testutil.NewTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(spec))
	}))

	parsed, err := openapihelper.FetchParseValidateAndHash(
		context.Background(),
		tools.OASInput{OasUrl: server.URL},
		openapihelper.FetchOpts{},
	)
	require.NoError(t, err)
	libopenapi.ClearAllCaches()

	repo := &artifactRepoStub{}
	for index := 1; index <= 25; index++ {
		repo.apis = append(repo.apis, models.Api{
			Id:      fmt.Sprintf("api-%02d", index),
			OasUri:  server.URL,
			OasHash: parsed.Hash,
			OAS:     models.OASMetadata{Version: "3.0.3", Status: models.OASStatusValid},
		})
	}
	service := NewAPIsAPIService(repo)

	var logBuffer bytes.Buffer
	logger, err := commonlogging.NewJSONLogger(&logBuffer, "api-register", "debug")
	require.NoError(t, err)
	previousLogger := slog.Default()
	slog.SetDefault(logger)
	t.Cleanup(func() { slog.SetDefault(previousLogger) })

	result, err := service.RefreshChangedApis(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 25, result.CandidateCount)
	assert.Equal(t, 25, result.ProcessedCount)
	assert.Zero(t, result.UpdatedCount)
	assert.Zero(t, result.FailedCount)

	var startRecord, progressRecord map[string]any
	for _, line := range strings.Split(strings.TrimSpace(logBuffer.String()), "\n") {
		var record map[string]any
		require.NoError(t, json.Unmarshal([]byte(line), &record))
		switch record["msg"] {
		case "OAS refresh started":
			startRecord = record
		case "OAS refresh progress":
			progressRecord = record
		}
	}
	require.NotNil(t, startRecord)
	require.NotNil(t, progressRecord)
	assert.Equal(t, "api-register", progressRecord["app"])
	assert.Equal(t, "oas_refresh", progressRecord["component"])
	assert.Equal(t, "run", progressRecord["operation"])
	assert.Equal(t, float64(25), progressRecord["candidate_count"])
	assert.Equal(t, float64(25), progressRecord["processed_count"])
	assert.Greater(t, progressRecord["heap_alloc_bytes"].(float64), float64(0))
}

func TestPersistOASArtifacts_StoresOriginalAndConverted(t *testing.T) {
	repo := &artifactRepoStub{}
	service := NewAPIsAPIService(repo)

	raw := []byte(`openapi: 3.0.3
info:
  title: Demo
  version: "1.0"
paths: {}
`)

	doc, err := libopenapi.NewDocument(raw)
	require.NoError(t, err)
	model, err := doc.BuildV3Model()
	require.NoError(t, err)
	spec := model.Model
	sum := sha256.Sum256(raw)

	res := &openapihelper.OASResult{
		Spec:        &spec,
		Hash:        hex.EncodeToString(sum[:]),
		Raw:         raw,
		ContentType: "application/yaml",
		Version:     "3.0.3",
		Major:       3,
		Minor:       0,
		Patch:       3,
	}

	err = service.persistOASArtifacts(context.Background(), "api-1", res)
	require.NoError(t, err)
	require.Len(t, repo.saved, 4)

	artifacts := map[string]*models.ApiArtifact{}
	for _, art := range repo.saved {
		key := art.Version + "-" + art.Format
		artifacts[key] = art
	}

	require.Contains(t, artifacts, "3.0-yaml")
	require.Contains(t, artifacts, "3.0-json")
	require.Contains(t, artifacts, "3.1-json")
	require.Contains(t, artifacts, "3.1-yaml")

	assert.Equal(t, "original", artifacts["3.0-yaml"].Source)
	assert.Equal(t, "converted", artifacts["3.0-json"].Source)
	assert.Equal(t, "converted", artifacts["3.1-json"].Source)
	assert.Equal(t, "converted", artifacts["3.1-yaml"].Source)

	assert.Equal(t, "application/yaml", artifacts["3.0-yaml"].ContentType)
	assert.Equal(t, "application/json", artifacts["3.0-json"].ContentType)
	assert.Equal(t, "application/json", artifacts["3.1-json"].ContentType)
	assert.Equal(t, "application/yaml", artifacts["3.1-yaml"].ContentType)
}

func TestPersistOASArtifacts_AcceptsJSONOriginal(t *testing.T) {
	repo := &artifactRepoStub{}
	service := NewAPIsAPIService(repo)

	raw := []byte(`{
  "openapi": "3.1.1",
  "info": {
    "title": "Demo JSON",
    "version": "2.0"
  },
  "paths": {}
}`)

	doc, err := libopenapi.NewDocument(raw)
	require.NoError(t, err)
	model, err := doc.BuildV3Model()
	require.NoError(t, err)
	spec := model.Model
	sum := sha256.Sum256(raw)

	res := &openapihelper.OASResult{
		Spec:        &spec,
		Hash:        hex.EncodeToString(sum[:]),
		Raw:         raw,
		ContentType: "application/json",
		Version:     "3.1.1",
		Major:       3,
		Minor:       1,
		Patch:       1,
	}

	err = service.persistOASArtifacts(context.Background(), "api-json", res)
	require.NoError(t, err)
	require.Len(t, repo.saved, 4)

	artifacts := map[string]*models.ApiArtifact{}
	for _, art := range repo.saved {
		key := art.Version + "-" + art.Format
		artifacts[key] = art
	}

	require.Contains(t, artifacts, "3.1-json")
	require.Contains(t, artifacts, "3.1-yaml")
	require.Contains(t, artifacts, "3.0-json")
	require.Contains(t, artifacts, "3.0-yaml")

	assert.Equal(t, "original", artifacts["3.1-json"].Source)
	assert.Equal(t, "converted", artifacts["3.1-yaml"].Source)
	assert.Equal(t, "converted", artifacts["3.0-json"].Source)
	assert.Equal(t, "converted", artifacts["3.0-yaml"].Source)

	assert.Equal(t, "application/json", artifacts["3.1-json"].ContentType)
	assert.Equal(t, "application/yaml", artifacts["3.1-yaml"].ContentType)
	assert.Equal(t, "application/json", artifacts["3.0-json"].ContentType)
	assert.Equal(t, "application/yaml", artifacts["3.0-yaml"].ContentType)
}

func TestUpdateOpenAPIVersion_UsesTopLevelRawMessages(t *testing.T) {
	out, err := updateOpenAPIVersion([]byte(`{
  "openapi": "3.1.0",
  "jsonSchemaDialect": "https://spec.openapis.org/oas/3.1/dialect/base",
  "$self": "https://example.com/openapi.json",
  "webhooks": {
    "changed": {}
  },
  "info": {
    "title": "Demo",
    "version": "1.0"
  },
  "paths": {
    "/items": {
      "get": {
        "responses": {
          "200": {
            "description": "ok"
          }
        }
      }
    }
  }
}`), "3.0.3")
	require.NoError(t, err)

	var doc map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(out, &doc))

	var version string
	require.NoError(t, json.Unmarshal(doc["openapi"], &version))
	assert.Equal(t, "3.0.3", version)
	assert.NotContains(t, doc, "jsonSchemaDialect")
	assert.NotContains(t, doc, "$self")
	assert.NotContains(t, doc, "webhooks")
	assert.Contains(t, doc, "paths")
}

func TestBackfillOASArtifacts_GeneratesWhenMissing(t *testing.T) {
	srv := testutil.NewTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/yaml")
		_, err := fmt.Fprint(w, `openapi: 3.1.0
info:
  title: Demo
  version: "1.0"
paths: {}
`)
		require.NoError(t, err)
	}))

	repo := &artifactRepoStub{
		apis: []models.Api{
			{Id: "api-1", OasUri: srv.URL},
		},
	}

	service := NewAPIsAPIService(repo)
	err := service.BackfillOASArtifacts(context.Background())
	require.NoError(t, err)

	require.NotEmpty(t, repo.saved)
	has, err := repo.HasArtifactOfKind(context.Background(), "api-1", "oas")
	require.NoError(t, err)
	assert.True(t, has)
	assert.NotEmpty(t, repo.updates)
	assert.NotEmpty(t, repo.updates[0].OasHash)
}

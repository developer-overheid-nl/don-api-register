package services

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"testing"

	openapihelper "github.com/developer-overheid-nl/don-api-register/pkg/api_client/helpers/openapi"
	"github.com/developer-overheid-nl/don-api-register/pkg/api_client/models"
	"github.com/developer-overheid-nl/don-api-register/pkg/api_client/testutil"
	"github.com/pb33f/libopenapi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type artifactRepoStub struct {
	saved   []*models.ApiArtifact
	apis    []models.Api
	updates []models.Api
}

func (a *artifactRepoStub) GetApis(ctx context.Context, page, perPage int, organisation *string, ids *string) ([]models.Api, models.Pagination, error) {
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

func TestBackfillOASArtifacts_GeneratesWhenMissing(t *testing.T) {
	srv := testutil.NewTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/yaml")
		fmt.Fprint(w, `openapi: 3.1.0
info:
  title: Demo
  version: "1.0"
paths: {}
`)
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

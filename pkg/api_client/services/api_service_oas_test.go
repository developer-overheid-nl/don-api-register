package services

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"testing"

	openapihelper "github.com/developer-overheid-nl/don-api-register/pkg/api_client/helpers/openapi"
	"github.com/developer-overheid-nl/don-api-register/pkg/api_client/models"
	"github.com/pb33f/libopenapi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type artifactRepoStub struct {
	saved []*models.ApiArtifact
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
func (a *artifactRepoStub) Save(api *models.Api) error                          { return nil }
func (a *artifactRepoStub) UpdateApi(ctx context.Context, api models.Api) error { return nil }
func (a *artifactRepoStub) FindByOasUrl(ctx context.Context, oasUrl string) (*models.Api, error) {
	return nil, nil
}
func (a *artifactRepoStub) SaveServer(server models.Server) error             { return nil }
func (a *artifactRepoStub) SaveOrganisatie(org *models.Organisation) error    { return nil }
func (a *artifactRepoStub) AllApis(ctx context.Context) ([]models.Api, error) { return nil, nil }
func (a *artifactRepoStub) SaveLintResult(ctx context.Context, res *models.LintResult) error {
	return nil
}
func (a *artifactRepoStub) GetLintResults(ctx context.Context, apiID string) ([]models.LintResult, error) {
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

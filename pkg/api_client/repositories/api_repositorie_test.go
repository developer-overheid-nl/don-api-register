package repositories_test

import (
	"context"
	"testing"
	"time"

	"github.com/developer-overheid-nl/don-api-register/pkg/api_client/models"
	"github.com/developer-overheid-nl/don-api-register/pkg/api_client/repositories"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func intPtr(v int) *int { return &v }

func setupDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&models.Api{},
		&models.Organisation{},
		&models.Server{},
		&models.ApiArtifact{},
		&models.LintResult{},
		&models.LintMessage{},
		&models.LintMessageInfo{},
	))
	return db
}

func TestApiRepository_SaveAndGet(t *testing.T) {
	db := setupDB(t)
	repo := repositories.NewApiRepository(db)
	orgURI := "org1"
	api := &models.Api{Id: "a1", OasUri: "u1", ContactName: "c", ContactEmail: "e", ContactUrl: "url", Organisation: &models.Organisation{Uri: orgURI, Label: "L"}, OrganisationID: &orgURI}
	err := repo.Save(api)
	require.NoError(t, err)

	got, err := repo.GetApiByID(context.Background(), api.Id)
	require.NoError(t, err)
	assert.Equal(t, "u1", got.OasUri)
}

func TestApiRepository_FindByOasUrl(t *testing.T) {
	db := setupDB(t)
	repo := repositories.NewApiRepository(db)
	orgURI := "org1"
	api := &models.Api{Id: "a1", OasUri: "u1", ContactName: "c", ContactEmail: "e", ContactUrl: "url", Organisation: &models.Organisation{Uri: orgURI, Label: "L"}, OrganisationID: &orgURI}
	require.NoError(t, repo.Save(api))

	got, err := repo.FindByOasUrl(context.Background(), "u1")
	require.NoError(t, err)
	assert.NotNil(t, got)
	assert.Equal(t, api.Id, got.Id)
}

func TestApiRepository_GetApisAppliesFilters(t *testing.T) {
	db := setupDB(t)
	repo := repositories.NewApiRepository(db)
	ctx := context.Background()
	orgURI := "org1"
	require.NoError(t, db.Create(&models.Organisation{Uri: orgURI, Label: "Org 1"}).Error)

	apis := []models.Api{
		{
			Id:             "active-api",
			OasUri:         "https://example.com/active.yaml",
			Title:          "Active API",
			Version:        "1.0.0",
			Auth:           "api_key",
			AdrScore:       intPtr(88),
			OrganisationID: &orgURI,
		},
		{
			Id:             "deprecated-api",
			OasUri:         "https://example.com/deprecated.yaml",
			Title:          "Deprecated API",
			Version:        "2.0.0",
			Auth:           "oauth2",
			AdrScore:       nil,
			Deprecated:     time.Now().AddDate(0, 0, -1).Format(time.DateOnly),
			OrganisationID: &orgURI,
		},
	}
	require.NoError(t, db.Create(&apis).Error)
	require.NoError(t, db.Create([]models.ApiArtifact{
		{ID: "art-1", ApiID: "active-api", Kind: "oas", Source: "original", Data: []byte(`{"openapi":"3.1.0"}`)},
		{ID: "art-2", ApiID: "deprecated-api", Kind: "oas", Source: "original", Data: []byte(`{"openapi":"3.0.0"}`)},
	}).Error)

	results, pagination, err := repo.GetApis(ctx, 1, 10, &models.ApiFiltersParams{
		Status:     []string{"deprecated"},
		OasVersion: []string{"3.0.0"},
		Auth:       []string{"oauth2"},
		AdrScore:   []string{"unknown"},
	})
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, "deprecated-api", results[0].Id)
	assert.Equal(t, 1, pagination.TotalRecords)
}

func TestApiRepository_GetApiFilterCountsRespectOtherFilters(t *testing.T) {
	db := setupDB(t)
	repo := repositories.NewApiRepository(db)
	ctx := context.Background()
	orgURI := "org1"
	require.NoError(t, db.Create(&models.Organisation{Uri: orgURI, Label: "Org 1"}).Error)

	apis := []models.Api{
		{
			Id:             "api-key-api",
			OasUri:         "https://example.com/api-key.yaml",
			Title:          "API key API",
			Version:        "1.0.0",
			Auth:           "api_key",
			AdrScore:       intPtr(88),
			OrganisationID: &orgURI,
		},
		{
			Id:             "oauth-api",
			OasUri:         "https://example.com/oauth.yaml",
			Title:          "OAuth API",
			Version:        "2.0.0",
			Auth:           "oauth2",
			AdrScore:       intPtr(42),
			Deprecated:     time.Now().AddDate(0, 0, -1).Format(time.DateOnly),
			OrganisationID: &orgURI,
		},
	}
	require.NoError(t, db.Create(&apis).Error)
	require.NoError(t, db.Create([]models.ApiArtifact{
		{ID: "art-3", ApiID: "api-key-api", Kind: "oas", Source: "original", Data: []byte(`{"openapi":"3.1.0"}`)},
		{ID: "art-4", ApiID: "oauth-api", Kind: "oas", Source: "original", Data: []byte(`{"openapi":"3.0.0"}`)},
	}).Error)

	counts, err := repo.GetApiFilterCounts(ctx, &models.ApiFiltersParams{Auth: []string{"oauth2"}})
	require.NoError(t, err)

	statusCounts := map[string]int{}
	for _, fc := range counts.Status {
		statusCounts[fc.Value] = fc.Count
	}
	versionCounts := map[string]int{}
	for _, fc := range counts.OasVersion {
		versionCounts[fc.Value] = fc.Count
	}
	authCounts := map[string]int{}
	for _, fc := range counts.Auth {
		authCounts[fc.Value] = fc.Count
	}

	assert.Equal(t, 1, statusCounts["deprecated"])
	assert.Equal(t, 1, versionCounts["3.0.0"])
	assert.Equal(t, 1, authCounts["api_key"])
	assert.Equal(t, 1, authCounts["oauth2"])
}

func TestApiRepository_GetApiFilterCounts_SortsByCountThenAlphabetically(t *testing.T) {
	db := setupDB(t)
	repo := repositories.NewApiRepository(db)
	ctx := context.Background()
	orgURI := "org1"
	require.NoError(t, db.Create(&models.Organisation{Uri: orgURI, Label: "Org 1"}).Error)

	apis := []models.Api{
		{Id: "api-1", OasUri: "https://example.com/1.yaml", Title: "API 1", OrganisationID: &orgURI},
		{Id: "api-2", OasUri: "https://example.com/2.yaml", Title: "API 2", OrganisationID: &orgURI},
		{Id: "api-3", OasUri: "https://example.com/3.yaml", Title: "API 3", OrganisationID: &orgURI},
		{Id: "api-4", OasUri: "https://example.com/4.yaml", Title: "API 4", OrganisationID: &orgURI},
	}
	require.NoError(t, db.Create(&apis).Error)
	require.NoError(t, db.Create([]models.ApiArtifact{
		{ID: "art-5", ApiID: "api-1", Kind: "oas", Source: "original", Data: []byte(`{"openapi":"3.0.0"}`)},
		{ID: "art-6", ApiID: "api-2", Kind: "oas", Source: "original", Data: []byte(`{"openapi":"3.0.0"}`)},
		{ID: "art-7", ApiID: "api-3", Kind: "oas", Source: "original", Data: []byte(`{"openapi":"3.1.0"}`)},
		{ID: "art-8", ApiID: "api-4", Kind: "oas", Source: "original", Data: []byte(`{"openapi":"3.0.1"}`)},
	}).Error)

	counts, err := repo.GetApiFilterCounts(ctx, &models.ApiFiltersParams{})
	require.NoError(t, err)
	require.Len(t, counts.OasVersion, 3)

	assert.Equal(t, "3.0.0", counts.OasVersion[0].Value)
	assert.Equal(t, 2, counts.OasVersion[0].Count)
	assert.Equal(t, "3.0.1", counts.OasVersion[1].Value)
	assert.Equal(t, "3.1.0", counts.OasVersion[2].Value)
}

func TestApiRepository_GetApiFilterCounts_SortsOrganisationsAlphabetically(t *testing.T) {
	db := setupDB(t)
	repo := repositories.NewApiRepository(db)
	ctx := context.Background()

	orgA := models.Organisation{Uri: "https://example.com/org-a", Label: "Alpha org"}
	orgB := models.Organisation{Uri: "https://example.com/org-b", Label: "Beta org"}
	orgZ := models.Organisation{Uri: "https://example.com/org-z", Label: "Zeta org"}
	require.NoError(t, db.Create([]models.Organisation{orgZ, orgA, orgB}).Error)

	apis := []models.Api{
		{Id: "api-1", OasUri: "https://example.com/a1.yaml", Title: "API 1", OrganisationID: &orgZ.Uri},
		{Id: "api-2", OasUri: "https://example.com/a2.yaml", Title: "API 2", OrganisationID: &orgZ.Uri},
		{Id: "api-3", OasUri: "https://example.com/a3.yaml", Title: "API 3", OrganisationID: &orgB.Uri},
		{Id: "api-4", OasUri: "https://example.com/a4.yaml", Title: "API 4", OrganisationID: &orgA.Uri},
	}
	require.NoError(t, db.Create(&apis).Error)

	selectedOrganisation := orgB.Uri
	counts, err := repo.GetApiFilterCounts(ctx, &models.ApiFiltersParams{Organisation: &selectedOrganisation})
	require.NoError(t, err)
	require.Len(t, counts.Organisation, 3)

	assert.Equal(t, "Alpha org", counts.Organisation[0].Label)
	assert.Equal(t, orgA.Uri, counts.Organisation[0].Value)
	assert.Equal(t, "Beta org", counts.Organisation[1].Label)
	assert.Equal(t, orgB.Uri, counts.Organisation[1].Value)
	assert.Equal(t, "Zeta org", counts.Organisation[2].Label)
	assert.Equal(t, orgZ.Uri, counts.Organisation[2].Value)
	assert.Equal(t, 2, counts.Organisation[2].Count)
}

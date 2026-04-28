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
			OAS:            models.OASMetadata{Version: "3.1.0"},
			Title:          "Active API",
			Version:        "1.0.0",
			Auth:           "api_key",
			AdrScore:       intPtr(88),
			OrganisationID: &orgURI,
		},
		{
			Id:             "deprecated-api",
			OasUri:         "https://example.com/deprecated.yaml",
			OAS:            models.OASMetadata{Version: "3.0.0"},
			Title:          "Deprecated API",
			Version:        "2.0.0",
			Auth:           "oauth2",
			AdrScore:       nil,
			Deprecated:     time.Now().AddDate(0, 0, -1).Format(time.DateOnly),
			OrganisationID: &orgURI,
		},
	}
	require.NoError(t, db.Create(&apis).Error)

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
			OAS:            models.OASMetadata{Version: "3.1.0"},
			Title:          "API key API",
			Version:        "1.0.0",
			Auth:           "api_key",
			AdrScore:       intPtr(88),
			OrganisationID: &orgURI,
		},
		{
			Id:             "oauth-api",
			OasUri:         "https://example.com/oauth.yaml",
			OAS:            models.OASMetadata{Version: "3.0.0"},
			Title:          "OAuth API",
			Version:        "2.0.0",
			Auth:           "oauth2",
			AdrScore:       intPtr(42),
			Deprecated:     time.Now().AddDate(0, 0, -1).Format(time.DateOnly),
			OrganisationID: &orgURI,
		},
	}
	require.NoError(t, db.Create(&apis).Error)

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
		{Id: "api-1", OasUri: "https://example.com/1.yaml", OAS: models.OASMetadata{Version: "3.0.0"}, Title: "API 1", OrganisationID: &orgURI},
		{Id: "api-2", OasUri: "https://example.com/2.yaml", OAS: models.OASMetadata{Version: "3.0.0"}, Title: "API 2", OrganisationID: &orgURI},
		{Id: "api-3", OasUri: "https://example.com/3.yaml", OAS: models.OASMetadata{Version: "3.1.0"}, Title: "API 3", OrganisationID: &orgURI},
		{Id: "api-4", OasUri: "https://example.com/4.yaml", OAS: models.OASMetadata{Version: "3.0.1"}, Title: "API 4", OrganisationID: &orgURI},
	}
	require.NoError(t, db.Create(&apis).Error)

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

func TestApiRepository_SaveLintResult_PersistsMessageRulesetVersion(t *testing.T) {
	db := setupDB(t)
	repo := repositories.NewApiRepository(db)
	ctx := context.Background()

	result := &models.LintResult{
		ID:        "lint-result-1",
		ApiID:     "api-1",
		Successes: true,
		Messages: []models.LintMessage{
			{
				ID:             "lint-message-1",
				Severity:       "warning",
				Code:           "adr-001",
				RulesetVersion: "2026.04",
				CreatedAt:      time.Now(),
			},
		},
		CreatedAt: time.Now(),
	}

	require.NoError(t, repo.SaveLintResult(ctx, result))

	stored, err := repo.GetLintResults(ctx, "api-1")
	require.NoError(t, err)
	require.Len(t, stored, 1)
	require.Len(t, stored[0].Messages, 1)
	assert.Equal(t, "2026.04", stored[0].Messages[0].RulesetVersion)
}

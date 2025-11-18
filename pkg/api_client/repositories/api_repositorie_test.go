package repositories_test

import (
	"context"
	"testing"

	"github.com/developer-overheid-nl/don-api-register/pkg/api_client/models"
	"github.com/developer-overheid-nl/don-api-register/pkg/api_client/repositories"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&models.Api{},
		&models.Organisation{},
		&models.Server{},
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

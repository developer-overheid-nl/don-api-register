package services_test

import (
	"context"
	"fmt"
	"github.com/developer-overheid-nl/don-api-register/pkg/api_client/models"
	"github.com/developer-overheid-nl/don-api-register/pkg/api_client/services"
	"github.com/stretchr/testify/assert"
	"gorm.io/gorm"
	"net/http"
	"net/http/httptest"
	"testing"
)

// stubRepo implements repositories.ApiRepository for testing
type stubRepo struct {
	findByOas  func(ctx context.Context, oasUrl string) (*models.Api, error)
	getByID    func(ctx context.Context, id string) (*models.Api, error)
	getLintRes func(ctx context.Context, apiID string) ([]models.LintResult, error)
	getApis    func(ctx context.Context, page, perPage int) ([]models.Api, models.Pagination, error)
	saveServer func(server models.Server) error
	saveApi    func(api *models.Api) error
}

func (s *stubRepo) FindByOasUrl(ctx context.Context, url string) (*models.Api, error) {
	return s.findByOas(ctx, url)
}
func (s *stubRepo) GetApiByID(ctx context.Context, id string) (*models.Api, error) {
	return s.getByID(ctx, id)
}
func (s *stubRepo) GetLintResults(ctx context.Context, apiID string) ([]models.LintResult, error) {
	return s.getLintRes(ctx, apiID)
}
func (s *stubRepo) GetApis(ctx context.Context, page, perPage int) ([]models.Api, models.Pagination, error) {
	return s.getApis(ctx, page, perPage)
}

// unused methods
func (s *stubRepo) SaveServer(server models.Server) error                               { return s.saveServer(server) }
func (s *stubRepo) Save(api *models.Api) error                                          { return s.saveApi(api) }
func (s *stubRepo) UpdateApi(ctx context.Context, api models.Api) error                 { return nil }
func (s *stubRepo) SaveOrganisatie(org *models.Organisation) error                      { return nil }
func (s *stubRepo) AllApis(ctx context.Context) ([]models.Api, error)                   { return nil, nil }
func (s *stubRepo) SaveLintResult(ctx context.Context, result *models.LintResult) error { return nil }

func TestUpdateOasUri_NotFound(t *testing.T) {
	repo := &stubRepo{
		findByOas: func(ctx context.Context, url string) (*models.Api, error) {
			return nil, gorm.ErrRecordNotFound
		},
	}
	service := services.NewAPIsAPIService(repo)

	input := &models.UpdateApiInput{
		Id:              "missing-id",
		OasUrl:          "https://niet-bestaand.nl/openapi.json",
		OrganisationUri: "https://identifier.overheid.nl/tooi/id/xxx",
		Contact:         models.Contact{}, // vul verder aan als nodig
	}

	result, err := service.UpdateOasUri(context.Background(), input)

	assert.Nil(t, result)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), services.ErrNeedsPost.Error())
}

func TestRetrieveApi_Success(t *testing.T) {
	api := &models.Api{
		Id: "1234",
		Organisation: &models.Organisation{
			Label: "Test Org",
			Uri:   "https://org.example.com",
		},
	}
	repo := &stubRepo{
		getByID: func(ctx context.Context, id string) (*models.Api, error) {
			return api, nil
		},
	}
	service := services.NewAPIsAPIService(repo)
	resp, err := service.RetrieveApi(context.Background(), "1234")
	assert.NoError(t, err)
	assert.Equal(t, api.Id, resp.Id)
}

func TestListApis_Pagination(t *testing.T) {
	apis := []models.Api{
		{
			Id:     "a1",
			Title:  "First",
			OasUri: "u1",
			Organisation: &models.Organisation{
				Uri:   "https://org1.test",
				Label: "Org 1",
			},
		},
		{
			Id:     "a2",
			Title:  "Second",
			OasUri: "u2",
			Organisation: &models.Organisation{
				Uri:   "https://org2.test",
				Label: "Org 2",
			},
		},
	}

	pagination := models.Pagination{CurrentPage: 1, RecordsPerPage: 2, TotalPages: 1, TotalRecords: 2}
	repo := &stubRepo{
		getApis: func(ctx context.Context, page, perPage int) ([]models.Api, models.Pagination, error) {
			return apis, pagination, nil
		},
	}
	service := services.NewAPIsAPIService(repo)
	baseURL := "http://example.com/apIs"
	res, err := service.ListApis(context.Background(), 1, 2, baseURL)
	assert.NoError(t, err)
	assert.Len(t, res.Apis, 2)
	assert.Equal(t, fmt.Sprintf("%s?page=1&perPage=2", baseURL), res.Links.Self.Href)
}

func TestCreateApiFromOas_Success(t *testing.T) {
	spec := `{
  "openapi": "3.0.0",
  "info": {
    "title": "T",
    "version": "1.0.0",
    "contact": {
      "name": "Testpersoon",
      "email": "test@example.com",
      "url": "https://example.com"
    }
  },
  "paths": {}
}`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, err := w.Write([]byte(spec))
		if err != nil {
			return
		}
	}))
	defer server.Close()

	// stub repo
	var saved models.Api
	repo := &stubRepo{
		saveServer: func(server models.Server) error { return nil },
		saveApi:    func(api *models.Api) error { saved = *api; return nil },
	}

	service := services.NewAPIsAPIService(repo)
	apiReq := models.ApiPost{
		OasUrl:          server.URL,
		OrganisationUri: "https://example.com",
	}
	resp, err := service.CreateApiFromOas(apiReq)
	assert.NoError(t, err)
	assert.Equal(t, saved.Id, resp.Id)
	assert.Equal(t, "T", resp.Title)
}

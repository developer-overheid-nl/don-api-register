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
	err := service.UpdateOasUri(context.Background(), "missing-url")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), services.ErrNeedsPost.Error())
}

func TestRetrieveApi_Success(t *testing.T) {
	api := &models.Api{Id: "1234"}
	repo := &stubRepo{
		getByID: func(ctx context.Context, id string) (*models.Api, error) {
			return api, nil
		},
		getLintRes: func(ctx context.Context, apiID string) ([]models.LintResult, error) {
			return []models.LintResult{{ID: "lr1", ApiID: apiID}}, nil
		},
	}
	service := services.NewAPIsAPIService(repo)
	resp, err := service.RetrieveApi(context.Background(), "1234")
	assert.NoError(t, err)
	assert.Equal(t, api, resp.Api)
	assert.Len(t, resp.LintResults, 1)
}

func TestListApis_Pagination(t *testing.T) {
	apis := []models.Api{
		{Id: "a1", Title: "First", OasUri: "u1"},
		{Id: "a2", Title: "Second", OasUri: "u2"},
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
	// minimal OpenAPI JSON
	spec := `{"openapi":"3.0.0","info":{"title":"T","version":"1.0.0"},"paths":{}}`
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
	apiReq := models.Api{
		OasUri:       server.URL,
		ContactName:  "Tester",
		ContactUrl:   "https://example.com",
		ContactEmail: "test@example.com",
	}
	resp, err := service.CreateApiFromOas(apiReq)
	assert.NoError(t, err)
	assert.Equal(t, saved.Id, resp.Id)
	assert.Equal(t, "T", resp.Title)
}

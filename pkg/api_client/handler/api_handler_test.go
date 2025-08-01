package handler

import (
	"context"
	"net/http/httptest"
	"testing"

	"github.com/developer-overheid-nl/don-api-register/pkg/api_client/models"
	"github.com/developer-overheid-nl/don-api-register/pkg/api_client/services"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

// stubRepo mocks ApiRepository for controller tests
type stubRepo struct {
	listFunc    func(ctx context.Context, page, perPage int) ([]models.Api, models.Pagination, error)
	retrFunc    func(ctx context.Context, id string) (*models.Api, error)
	lintResFunc func(ctx context.Context, apiID string) ([]models.LintResult, error)
	findOasFunc func(ctx context.Context, oasUrl string) (*models.Api, error)
}

func (s *stubRepo) GetApis(ctx context.Context, page, perPage int) ([]models.Api, models.Pagination, error) {
	return s.listFunc(ctx, page, perPage)
}
func (s *stubRepo) GetApiByID(ctx context.Context, id string) (*models.Api, error) {
	return s.retrFunc(ctx, id)
}
func (s *stubRepo) GetLintResults(ctx context.Context, apiID string) ([]models.LintResult, error) {
	return s.lintResFunc(ctx, apiID)
}
func (s *stubRepo) FindByOasUrl(ctx context.Context, oasUrl string) (*models.Api, error) {
	return s.findOasFunc(ctx, oasUrl)
}

// unused
func (s *stubRepo) Save(api *models.Api) error                                       { return nil }
func (s *stubRepo) UpdateApi(ctx context.Context, api models.Api) error              { return nil }
func (s *stubRepo) SaveServer(server models.Server) error                            { return nil }
func (s *stubRepo) SaveOrganisatie(org *models.Organisation) error                   { return nil }
func (s *stubRepo) AllApis(ctx context.Context) ([]models.Api, error)                { return nil, nil }
func (s *stubRepo) SaveLintResult(ctx context.Context, res *models.LintResult) error { return nil }

func TestListApis_Handler(t *testing.T) {
	repo := &stubRepo{
		listFunc: func(ctx context.Context, page, perPage int) ([]models.Api, models.Pagination, error) {
			apis := []models.Api{
				{
					Id:           "a1",
					Organisation: &models.Organisation{Uri: "org1", Label: "Label 1"},
					Servers:      []models.Server{},
				},
				{
					Id:           "a2",
					Organisation: &models.Organisation{Uri: "org2", Label: "Label 2"},
					Servers:      []models.Server{},
				},
			}
			pag := models.Pagination{CurrentPage: page, RecordsPerPage: perPage}
			return apis, pag, nil
		},
	}
	svc := services.NewAPIsAPIService(repo)
	ctrl := NewAPIsAPIController(svc)

	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)
	req := httptest.NewRequest("GET", "/v1/apis?page=3&perPage=7", nil)
	req.Host = "host"
	ctx.Request = req
	ctx.Set("FullPath", "/v1/apis")

	resp, err := ctrl.ListApis(ctx, &listApisParams{Page: 3, PerPage: 7})
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, "https://host?page=3&perPage=7", resp.Links.Self.Href)
	assert.Len(t, resp.Apis, 2)
}

func TestRetrieveApi_Handler(t *testing.T) {
	// success case
	repo1 := &stubRepo{
		retrFunc: func(ctx context.Context, id string) (*models.Api, error) {
			return &models.Api{
				Id:           id,
				Organisation: &models.Organisation{Uri: "dummy", Label: "dummy"},
				Servers:      []models.Server{},
			}, nil
		},
		lintResFunc: func(ctx context.Context, apiID string) ([]models.LintResult, error) {
			return []models.LintResult{{ID: "lr1", ApiID: apiID}}, nil
		},
	}
	svc1 := services.NewAPIsAPIService(repo1)
	ctrl1 := NewAPIsAPIController(svc1)

	w := httptest.NewRecorder()
	ctx1, _ := gin.CreateTestContext(w)
	req1 := httptest.NewRequest("GET", "/v1/apis/id1", nil)
	req1.Host = "host"
	ctx1.Request = req1

	resp1, err1 := ctrl1.RetrieveApi(ctx1, &models.ApiParams{Id: "id1"})
	assert.NoError(t, err1)
	assert.NotNil(t, resp1)
	assert.Equal(t, "id1", resp1.Id)

	// not found case
	repo2 := &stubRepo{
		retrFunc:    func(ctx context.Context, id string) (*models.Api, error) { return nil, nil },
		lintResFunc: func(ctx context.Context, apiID string) ([]models.LintResult, error) { return nil, nil },
	}
	svc2 := services.NewAPIsAPIService(repo2)
	ctrl2 := NewAPIsAPIController(svc2)

	ctx2, _ := gin.CreateTestContext(w)
	req2 := httptest.NewRequest("GET", "/v1/apis/missing", nil)
	req2.Host = "host"
	ctx2.Request = req2

	resp2, err2 := ctrl2.RetrieveApi(ctx2, &models.ApiParams{Id: "missing"})
	assert.Error(t, err2)
	assert.Nil(t, resp2)
}

func TestCreateApiFromOas_Handler(t *testing.T) {
	repo := &stubRepo{
		findOasFunc: func(ctx context.Context, url string) (*models.Api, error) { return nil, nil },
	}
	svc := services.NewAPIsAPIService(repo)
	ctrl := NewAPIsAPIController(svc)

	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)
	body := &models.ApiPost{OasUrl: "u", OrganisationUri: "https://example.org"}
	resp, err := ctrl.CreateApiFromOas(ctx, body)
	assert.Nil(t, resp)
	assert.Error(t, err)
}

func TestUpdateApi_Handler(t *testing.T) {
	// needs post (geen bestaande API, moet fout geven)
	repo1 := &stubRepo{
		findOasFunc: func(ctx context.Context, url string) (*models.Api, error) { return nil, nil },
	}
	svc1 := services.NewAPIsAPIService(repo1)
	ctrl1 := NewAPIsAPIController(svc1)

	w := httptest.NewRecorder()
	ctx1, _ := gin.CreateTestContext(w)
	ctx1.Request = httptest.NewRequest("PUT", "/v1/apis", nil)

	input := &models.UpdateApiInput{OasUrl: "u", OrganisationUri: "https://example.org"}
	resp1, err1 := ctrl1.UpdateApi(ctx1, input)
	assert.Error(t, err1)
	assert.Nil(t, resp1)

	// success pad
	orgID := "https://example.org"
	repo2 := &stubRepo{
		findOasFunc: func(ctx context.Context, url string) (*models.Api, error) {
			return &models.Api{
				Id:             "id",
				OrganisationID: &orgID,
				Organisation:   &models.Organisation{Uri: orgID, Label: "ORG"},
				Servers:        []models.Server{}, // altijd een lege slice, nooit nil
			}, nil
		},
	}
	svc2 := services.NewAPIsAPIService(repo2)
	ctrl2 := NewAPIsAPIController(svc2)

	ctx2, _ := gin.CreateTestContext(w)
	ctx2.Request = httptest.NewRequest("PUT", "/v1/apis", nil)

	input2 := &models.UpdateApiInput{OasUrl: "u", OrganisationUri: "https://example.org"}
	resp2, err2 := ctrl2.UpdateApi(ctx2, input2)
	assert.NoError(t, err2)
	assert.NotNil(t, resp2)
}

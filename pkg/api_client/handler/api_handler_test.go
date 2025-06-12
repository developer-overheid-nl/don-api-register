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
	// stub repository
	repo := &stubRepo{
		listFunc: func(ctx context.Context, page, perPage int) ([]models.Api, models.Pagination, error) {
			apis := []models.Api{{Id: "a1"}, {Id: "a2"}}
			pag := models.Pagination{CurrentPage: page, RecordsPerPage: perPage}
			return apis, pag, nil
		},
	}
	// real service and controller
	svc := services.NewAPIsAPIService(repo)
	ctrl := NewAPIsAPIController(svc)

	// setup gin context
	w := httptest.NewRecorder()
	r := gin.New()
	r.GET("/v1/apis", func(c *gin.Context) {})
	ctx, _ := gin.CreateTestContext(w)
	req := httptest.NewRequest("GET", "/v1/apis?page=3&perPage=7", nil)
	req.Host = "host"
	ctx.Request = req
	// invoke router to set FullPath
	r.HandleContext(ctx)

	// call handler
	resp, err := ctrl.ListApis(ctx, &listApisParams{Page: 3, PerPage: 7})
	assert.NoError(t, err)
	res := resp.(models.ApiListResponse)
	assert.Equal(t, "https://host/v1/apis?page=3&perPage=7", res.Links.Self.Href)
	assert.Len(t, res.Apis, 2)
}

func TestRetrieveApi_Handler(t *testing.T) {
	// success
	repo1 := &stubRepo{
		retrFunc: func(ctx context.Context, id string) (*models.Api, error) { return &models.Api{Id: id}, nil },
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
	ctx1.Params = gin.Params{{Key: "id", Value: "id1"}}

	resp1, err1 := ctrl1.RetrieveApi(ctx1, &models.RetrieveApiRequest{Id: "id1"})
	assert.NoError(t, err1)
	assert.Equal(t, "id1", resp1.Api.Id)
	assert.Len(t, resp1.LintResults, 1)

	// not found
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
	ctx2.Params = gin.Params{{Key: "id", Value: "missing"}}

	_, err2 := ctrl2.RetrieveApi(ctx2, &models.RetrieveApiRequest{Id: "missing"})
	assert.Error(t, err2)
}

func TestCreateApiFromOas_Handler(t *testing.T) {
	repo := &stubRepo{findOasFunc: func(ctx context.Context, url string) (*models.Api, error) { return &models.Api{Id: "i"}, nil }}
	svc := services.NewAPIsAPIService(repo)
	ctrl := NewAPIsAPIController(svc)

	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)
	resp, err := ctrl.CreateApiFromOas(ctx, &models.Api{OasUri: "u"})
	assert.Nil(t, resp)
	assert.Error(t, err)
}

func TestUpdateApi_Handler(t *testing.T) {
	// needs post
	repo1 := &stubRepo{findOasFunc: func(ctx context.Context, url string) (*models.Api, error) { return nil, nil }}
	svc1 := services.NewAPIsAPIService(repo1)
	ctrl1 := NewAPIsAPIController(svc1)

	w := httptest.NewRecorder()
	ctx1, _ := gin.CreateTestContext(w)
	ctx1.Request = httptest.NewRequest("PUT", "/v1/apis", nil)

	_, err1 := ctrl1.UpdateApi(ctx1, &models.OasParams{OasUrl: "u"})
	assert.Error(t, err1)

	// success
	repo2 := &stubRepo{findOasFunc: func(ctx context.Context, url string) (*models.Api, error) { return &models.Api{Id: "id"}, nil }}
	svc2 := services.NewAPIsAPIService(repo2)
	ctrl2 := NewAPIsAPIController(svc2)

	ctx2, _ := gin.CreateTestContext(w)
	ctx2.Request = httptest.NewRequest("PUT", "/v1/apis", nil)

	resp2, err2 := ctrl2.UpdateApi(ctx2, &models.OasParams{OasUrl: "u"})
	assert.NoError(t, err2)
	assert.Nil(t, resp2)
}

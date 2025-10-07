package services_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/developer-overheid-nl/don-api-register/pkg/api_client/models"
	"github.com/developer-overheid-nl/don-api-register/pkg/api_client/services"
	"github.com/stretchr/testify/assert"
	"gorm.io/gorm"
)

// stubRepo implements repositories.ApiRepository for testing
type stubRepo struct {
	findByOas  func(ctx context.Context, oasUrl string) (*models.Api, error)
	findOrg    func(ctx context.Context, uri string) (*models.Organisation, error)
	getByID    func(ctx context.Context, id string) (*models.Api, error)
	getLintRes func(ctx context.Context, apiID string) ([]models.LintResult, error)
	getApis    func(ctx context.Context, page, perPage int, organisation *string, ids *string) ([]models.Api, models.Pagination, error)
	searchApis func(ctx context.Context, query string, limit int) ([]models.Api, error)
	saveServer func(server models.Server) error
	saveApi    func(api *models.Api) error
	saveOrg    func(org *models.Organisation) error
	getOrgs    func(ctx context.Context) ([]models.Organisation, int, error)
}

func (s *stubRepo) FindByOasUrl(ctx context.Context, url string) (*models.Api, error) {
	return s.findByOas(ctx, url)
}
func (s *stubRepo) FindOrganisationByURI(ctx context.Context, uri string) (*models.Organisation, error) {
	return s.findOrg(ctx, uri)
}
func (s *stubRepo) GetApiByID(ctx context.Context, id string) (*models.Api, error) {
	return s.getByID(ctx, id)
}
func (s *stubRepo) GetLintResults(ctx context.Context, apiID string) ([]models.LintResult, error) {
	if s.getLintRes != nil {
		return s.getLintRes(ctx, apiID)
	}
	return nil, nil
}
func (s *stubRepo) GetApis(ctx context.Context, page, perPage int, organisation *string, ids *string) ([]models.Api, models.Pagination, error) {
	return s.getApis(ctx, page, perPage, organisation, ids)
}
func (s *stubRepo) SearchApis(ctx context.Context, query string, limit int) ([]models.Api, error) {
	if s.searchApis != nil {
		return s.searchApis(ctx, query, limit)
	}
	return []models.Api{}, nil
}

// unused methods
func (s *stubRepo) SaveServer(server models.Server) error               { return s.saveServer(server) }
func (s *stubRepo) Save(api *models.Api) error                          { return s.saveApi(api) }
func (s *stubRepo) UpdateApi(ctx context.Context, api models.Api) error { return nil }
func (s *stubRepo) SaveOrganisatie(org *models.Organisation) error {
	if s.saveOrg != nil {
		return s.saveOrg(org)
	}
	return nil
}
func (s *stubRepo) AllApis(ctx context.Context) ([]models.Api, error)                   { return nil, nil }
func (s *stubRepo) SaveLintResult(ctx context.Context, result *models.LintResult) error { return nil }
func (s *stubRepo) GetOrganisations(ctx context.Context) ([]models.Organisation, int, error) {
	return s.getOrgs(ctx)
}
func (s *stubRepo) SaveArtifact(ctx context.Context, art *models.ApiArtifact) error { return nil }
func (s *stubRepo) GetArtifact(ctx context.Context, apiID, kind string) (*models.ApiArtifact, error) {
	return nil, nil
}

func TestUpdateOasUri_NotFound(t *testing.T) {
	repo := &stubRepo{
		getByID: func(ctx context.Context, id string) (*models.Api, error) {
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
		getApis: func(ctx context.Context, page, perPage int, organisation *string, ids *string) ([]models.Api, models.Pagination, error) {
			return apis, pagination, nil
		},
	}
	service := services.NewAPIsAPIService(repo)
	baseURL := "/v1/apis"
	p := &models.ListApisParams{Page: 1, PerPage: 2, BaseURL: baseURL}
	res, _, err := service.ListApis(context.Background(), p)
	assert.NoError(t, err)
	assert.Len(t, res, 2)
	assert.Equal(t, 2, pagination.TotalRecords)
}

func TestListApis_UsesApisFilter(t *testing.T) {
	repo := &stubRepo{
		getApis: func(ctx context.Context, page, perPage int, organisation *string, ids *string) ([]models.Api, models.Pagination, error) {
			if ids == nil {
				t.Fatal("expected ids filter to be passed")
			}
			if want := "a1,a2"; *ids != want {
				t.Fatalf("expected ids %q, got %q", want, *ids)
			}
			return []models.Api{}, models.Pagination{}, nil
		},
	}
	service := services.NewAPIsAPIService(repo)
	raw := "  a1,a2  "
	params := &models.ListApisParams{Page: 1, PerPage: 10, Ids: &raw}
	_, _, err := service.ListApis(context.Background(), params)
	assert.NoError(t, err)
}

func TestSearchApis_TrimsQueryAndAppliesDefaultLimit(t *testing.T) {
	called := false
	repo := &stubRepo{
		searchApis: func(ctx context.Context, query string, limit int) ([]models.Api, error) {
			called = true
			assert.Equal(t, "digid", query)
			assert.Equal(t, models.DefaultSearchLimit, limit)
			return []models.Api{{
				Id:     "api-1",
				OasUri: "https://example.com/openapi.json",
				Title:  "DigiD API",
				Organisation: &models.Organisation{
					Uri:   "https://org.test",
					Label: "Org",
				},
			}}, nil
		},
	}
	service := services.NewAPIsAPIService(repo)
	results, err := service.SearchApis(context.Background(), "  digid  ", 0)
	assert.NoError(t, err)
	assert.True(t, called)
	assert.Len(t, results, 1)
	assert.Equal(t, "api-1", results[0].Id)
}

func TestSearchApis_EmptyQueryReturnsNoResults(t *testing.T) {
	repo := &stubRepo{
		searchApis: func(ctx context.Context, query string, limit int) ([]models.Api, error) {
			t.Fatalf("expected repository not to be called, got query=%q", query)
			return nil, nil
		},
	}
	service := services.NewAPIsAPIService(repo)
	results, err := service.SearchApis(context.Background(), "   ", 5)
	assert.NoError(t, err)
	assert.Len(t, results, 0)
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
		findOrg: func(ctx context.Context, uri string) (*models.Organisation, error) {
			return &models.Organisation{Uri: uri, Label: "Org"}, nil
		},
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

func TestListOrganisations_Service(t *testing.T) {
	repo := &stubRepo{
		getOrgs: func(ctx context.Context) ([]models.Organisation, int, error) {
			orgs := []models.Organisation{
				{Uri: "https://example.org/a", Label: "A"},
				{Uri: "https://example.org/b", Label: "B"},
			}
			return orgs, len(orgs), nil
		},
	}

	service := services.NewAPIsAPIService(repo)
	orgs, _, err := service.ListOrganisations(context.Background())

	assert.NoError(t, err)
	assert.Len(t, orgs, 2)
	assert.Equal(t, "A", orgs[0].Label)
}

func TestCreateOrganisation_Service(t *testing.T) {
	var saved models.Organisation
	repo := &stubRepo{
		saveOrg: func(org *models.Organisation) error { saved = *org; return nil },
	}
	service := services.NewAPIsAPIService(repo)
	org := &models.Organisation{Uri: "https://example.org", Label: "Org"}
	res, err := service.CreateOrganisation(context.Background(), org)
	assert.NoError(t, err)
	assert.Equal(t, "Org", res.Label)
	assert.Equal(t, saved.Uri, res.Uri)
}

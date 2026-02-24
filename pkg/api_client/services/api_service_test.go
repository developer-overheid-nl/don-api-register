package services_test

import (
	"context"
	"net/http"
	"testing"

	httpclient "github.com/developer-overheid-nl/don-api-register/pkg/api_client/helpers/httpclient"
	openapihelper "github.com/developer-overheid-nl/don-api-register/pkg/api_client/helpers/openapi"
	toolslint "github.com/developer-overheid-nl/don-api-register/pkg/api_client/helpers/tools"
	"github.com/developer-overheid-nl/don-api-register/pkg/api_client/models"
	"github.com/developer-overheid-nl/don-api-register/pkg/api_client/services"
	"github.com/developer-overheid-nl/don-api-register/pkg/api_client/testutil"
	"github.com/stretchr/testify/assert"
	"gorm.io/gorm"
)

// stubRepo implements repositories.ApiRepository for testing
type stubRepo struct {
	findByOas    func(ctx context.Context, oasUrl string) (*models.Api, error)
	findOrg      func(ctx context.Context, uri string) (*models.Organisation, error)
	getByID      func(ctx context.Context, id string) (*models.Api, error)
	getLintRes   func(ctx context.Context, apiID string) ([]models.LintResult, error)
	getApis      func(ctx context.Context, page, perPage int, organisation *string, ids *string) ([]models.Api, models.Pagination, error)
	searchApis   func(ctx context.Context, page, perPage int, organisation *string, query string) ([]models.Api, models.Pagination, error)
	saveServer   func(server models.Server) error
	saveApi      func(api *models.Api) error
	saveOrg      func(org *models.Organisation) error
	getOrgs      func(ctx context.Context) ([]models.Organisation, int, error)
	allApis      func(ctx context.Context) ([]models.Api, error)
	updateApi    func(ctx context.Context, api models.Api) error
	delArtifacts func(ctx context.Context, apiID, kind string, keep []string) error
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
func (s *stubRepo) SearchApis(ctx context.Context, page, perPage int, organisation *string, query string) ([]models.Api, models.Pagination, error) {
	if s.searchApis != nil {
		return s.searchApis(ctx, page, perPage, organisation, query)
	}
	return []models.Api{}, models.Pagination{}, nil
}

// unused methods
func (s *stubRepo) SaveServer(server models.Server) error { return s.saveServer(server) }
func (s *stubRepo) Save(api *models.Api) error            { return s.saveApi(api) }
func (s *stubRepo) UpdateApi(ctx context.Context, api models.Api) error {
	if s.updateApi != nil {
		return s.updateApi(ctx, api)
	}
	return nil
}
func (s *stubRepo) SaveOrganisatie(org *models.Organisation) error {
	if s.saveOrg != nil {
		return s.saveOrg(org)
	}
	return nil
}
func (s *stubRepo) AllApis(ctx context.Context) ([]models.Api, error) {
	if s.allApis != nil {
		return s.allApis(ctx)
	}
	return nil, nil
}
func (s *stubRepo) SaveLintResult(ctx context.Context, result *models.LintResult) error { return nil }
func (s *stubRepo) GetOrganisations(ctx context.Context) ([]models.Organisation, int, error) {
	return s.getOrgs(ctx)
}
func (s *stubRepo) SaveArtifact(ctx context.Context, art *models.ApiArtifact) error { return nil }
func (s *stubRepo) HasArtifactOfKind(ctx context.Context, apiID, kind string) (bool, error) {
	return false, nil
}
func (s *stubRepo) GetOasArtifact(ctx context.Context, apiID, version, format string) (*models.ApiArtifact, error) {
	return nil, nil
}
func (s *stubRepo) GetArtifact(ctx context.Context, apiID, kind string) (*models.ApiArtifact, error) {
	return nil, nil
}
func (s *stubRepo) DeleteArtifactsByKind(ctx context.Context, apiID, kind string, keep []string) error {
	if s.delArtifacts != nil {
		return s.delArtifacts(ctx, apiID, kind, keep)
	}
	return nil
}

func TestGetOasDocument_InvalidVersion(t *testing.T) {
	repo := &stubRepo{}
	service := services.NewAPIsAPIService(repo)
	art, err := service.GetOasDocument(context.Background(), "api-1", "3.2", "json")
	assert.Nil(t, art)
	assert.Error(t, err)
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

func TestCreateApiFromOas_UsesSpecContactOverBody(t *testing.T) {
	spec := `{
  "openapi": "3.0.0",
  "info": {
    "title": "Contact Spec",
    "version": "1.1.0",
    "contact": {
      "name": "Spec Contact",
      "email": "spec@example.com",
      "url": "https://spec.example.com"
    }
  },
  "paths": {
    "/ping": {
      "get": {
        "responses": {
          "200": {
            "description": "pong"
          }
        }
      }
    }
  }
}`

	srv := testutil.NewTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(spec))
	}))

	orgURI := "https://identifier.overheid.nl/tooi/id/0000"

	var updated models.Api
	repo := &stubRepo{
		findByOas: func(ctx context.Context, url string) (*models.Api, error) { return nil, nil },
		findOrg: func(ctx context.Context, uri string) (*models.Organisation, error) {
			return &models.Organisation{Uri: orgURI, Label: "Org Label"}, nil
		},
		saveServer: func(server models.Server) error { return nil },
		saveApi: func(api *models.Api) error {
			return nil
		},
		updateApi: func(ctx context.Context, api models.Api) error {
			updated = api
			return nil
		},
	}

	service := services.NewAPIsAPIService(repo)
	input := models.ApiPost{
		OasUrl:          srv.URL,
		OrganisationUri: orgURI,
		Contact: models.Contact{
			Name:  "Body Contact",
			Email: "body@example.com",
			URL:   "https://body.example.com",
		},
	}

	summary, err := service.CreateApiFromOas(input)
	assert.NoError(t, err)
	assert.NotNil(t, summary)

	assert.Equal(t, "Spec Contact", summary.Contact.Name)
	assert.Equal(t, "spec@example.com", summary.Contact.Email)
	assert.Equal(t, "https://spec.example.com", summary.Contact.URL)

	assert.Equal(t, "Spec Contact", updated.ContactName)
	assert.Equal(t, "spec@example.com", updated.ContactEmail)
	assert.Equal(t, "https://spec.example.com", updated.ContactUrl)
}

func TestUpdateOasUri_PersistsUpdatedFields(t *testing.T) {
	spec := `{
  "openapi": "3.0.0",
  "info": {
    "title": "Nieuwe API",
    "version": "2.0.0",
    "description": "Nieuwe beschrijving",
    "contact": {
      "name": "Nieuw Contact",
      "email": "nieuw@example.com",
      "url": "https://nieuw.example.com"
    },
    "x-sunset": "2026-01-01",
    "x-deprecated": "2025-01-01"
  },
  "externalDocs": {
    "url": "https://docs.nieuw.example.com"
  },
  "servers": [
    {
      "url": "https://api.nieuw.example.com",
      "description": "Production"
    }
  ],
  "security": [
    {
      "ApiKeyAuth": []
    }
  ],
  "components": {
    "securitySchemes": {
      "ApiKeyAuth": {
        "type": "apiKey",
        "name": "X-API-Key",
        "in": "header"
      }
    }
  },
  "paths": {
    "/ping": {
      "get": {
        "responses": {
          "200": {
            "description": "pong"
          }
        }
      }
    }
  }
}`

	srv := testutil.NewTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(spec))
	}))

	res, err := openapihelper.FetchParseValidateAndHash(context.Background(), toolslint.OASInput{OasUrl: srv.URL}, openapihelper.FetchOpts{})
	assert.NoError(t, err)

	orgURI := "https://example.org/org"
	existing := &models.Api{
		Id:           "api-123",
		OasUri:       "https://old.example.com/openapi.json",
		OasHash:      res.Hash,
		Title:        "Oude titel",
		Description:  "Oude beschrijving",
		Version:      "1.0.0",
		Sunset:       "2024-01-01",
		Deprecated:   "2023-01-01",
		DocsUrl:      "https://docs.oud.example.com",
		ContactName:  "Oud Contact",
		ContactEmail: "oud@example.com",
		ContactUrl:   "https://oud.example.com",
		Auth:         "http",
		Organisation: &models.Organisation{Uri: orgURI, Label: "Org Label"},
		AdrScore:     nil,
	}
	existing.OrganisationID = &orgURI

	var saved models.Api
	var updateCalled bool
	repo := &stubRepo{
		getByID: func(ctx context.Context, id string) (*models.Api, error) {
			assert.Equal(t, "api-123", id)
			return existing, nil
		},
		updateApi: func(ctx context.Context, api models.Api) error {
			updateCalled = true
			saved = api
			return nil
		},
	}

	service := services.NewAPIsAPIService(repo)
	input := &models.UpdateApiInput{
		Id:              "api-123",
		OasUrl:          srv.URL,
		OrganisationUri: orgURI,
		Contact: models.Contact{
			Name:  "Fallback Naam",
			Email: "fallback@example.com",
			URL:   "https://fallback.example.com",
		},
	}

	summary, err := service.UpdateOasUri(context.Background(), input)
	assert.NoError(t, err)
	assert.NotNil(t, summary)
	assert.True(t, updateCalled)

	assert.Equal(t, srv.URL, saved.OasUri)
	assert.Equal(t, "Nieuwe API", saved.Title)
	assert.Equal(t, "Nieuwe beschrijving", saved.Description)
	assert.Equal(t, "2.0.0", saved.Version)
	assert.Equal(t, "2026-01-01", saved.Sunset)
	assert.Equal(t, "2025-01-01", saved.Deprecated)
	assert.Equal(t, "https://docs.nieuw.example.com", saved.DocsUrl)
	assert.Equal(t, "Nieuw Contact", saved.ContactName)
	assert.Equal(t, "nieuw@example.com", saved.ContactEmail)
	assert.Equal(t, "https://nieuw.example.com", saved.ContactUrl)
	assert.Equal(t, res.Hash, saved.OasHash)
	assert.Equal(t, "api_key", saved.Auth)
	if assert.NotNil(t, saved.OrganisationID) {
		assert.Equal(t, orgURI, *saved.OrganisationID)
	}
	if assert.NotNil(t, saved.Organisation) {
		assert.Equal(t, orgURI, saved.Organisation.Uri)
		assert.Equal(t, "Org Label", saved.Organisation.Label)
	}
	assert.Len(t, saved.Servers, 1)
	assert.Equal(t, srv.URL, summary.OasUrl)
	assert.Equal(t, "Nieuwe API", summary.Title)
	assert.Equal(t, "2.0.0", summary.Lifecycle.Version)
}

func TestRefreshChangedApis_UpdatesWhenHashDiffers(t *testing.T) {
	spec := `{
  "openapi": "3.0.0",
  "info": {
    "title": "Dagelijkse refresh",
    "version": "1.0.0",
    "contact": {
      "name": "Spec Contact",
      "email": "spec@example.com",
      "url": "https://spec.example.com"
    }
  },
  "paths": {
    "/ping": {
      "get": {
        "responses": {
          "200": {
            "description": "pong"
          }
        }
      }
    }
  }
}`

	srv := testutil.NewTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(spec))
	}))

	orgURI := "https://org.example.com"
	var updated models.Api
	repo := &stubRepo{
		allApis: func(ctx context.Context) ([]models.Api, error) {
			return []models.Api{
				{
					Id:      "api-refresh",
					OasUri:  srv.URL,
					OasHash: "outdated",
				},
			}, nil
		},
		getByID: func(ctx context.Context, id string) (*models.Api, error) {
			if id != "api-refresh" {
				return nil, gorm.ErrRecordNotFound
			}
			return &models.Api{
				Id:           "api-refresh",
				OasUri:       srv.URL,
				OasHash:      "outdated",
				Organisation: &models.Organisation{Uri: orgURI, Label: "Org"},
				OrganisationID: func() *string {
					return &orgURI
				}(),
			}, nil
		},
		updateApi: func(ctx context.Context, api models.Api) error {
			updated = api
			return nil
		},
	}

	service := services.NewAPIsAPIService(repo)
	count, err := service.RefreshChangedApis(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, 1, count)
	assert.Equal(t, srv.URL, updated.OasUri)
	assert.NotEmpty(t, updated.OasHash)
	assert.Equal(t, "Dagelijkse refresh", updated.Title)
}

func TestRefreshChangedApis_SkipsWhenHashUnchanged(t *testing.T) {
	spec := `{
  "openapi": "3.0.0",
  "info": { "title": "Ongewijzigd", "version": "1" },
  "paths": { "/ping": { "get": { "responses": { "200": { "description": "ok" } } } } }
}`

	srv := testutil.NewTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(spec))
	}))

	res, err := openapihelper.FetchParseValidateAndHash(context.Background(), toolslint.OASInput{OasUrl: srv.URL}, openapihelper.FetchOpts{})
	assert.NoError(t, err)

	repo := &stubRepo{
		allApis: func(ctx context.Context) ([]models.Api, error) {
			return []models.Api{
				{Id: "api-static", OasUri: srv.URL, OasHash: res.Hash},
			}, nil
		},
		getByID: func(ctx context.Context, id string) (*models.Api, error) {
			t.Fatalf("GetApiByID zou niet aangeroepen moeten worden, maar is aangeroepen met %s", id)
			return nil, nil
		},
	}

	service := services.NewAPIsAPIService(repo)
	count, err := service.RefreshChangedApis(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, 0, count)
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
		searchApis: func(ctx context.Context, page, perPage int, organisation *string, query string) ([]models.Api, models.Pagination, error) {
			called = true
			assert.Equal(t, 1, page)
			assert.Equal(t, 0, perPage)
			assert.Equal(t, "digid", query)
			return []models.Api{{
					Id:     "api-1",
					OasUri: "https://example.com/openapi.json",
					Title:  "DigiD API",
					Organisation: &models.Organisation{
						Uri:   "https://org.test",
						Label: "Org",
					},
				}}, models.Pagination{
					CurrentPage:    page,
					RecordsPerPage: perPage,
					TotalPages:     2,
					TotalRecords:   12,
				}, nil
		},
	}
	service := services.NewAPIsAPIService(repo)
	params := &models.ListApisSearchParams{Query: "  digid  ", Page: 1}
	results, pagination, err := service.SearchApis(context.Background(), params)
	assert.NoError(t, err)
	assert.True(t, called)
	assert.Equal(t, 12, pagination.TotalRecords)
	assert.Equal(t, 2, pagination.TotalPages)
	assert.Len(t, results, 1)
	assert.Equal(t, "api-1", results[0].Id)
}

func TestSearchApis_EmptyQueryReturnsNoResults(t *testing.T) {
	repo := &stubRepo{}
	service := services.NewAPIsAPIService(repo)
	results, pagination, err := service.SearchApis(context.Background(), &models.ListApisSearchParams{Query: "   ", Page: 2, PerPage: 5})
	assert.NoError(t, err)
	assert.Len(t, results, 0)
	assert.Equal(t, 0, pagination.TotalRecords)
	assert.Equal(t, 0, pagination.TotalPages)
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
  "paths": {
    "/ping": {
      "get": {
        "responses": {
          "200": {
            "description": "pong"
          }
        }
      }
    }
  }
}`
	server := testutil.NewTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, err := w.Write([]byte(spec))
		if err != nil {
			return
		}
	}))

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

func TestPublishAllApisToTypesense_Disabled(t *testing.T) {
	t.Setenv("ENABLE_TYPESENSE", "false")
	repo := &stubRepo{
		allApis: func(ctx context.Context) ([]models.Api, error) {
			t.Fatalf("AllApis should not be called when Typesense is disabled")
			return nil, nil
		},
	}
	service := services.NewAPIsAPIService(repo)
	err := service.PublishAllApisToTypesense(context.Background())
	assert.NoError(t, err)
}

func TestPublishAllApisToTypesense_SendsDocuments(t *testing.T) {
	var calls int
	server := testutil.NewTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(http.StatusCreated)
	}))

	t.Setenv("TYPESENSE_ENDPOINT", server.URL)
	t.Setenv("TYPESENSE_API_KEY", "secret")
	t.Setenv("TYPESENSE_COLLECTION", "apis")
	t.Setenv("ENABLE_TYPESENSE", "true")

	prevClient := httpclient.HTTPClient
	httpclient.HTTPClient = server.Client()
	t.Cleanup(func() { httpclient.HTTPClient = prevClient })

	repo := &stubRepo{
		allApis: func(ctx context.Context) ([]models.Api, error) {
			return []models.Api{{Id: "api-1"}, {Id: "api-2"}}, nil
		},
	}
	service := services.NewAPIsAPIService(repo)
	err := service.PublishAllApisToTypesense(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, 2, calls)
}

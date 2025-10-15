package api_client_test

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync"
	"testing"

	api_client "github.com/developer-overheid-nl/don-api-register/pkg/api_client"
	"github.com/developer-overheid-nl/don-api-register/pkg/api_client/handler"
	problem "github.com/developer-overheid-nl/don-api-register/pkg/api_client/helpers/problem"
	"github.com/developer-overheid-nl/don-api-register/pkg/api_client/models"
	"github.com/developer-overheid-nl/don-api-register/pkg/api_client/repositories"
	"github.com/developer-overheid-nl/don-api-register/pkg/api_client/services"
	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"github.com/loopfz/gadgeto/tonic"
	"github.com/stretchr/testify/assert"
)

var errorHookOnce sync.Once

func setupErrorHook() {
	errorHookOnce.Do(func() {
		tonic.SetErrorHook(func(c *gin.Context, err error) (int, interface{}) {
			var be tonic.BindError
			if errors.As(err, &be) || isValidationErr(err) {
				invalids := invalidParamsFromBinding(err, models.UpdateApiInput{})
				apiErr := problem.NewBadRequest("body", "Invalid input voor update", invalids...)
				c.Header("Content-Type", "application/problem+json")
				return apiErr.Status, apiErr
			}

			if apiErr, ok := err.(problem.APIError); ok {
				c.Header("Content-Type", "application/problem+json")
				return apiErr.Status, apiErr
			}

			internal := problem.NewInternalServerError(err.Error())
			c.Header("Content-Type", "application/problem+json")
			return internal.Status, internal
		})
	})
}

func invalidParamsFromBinding(err error, sample any) []problem.InvalidParam {
	var verrs validator.ValidationErrors
	if !errors.As(err, &verrs) {
		return []problem.InvalidParam{{Name: "body", Reason: err.Error()}}
	}

	t := reflect.TypeOf(sample)
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}

	out := make([]problem.InvalidParam, 0, len(verrs))
	for _, fe := range verrs {
		name := fe.Field()
		if f, ok := t.FieldByName(fe.StructField()); ok {
			if tag := f.Tag.Get("json"); tag != "" && tag != "-" {
				name = strings.Split(tag, ",")[0]
			}
		}
		out = append(out, problem.InvalidParam{
			Name:   name,
			Reason: humanReason(fe),
		})
	}
	return out
}

func humanReason(fe validator.FieldError) string {
	switch fe.Tag() {
	case "required":
		return "is verplicht"
	case "url":
		return "Moet een geldige URL zijn (bijv. https://…)"
	default:
		return fe.Error()
	}
}

func isValidationErr(err error) bool {
	var verrs validator.ValidationErrors
	return errors.As(err, &verrs)
}

// stubRepo implements repositories.ApiRepository for integration tests
// Each function can be optional; unused functions return zero values.
type stubRepo struct {
	getApis   func(ctx context.Context, page, perPage int, organisation *string, ids *string) ([]models.Api, models.Pagination, error)
	search    func(ctx context.Context, query string, limit int) ([]models.Api, error)
	getByID   func(ctx context.Context, id string) (*models.Api, error)
	findByOas func(ctx context.Context, url string) (*models.Api, error)
	findOrg   func(ctx context.Context, uri string) (*models.Organisation, error)
	saveApi   func(api *models.Api) error
	saveSrv   func(server models.Server) error
	saveOrg   func(org *models.Organisation) error
	update    func(ctx context.Context, api models.Api) error
	getLint   func(ctx context.Context, apiID string) ([]models.LintResult, error)
	getOrgs   func(ctx context.Context) ([]models.Organisation, int, error)
	saveLint  func(ctx context.Context, result *models.LintResult) error
	saveArt   func(ctx context.Context, art *models.ApiArtifact) error
	getArt    func(ctx context.Context, apiID, kind string) (*models.ApiArtifact, error)
}

func (s *stubRepo) GetApis(ctx context.Context, page, perPage int, organisation *string, ids *string) ([]models.Api, models.Pagination, error) {
	if s.getApis != nil {
		return s.getApis(ctx, page, perPage, organisation, ids)
	}
	return nil, models.Pagination{}, nil
}
func (s *stubRepo) SearchApis(ctx context.Context, query string, limit int) ([]models.Api, error) {
	if s.search != nil {
		return s.search(ctx, query, limit)
	}
	return []models.Api{}, nil
}
func (s *stubRepo) GetApiByID(ctx context.Context, id string) (*models.Api, error) {
	if s.getByID != nil {
		return s.getByID(ctx, id)
	}
	return nil, nil
}
func (s *stubRepo) FindByOasUrl(ctx context.Context, url string) (*models.Api, error) {
	if s.findByOas != nil {
		return s.findByOas(ctx, url)
	}
	return nil, nil
}
func (s *stubRepo) FindOrganisationByURI(ctx context.Context, uri string) (*models.Organisation, error) {
	if s.findOrg != nil {
		return s.findOrg(ctx, uri)
	}
	return nil, nil
}
func (s *stubRepo) Save(api *models.Api) error {
	if s.saveApi != nil {
		return s.saveApi(api)
	}
	return nil
}
func (s *stubRepo) SaveServer(server models.Server) error {
	if s.saveSrv != nil {
		return s.saveSrv(server)
	}
	return nil
}
func (s *stubRepo) SaveOrganisatie(org *models.Organisation) error {
	if s.saveOrg != nil {
		return s.saveOrg(org)
	}
	return nil
}
func (s *stubRepo) AllApis(ctx context.Context) ([]models.Api, error) { return nil, nil }
func (s *stubRepo) UpdateApi(ctx context.Context, api models.Api) error {
	if s.update != nil {
		return s.update(ctx, api)
	}
	return nil
}
func (s *stubRepo) SaveLintResult(ctx context.Context, result *models.LintResult) error {
	if s.saveLint != nil {
		return s.saveLint(ctx, result)
	}
	return nil
}
func (s *stubRepo) GetLintResults(ctx context.Context, apiID string) ([]models.LintResult, error) {
	if s.getLint != nil {
		return s.getLint(ctx, apiID)
	}
	return nil, nil
}
func (s *stubRepo) GetOrganisations(ctx context.Context) ([]models.Organisation, int, error) {
	if s.getOrgs != nil {
		return s.getOrgs(ctx)
	}
	return nil, 0, nil
}
func (s *stubRepo) SaveArtifact(ctx context.Context, art *models.ApiArtifact) error {
	if s.saveArt != nil {
		return s.saveArt(ctx, art)
	}
	return nil
}
func (s *stubRepo) GetArtifact(ctx context.Context, apiID, kind string) (*models.ApiArtifact, error) {
	if s.getArt != nil {
		return s.getArt(ctx, apiID, kind)
	}
	return nil, nil
}

func newServer(repo repositories.ApiRepository) *httptest.Server {
	setupErrorHook()
	svc := services.NewAPIsAPIService(repo)
	ctrl := handler.NewAPIsAPIController(svc)
	router := api_client.NewRouter("test", ctrl)
	return httptest.NewServer(router)
}

func tokenWithScope(scope string) string {
	hdr := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"HS256"}`))
	payload := base64.RawURLEncoding.EncodeToString([]byte(fmt.Sprintf(`{"scope":"%s"}`, scope)))
	return hdr + "." + payload + ".sig"
}

func readAndLogBody(t *testing.T, resp *http.Response) []byte {
	t.Helper()
	data, err := io.ReadAll(resp.Body)
	assert.NoError(t, err)
	resp.Body.Close()
	method, path := "", ""
	if resp.Request != nil {
		method = resp.Request.Method
		if resp.Request.URL != nil {
			path = resp.Request.URL.RequestURI()
		}
	}
	t.Logf("response %s %s -> %d: %s", method, path, resp.StatusCode, string(data))
	return data
}

func decodeJSONBody[T any](t *testing.T, resp *http.Response, out *T) {
	t.Helper()
	data := readAndLogBody(t, resp)
	assert.NoError(t, json.Unmarshal(data, out))
}

func TestIntegration_ListApis(t *testing.T) {
	repo := &stubRepo{getApis: func(ctx context.Context, page, perPage int, organisation *string, ids *string) ([]models.Api, models.Pagination, error) {
		apis := []models.Api{
			{Id: "a1", Organisation: &models.Organisation{Uri: "org", Label: "l"}},
			{Id: "a2", Organisation: &models.Organisation{Uri: "org", Label: "l"}},
		}
		return apis, models.Pagination{}, nil
	}}
	srv := newServer(repo)
	defer srv.Close()

	req, _ := http.NewRequest(http.MethodGet, srv.URL+"/v1/apis?page=1&perPage=2", nil)
	req.Header.Set("x-api-key", "test")
	resp, err := http.DefaultClient.Do(req)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	var body []models.ApiSummary
	decodeJSONBody(t, resp, &body)
	assert.Len(t, body, 2)
}

func TestIntegration_ListApis_Error(t *testing.T) {
	repoErr := errors.New("db kapot")
	repo := &stubRepo{getApis: func(ctx context.Context, page, perPage int, organisation *string, ids *string) ([]models.Api, models.Pagination, error) {
		return nil, models.Pagination{}, repoErr
	}}
	srv := newServer(repo)
	defer srv.Close()

	req, _ := http.NewRequest(http.MethodGet, srv.URL+"/v1/apis?page=1", nil)
	req.Header.Set("x-api-key", "test")
	resp, err := http.DefaultClient.Do(req)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
	_ = readAndLogBody(t, resp)
}

func TestIntegration_SearchApis(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		repo := &stubRepo{search: func(ctx context.Context, query string, limit int) ([]models.Api, error) {
			return []models.Api{{Id: "a1", Title: "API", Organisation: &models.Organisation{Uri: "https://org", Label: "Org"}}}, nil
		}}
		srv := newServer(repo)
		defer srv.Close()

		req, _ := http.NewRequest(http.MethodGet, srv.URL+"/v1/apis/_search?q=test&limit=5", nil)
		req.Header.Set("x-api-key", "test")
		resp, err := http.DefaultClient.Do(req)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		var body []models.ApiSummary
		decodeJSONBody(t, resp, &body)
		assert.Len(t, body, 1)
	})

	t.Run("backend error", func(t *testing.T) {
		repo := &stubRepo{search: func(ctx context.Context, query string, limit int) ([]models.Api, error) {
			return nil, errors.New("zoekfout")
		}}
		srv := newServer(repo)
		defer srv.Close()
		req, _ := http.NewRequest(http.MethodGet, srv.URL+"/v1/apis/_search?q=test", nil)
		req.Header.Set("x-api-key", "test")
		resp, err := http.DefaultClient.Do(req)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
		data := readAndLogBody(t, resp)
		assert.Contains(t, string(data), "zoekfout")
	})
}

func TestIntegration_RetrieveApi(t *testing.T) {
	repo := &stubRepo{getByID: func(ctx context.Context, id string) (*models.Api, error) {
		return &models.Api{Id: id, Organisation: &models.Organisation{Uri: "o", Label: "l"}}, nil
	}}
	srv := newServer(repo)
	defer srv.Close()

	req, _ := http.NewRequest(http.MethodGet, srv.URL+"/v1/apis/id1", nil)
	req.Header.Set("x-api-key", "test")
	resp, err := http.DefaultClient.Do(req)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	var body models.ApiDetail
	decodeJSONBody(t, resp, &body)
	assert.Equal(t, "id1", body.Id)
}

func TestIntegration_RetrieveApi_NotFound(t *testing.T) {
	repo := &stubRepo{getByID: func(ctx context.Context, id string) (*models.Api, error) {
		return nil, nil
	}}
	srv := newServer(repo)
	defer srv.Close()
	req, _ := http.NewRequest(http.MethodGet, srv.URL+"/v1/apis/missing", nil)
	resp, err := http.DefaultClient.Do(req)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	data := readAndLogBody(t, resp)
	assert.Contains(t, string(data), "Api not found")
}

func TestIntegration_CreateApiFromOas(t *testing.T) {
	spec := `{
  "openapi": "3.0.0",
  "info": {
    "title": "T",
    "version": "1.0.0",
    "contact": {
      "name": "n",
      "email": "test@example.org",
      "url": "https://example.org"
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
	oasSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(spec))
	}))
	defer oasSrv.Close()

	var saved models.Api
	repo := &stubRepo{
		findByOas: func(ctx context.Context, url string) (*models.Api, error) { return nil, nil },
		saveApi:   func(api *models.Api) error { saved = *api; return nil },
		saveSrv:   func(server models.Server) error { return nil },
		saveOrg:   func(org *models.Organisation) error { return nil },
		findOrg: func(ctx context.Context, uri string) (*models.Organisation, error) {
			return &models.Organisation{Uri: uri, Label: "Org"}, nil
		},
	}
	srv := newServer(repo)
	defer srv.Close()

	body := fmt.Sprintf(`{"oasUrl":"%s","organisationUri":"%s"}`, oasSrv.URL, oasSrv.URL)
	req, _ := http.NewRequest(http.MethodPost, srv.URL+"/v1/apis", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+tokenWithScope("apis:write"))
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusCreated, resp.StatusCode)
	data := readAndLogBody(t, resp)
	assert.NotZero(t, saved.Id)
	assert.Contains(t, string(data), "\"id\"")
}

func TestIntegration_CreateApiFromOas_RetryWithoutOrigin(t *testing.T) {
	spec := `{
  "openapi": "3.0.0",
  "info": {
    "title": "Retry",
    "version": "1.0.0",
    "contact": {
      "name": "n",
      "email": "test@example.org",
      "url": "https://example.org"
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

	var origins []string
	oasSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origins = append(origins, r.Header.Get("Origin"))
		if r.Header.Get("Origin") != "" {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(spec))
	}))
	t.Cleanup(oasSrv.Close)

	var saved models.Api
	repo := &stubRepo{
		findByOas: func(ctx context.Context, url string) (*models.Api, error) { return nil, nil },
		saveApi:   func(api *models.Api) error { saved = *api; return nil },
		saveSrv:   func(server models.Server) error { return nil },
		saveOrg:   func(org *models.Organisation) error { return nil },
		findOrg: func(ctx context.Context, uri string) (*models.Organisation, error) {
			return &models.Organisation{Uri: uri, Label: "Org"}, nil
		},
	}
	srv := newServer(repo)
	t.Cleanup(srv.Close)

	body := fmt.Sprintf(`{"oasUrl":"%s","organisationUri":"%s"}`, oasSrv.URL, oasSrv.URL)
	req, _ := http.NewRequest(http.MethodPost, srv.URL+"/v1/apis", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+tokenWithScope("apis:write"))
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusCreated, resp.StatusCode)
	_ = readAndLogBody(t, resp)

	assert.NotZero(t, saved.Id)
	if assert.GreaterOrEqual(t, len(origins), 2) {
		assert.NotEmpty(t, origins[0])
		assert.Empty(t, origins[1])
	}
}

func TestIntegration_GetBruno(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		var requestedKind string
		repo := &stubRepo{getArt: func(ctx context.Context, apiID, kind string) (*models.ApiArtifact, error) {
			requestedKind = kind
			return &models.ApiArtifact{ApiID: apiID, Kind: kind, ContentType: "application/zip", Filename: "proj.zip", Data: []byte("ZIP")}, nil
		}}
		srv := newServer(repo)
		defer srv.Close()
		req, _ := http.NewRequest(http.MethodGet, srv.URL+"/v1/apis/a1/bruno", nil)
		req.Header.Set("x-api-key", "test")
		resp, err := http.DefaultClient.Do(req)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		body := readAndLogBody(t, resp)
		assert.Equal(t, "bruno", requestedKind)
		assert.Equal(t, []byte("ZIP"), body)
		assert.Equal(t, "application/zip", resp.Header.Get("Content-Type"))
		assert.Contains(t, resp.Header.Get("Content-Disposition"), "proj.zip")
	})

	t.Run("not found", func(t *testing.T) {
		srv := newServer(&stubRepo{})
		defer srv.Close()
		req, _ := http.NewRequest(http.MethodGet, srv.URL+"/v1/apis/a1/bruno", nil)
		resp, err := http.DefaultClient.Do(req)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusNotFound, resp.StatusCode)
		data := readAndLogBody(t, resp)
		assert.Contains(t, string(data), "Bruno artifact not found")
	})
}

func TestIntegration_GetPostman(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		var requestedKind string
		repo := &stubRepo{getArt: func(ctx context.Context, apiID, kind string) (*models.ApiArtifact, error) {
			requestedKind = kind
			return &models.ApiArtifact{ApiID: apiID, Kind: kind, ContentType: "application/json", Filename: "postman.json", Data: []byte("{}")}, nil
		}}
		srv := newServer(repo)
		defer srv.Close()
		req, _ := http.NewRequest(http.MethodGet, srv.URL+"/v1/apis/a1/postman", nil)
		req.Header.Set("x-api-key", "test")
		resp, err := http.DefaultClient.Do(req)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		body := readAndLogBody(t, resp)
		assert.Equal(t, "postman", requestedKind)
		assert.Equal(t, []byte("{}"), body)
	})

	t.Run("not found", func(t *testing.T) {
		srv := newServer(&stubRepo{})
		defer srv.Close()
		req, _ := http.NewRequest(http.MethodGet, srv.URL+"/v1/apis/a1/postman", nil)
		resp, err := http.DefaultClient.Do(req)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusNotFound, resp.StatusCode)
		data := readAndLogBody(t, resp)
		assert.Contains(t, string(data), "Postman artifact not found")
	})
}

func TestIntegration_GetOas31(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		var requestedKind string
		repo := &stubRepo{getArt: func(ctx context.Context, apiID, kind string) (*models.ApiArtifact, error) {
			requestedKind = kind
			return &models.ApiArtifact{ApiID: apiID, Kind: kind, ContentType: "application/json", Filename: "oas.json", Data: []byte("{\"openapi\":\"3.1.0\"}")}, nil
		}}
		srv := newServer(repo)
		defer srv.Close()
		req, _ := http.NewRequest(http.MethodGet, srv.URL+"/v1/apis/a1/oas31", nil)
		req.Header.Set("x-api-key", "test")
		resp, err := http.DefaultClient.Do(req)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		body := readAndLogBody(t, resp)
		assert.Equal(t, "oas-converted", requestedKind)
		assert.Contains(t, string(body), "openapi")
	})

	t.Run("not found", func(t *testing.T) {
		srv := newServer(&stubRepo{})
		defer srv.Close()
		req, _ := http.NewRequest(http.MethodGet, srv.URL+"/v1/apis/a1/oas31", nil)
		resp, err := http.DefaultClient.Do(req)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusNotFound, resp.StatusCode)
		data := readAndLogBody(t, resp)
		assert.Contains(t, string(data), "OAS 3.1 artifact not found")
	})
}

func TestIntegration_ListOrganisations(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		repo := &stubRepo{getOrgs: func(ctx context.Context) ([]models.Organisation, int, error) {
			return []models.Organisation{{Uri: "https://org1", Label: "Org1"}, {Uri: "https://org2", Label: "Org2"}}, 2, nil
		}}
		srv := newServer(repo)
		defer srv.Close()
		req, _ := http.NewRequest(http.MethodGet, srv.URL+"/v1/organisations", nil)
		req.Header.Set("x-api-key", "test")
		resp, err := http.DefaultClient.Do(req)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		assert.Equal(t, "2", resp.Header.Get("X-Total-Count"))
		var body []models.OrganisationSummary
		decodeJSONBody(t, resp, &body)
		assert.Len(t, body, 2)
	})

	t.Run("error", func(t *testing.T) {
		repo := &stubRepo{getOrgs: func(ctx context.Context) ([]models.Organisation, int, error) {
			return nil, 0, errors.New("db down")
		}}
		srv := newServer(repo)
		defer srv.Close()
		req, _ := http.NewRequest(http.MethodGet, srv.URL+"/v1/organisations", nil)
		req.Header.Set("x-api-key", "test")
		resp, err := http.DefaultClient.Do(req)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
		data := readAndLogBody(t, resp)
		assert.Contains(t, string(data), "db down")
	})
}

func TestIntegration_UpdateApi(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		org := "https://org.example"
		stored := models.Api{Id: "api-1", Organisation: &models.Organisation{Uri: org, Label: "Org"}, OrganisationID: &org, OasHash: "old"}
		var updated models.Api
		repo := &stubRepo{
			getByID: func(ctx context.Context, id string) (*models.Api, error) {
				return &stored, nil
			},
			update: func(ctx context.Context, api models.Api) error {
				updated = api
				return nil
			},
		}
		oas := newMockOASServer(t, `{
  "openapi": "3.0.0",
  "info": {"title": "ok", "version": "1.0.0", "contact": {"name": "n", "email": "test@example.org", "url": "https://example.org"}},
  "paths": {"/ping": {"get": {"responses": {"200": {"description": "pong"}}}}}
}`)
		defer oas.Close()
		srv := newServer(repo)
		defer srv.Close()
		body := fmt.Sprintf(`{"oasUrl":"%s","organisationUri":"%s"}`, oas.URL, org)
		req, _ := http.NewRequest(http.MethodPut, srv.URL+"/v1/apis/"+stored.Id, strings.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+tokenWithScope("apis:write"))
		req.Header.Set("Content-Type", "application/json")
		resp, err := http.DefaultClient.Do(req)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusCreated, resp.StatusCode)
		_ = readAndLogBody(t, resp)
		assert.NotEmpty(t, updated.OasHash)
		assert.Equal(t, stored.Id, updated.Id)
	})

	t.Run("not found", func(t *testing.T) {
		repo := &stubRepo{getByID: func(ctx context.Context, id string) (*models.Api, error) {
			return nil, nil
		}}
		srv := newServer(repo)
		defer srv.Close()
		body := `{"oasUrl":"https://example.org/oas.json","organisationUri":"https://org"}`
		req, _ := http.NewRequest(http.MethodPut, srv.URL+"/v1/apis/api-1", strings.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+tokenWithScope("apis:write"))
		req.Header.Set("Content-Type", "application/json")
		resp, err := http.DefaultClient.Do(req)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusNotFound, resp.StatusCode)
		data := readAndLogBody(t, resp)
		assert.Contains(t, string(data), "moet als nieuwe API geregistreerd")
	})

	t.Run("forbidden", func(t *testing.T) {
		org := "https://org.example"
		stored := models.Api{Id: "api-1", Organisation: &models.Organisation{Uri: org, Label: "Org"}, OrganisationID: &org}
		repo := &stubRepo{getByID: func(ctx context.Context, id string) (*models.Api, error) {
			return &stored, nil
		}}
		srv := newServer(repo)
		defer srv.Close()
		body := `{"oasUrl":"https://example.org/oas.json","organisationUri":"https://ander.org"}`
		req, _ := http.NewRequest(http.MethodPut, srv.URL+"/v1/apis/api-1", strings.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+tokenWithScope("apis:write"))
		req.Header.Set("Content-Type", "application/json")
		resp, err := http.DefaultClient.Do(req)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusForbidden, resp.StatusCode)
		data := readAndLogBody(t, resp)
		assert.Contains(t, string(data), "organisationUri komt niet overeen")
	})

	t.Run("invalid oas", func(t *testing.T) {
		org := "https://org.example"
		stored := models.Api{Id: "api-1", Organisation: &models.Organisation{Uri: org, Label: "Org"}, OrganisationID: &org}
		repo := &stubRepo{getByID: func(ctx context.Context, id string) (*models.Api, error) {
			return &stored, nil
		}}
		broken := newMockOASServer(t, `{"openapi":"3.0.0","info":{},"paths":{}}`)
		defer broken.Close()
		srv := newServer(repo)
		defer srv.Close()
		body := fmt.Sprintf(`{"oasUrl":"%s","organisationUri":"%s"}`, broken.URL, org)
		req, _ := http.NewRequest(http.MethodPut, srv.URL+"/v1/apis/api-1", strings.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+tokenWithScope("apis:write"))
		req.Header.Set("Content-Type", "application/json")
		resp, err := http.DefaultClient.Do(req)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
		data := readAndLogBody(t, resp)
		assert.Contains(t, string(data), "invalid OAS")
	})
}

func TestIntegration_CreateOrganisation(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		var saved models.Organisation
		repo := &stubRepo{saveOrg: func(org *models.Organisation) error { saved = *org; return nil }}
		srv := newServer(repo)
		defer srv.Close()

		body := `{"uri":"https://example.org","label":"Org"}`
		req, _ := http.NewRequest(http.MethodPost, srv.URL+"/v1/organisations", strings.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+tokenWithScope("organisations:write"))
		req.Header.Set("Content-Type", "application/json")
		resp, err := http.DefaultClient.Do(req)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusCreated, resp.StatusCode)
		data := readAndLogBody(t, resp)
		assert.NotContains(t, string(data), "problem")
		assert.Equal(t, "https://example.org", saved.Uri)
	})

	t.Run("invalid url", func(t *testing.T) {
		srv := newServer(&stubRepo{})
		defer srv.Close()
		body := `{"uri":"notaurl","label":"Org"}`
		req, _ := http.NewRequest(http.MethodPost, srv.URL+"/v1/organisations", strings.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+tokenWithScope("organisations:write"))
		req.Header.Set("Content-Type", "application/json")
		resp, err := http.DefaultClient.Do(req)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
		data := readAndLogBody(t, resp)
		assert.Contains(t, string(data), "foutieve uri")
	})

	t.Run("save error", func(t *testing.T) {
		saveErr := errors.New("save failed")
		repo := &stubRepo{saveOrg: func(org *models.Organisation) error { return saveErr }}
		srv := newServer(repo)
		defer srv.Close()
		body := `{"uri":"https://example.org","label":"Org"}`
		req, _ := http.NewRequest(http.MethodPost, srv.URL+"/v1/organisations", strings.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+tokenWithScope("organisations:write"))
		req.Header.Set("Content-Type", "application/json")
		resp, err := http.DefaultClient.Do(req)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
		data := readAndLogBody(t, resp)
		assert.Contains(t, string(data), "save failed")
	})
}

func TestIntegration_CreateApiFromOas_Versions(t *testing.T) {
	t.Run("accepts openapi 3.0", func(t *testing.T) {
		valid300 := newMockOASServer(t, `{
	  "openapi": "3.0.0",
  "info": {"title": "ok", "version": "1.0.0", "contact": {"name": "n", "email": "test@example.org", "url": "https://example.org"}},
	  "paths": {"/ping": {"get": {"responses": {"200": {"description": "pong"}}}}}
	}`)
		defer valid300.Close()

		repo := &stubRepo{
			findByOas: func(ctx context.Context, url string) (*models.Api, error) { return nil, nil },
			saveApi:   func(api *models.Api) error { return nil },
			saveSrv:   func(server models.Server) error { return nil },
			findOrg: func(ctx context.Context, uri string) (*models.Organisation, error) {
				return &models.Organisation{Uri: uri, Label: "Org"}, nil
			},
		}
		srv := newServer(repo)
		defer srv.Close()

		_ = postOAS(t, srv.URL, valid300.URL, http.StatusCreated)
	})

	t.Run("accepts openapi 3.1", func(t *testing.T) {
		valid31 := newMockOASServer(t, `{
	  "openapi": "3.1.0",
  "info": {"title": "ok", "version": "1.0.0", "contact": {"name": "n", "email": "test@example.org", "url": "https://example.org"}},
	  "paths": {"/ping": {"get": {"responses": {"200": {"description": "pong"}}}}}
	}`)
		defer valid31.Close()

		repo := &stubRepo{
			findByOas: func(ctx context.Context, url string) (*models.Api, error) { return nil, nil },
			saveApi:   func(api *models.Api) error { return nil },
			saveSrv:   func(server models.Server) error { return nil },
			findOrg: func(ctx context.Context, uri string) (*models.Organisation, error) {
				return &models.Organisation{Uri: uri, Label: "Org"}, nil
			},
		}
		srv := newServer(repo)
		defer srv.Close()

		_ = postOAS(t, srv.URL, valid31.URL, http.StatusCreated)
	})

	t.Run("rejects invalid 3.0", func(t *testing.T) {
		broken := newMockOASServer(t, `{"openapi": "3.0.0", "info": {"title":"bad"}, "paths": {}}`)
		defer broken.Close()

		repo := &stubRepo{
			findByOas: func(ctx context.Context, url string) (*models.Api, error) { return nil, nil },
			saveApi:   func(api *models.Api) error { return nil },
			saveSrv:   func(server models.Server) error { return nil },
			findOrg: func(ctx context.Context, uri string) (*models.Organisation, error) {
				return &models.Organisation{Uri: uri, Label: "Org"}, nil
			},
		}
		srv := newServer(repo)
		defer srv.Close()

		data := postOAS(t, srv.URL, broken.URL, http.StatusBadRequest)
		assert.Contains(t, string(data), "invalid OAS")
	})

	t.Run("rejects invalid 3.1", func(t *testing.T) {
		broken := newMockOASServer(t, `{"openapi": "3.1.0", "info": {"version":"1.0.0"}, "paths": {}}`)
		defer broken.Close()

		repo := &stubRepo{
			findByOas: func(ctx context.Context, url string) (*models.Api, error) { return nil, nil },
			saveApi:   func(api *models.Api) error { return nil },
			saveSrv:   func(server models.Server) error { return nil },
			findOrg: func(ctx context.Context, uri string) (*models.Organisation, error) {
				return &models.Organisation{Uri: uri, Label: "Org"}, nil
			},
		}
		srv := newServer(repo)
		defer srv.Close()

		data := postOAS(t, srv.URL, broken.URL, http.StatusBadRequest)
		assert.Contains(t, string(data), "invalid OAS")
	})

	t.Run("rejects unsupported 3.2", func(t *testing.T) {
		unsupported := newMockOASServer(t, `{
	  "openapi": "3.2.0",
  "info": {"title": "n", "version": "1.0.0", "contact": {"name":"n","email":"test@example.org","url":"https://example.org"}},
	  "paths": {"/ping": {"get": {"responses": {"200": {"description": "pong"}}}}}
	}`)
		defer unsupported.Close()

		repo := &stubRepo{
			findByOas: func(ctx context.Context, url string) (*models.Api, error) { return nil, nil },
			saveApi:   func(api *models.Api) error { return nil },
			saveSrv:   func(server models.Server) error { return nil },
			findOrg: func(ctx context.Context, uri string) (*models.Organisation, error) {
				return &models.Organisation{Uri: uri, Label: "Org"}, nil
			},
		}
		srv := newServer(repo)
		defer srv.Close()

		data := postOAS(t, srv.URL, unsupported.URL, http.StatusBadRequest)
		assert.Contains(t, string(data), "unsupported OpenAPI version")
	})
}

func newMockOASServer(t *testing.T, payload string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(payload))
	}))
}

func postOAS(t *testing.T, serverURL, oasURL string, expected int) []byte {
	t.Helper()
	body := fmt.Sprintf(`{"oasUrl":"%s","organisationUri":"https://example.org","contact":{"name":"n","email":"test@example.org","url":"https://example.org"}}`, oasURL)
	req, err := http.NewRequest(http.MethodPost, serverURL+"/v1/apis", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer "+tokenWithScope("apis:write"))
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != expected {
		data, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		t.Fatalf("expected status %d, got %d: %s", expected, resp.StatusCode, string(data))
	}
	return readAndLogBody(t, resp)
}

func TestIntegration_OpenAPIDocument(t *testing.T) {
	srv := newServer(&stubRepo{})
	defer srv.Close()
	req, _ := http.NewRequest(http.MethodGet, srv.URL+"/v1/openapi.json", nil)
	resp, err := http.DefaultClient.Do(req)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Contains(t, resp.Header.Get("Content-Type"), "application/json")
	_ = readAndLogBody(t, resp)
}

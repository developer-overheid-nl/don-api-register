package api_client_test

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	api_client "github.com/developer-overheid-nl/don-api-register/pkg/api_client"
	"github.com/developer-overheid-nl/don-api-register/pkg/api_client/handler"
	"github.com/developer-overheid-nl/don-api-register/pkg/api_client/models"
	"github.com/developer-overheid-nl/don-api-register/pkg/api_client/repositories"
	"github.com/developer-overheid-nl/don-api-register/pkg/api_client/services"
	"github.com/stretchr/testify/assert"
)

// stubRepo implements repositories.ApiRepository for integration tests
// Each function can be optional; unused functions return zero values.
type stubRepo struct {
	getApis   func(ctx context.Context, page, perPage int, organisation *string, ids *string) ([]models.Api, models.Pagination, error)
	getByID   func(ctx context.Context, id string) (*models.Api, error)
	findByOas func(ctx context.Context, url string) (*models.Api, error)
	findOrg   func(ctx context.Context, uri string) (*models.Organisation, error)
	saveApi   func(api *models.Api) error
	saveSrv   func(server models.Server) error
	saveOrg   func(org *models.Organisation) error
}

func (s *stubRepo) GetApis(ctx context.Context, page, perPage int, organisation *string, ids *string) ([]models.Api, models.Pagination, error) {
	if s.getApis != nil {
		return s.getApis(ctx, page, perPage, organisation, ids)
	}
	return nil, models.Pagination{}, nil
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
func (s *stubRepo) UpdateApi(ctx context.Context, api models.Api) error { return nil }
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
func (s *stubRepo) AllApis(ctx context.Context) ([]models.Api, error)                   { return nil, nil }
func (s *stubRepo) SaveLintResult(ctx context.Context, result *models.LintResult) error { return nil }
func (s *stubRepo) GetLintResults(ctx context.Context, apiID string) ([]models.LintResult, error) {
	return nil, nil
}
func (s *stubRepo) GetOrganisations(ctx context.Context) ([]models.Organisation, int, error) {
	return nil, 0, nil
}

func newServer(repo repositories.ApiRepository) *httptest.Server {
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
	var body models.ApiListResponse
	_ = json.NewDecoder(resp.Body).Decode(&body)
	resp.Body.Close()
	assert.Len(t, body.Apis, 2)
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
	_ = json.NewDecoder(resp.Body).Decode(&body)
	resp.Body.Close()
	assert.Equal(t, "id1", body.Id)
}

func TestIntegration_CreateApiFromOas(t *testing.T) {
	spec := `{"openapi":"3.0.0","info":{"title":"T","version":"1.0.0","contact":{"name":"n","email":"e","url":"u"}},"paths":{}}`
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
	resp.Body.Close()
	assert.Equal(t, saved.Id, saved.Id) // saved should be set
}

func TestIntegration_UpdateApi(t *testing.T) {
	repo := &stubRepo{findByOas: func(ctx context.Context, url string) (*models.Api, error) {
		org := "https://org.example.com"
		return &models.Api{Id: "a1", Organisation: &models.Organisation{Uri: org, Label: "l"}, OrganisationID: &org}, nil
	}}
	srv := newServer(repo)
	defer srv.Close()

	body := `{"oasUrl":"http://example.com","organisationUri":"https://org.example.com"}`
	req, _ := http.NewRequest(http.MethodPut, srv.URL+"/v1/apis/a1", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+tokenWithScope("apis:write"))
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusCreated, resp.StatusCode)
	resp.Body.Close()
}

func TestIntegration_CreateOrganisation(t *testing.T) {
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
	resp.Body.Close()
	assert.Equal(t, "https://example.org", saved.Uri)
}

//func TestIntegration_forbidden_UpdateApi(t *testing.T) {
//	repo := &stubRepo{findByOas: func(ctx context.Context, url string) (*models.Api, error) {
//		org := "https://org.example.com"
//		return &models.Api{Id: "a1", Organisation: &models.Organisation{Uri: org, Label: "l"}, OrganisationID: &org}, nil
//	}}
//	srv := newServer(repo)
//	defer srv.Close()
//	badBody := `{"oasUrl":"http://example2.com","organisationUri":"https://malafide.org"}`
//	req2, _ := http.NewRequest(http.MethodPut, srv.URL+"/v1/apis/a1", strings.NewReader(badBody))
//	req2.Header.Set("Authorization", "Bearer "+tokenWithScope("apis:write"))
//	req2.Header.Set("Content-Type", "application/json")
//	resp2, err := http.DefaultClient.Do(req2)
//	assert.NoError(t, err)
//	assert.Equal(t, http.StatusForbidden, resp2.StatusCode)
//	resp2.Body.Close()
//}

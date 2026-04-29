package api_client_test

import (
	"bytes"
	"context"
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
	"time"

	api_client "github.com/developer-overheid-nl/don-api-register/pkg/api_client"
	"github.com/developer-overheid-nl/don-api-register/pkg/api_client/handler"
	problem "github.com/developer-overheid-nl/don-api-register/pkg/api_client/helpers/problem"
	"github.com/developer-overheid-nl/don-api-register/pkg/api_client/models"
	"github.com/developer-overheid-nl/don-api-register/pkg/api_client/repositories"
	"github.com/developer-overheid-nl/don-api-register/pkg/api_client/services"
	"github.com/developer-overheid-nl/don-api-register/pkg/api_client/testutil"
	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
	"github.com/loopfz/gadgeto/tonic"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
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
	case "required_without", "required_without_all":
		return "is verplicht wanneer geen OAS of lifecycle-datum is opgegeven"
	case "datetime":
		return "Moet een geldige datum zijn (YYYY-MM-DD)"
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

type integrationEnv struct {
	server  *httptest.Server
	repo    repositories.ApiRepository
	service *services.APIsAPIService
	client  *http.Client
}

func newIntegrationEnv(t *testing.T) *integrationEnv {
	t.Helper()

	gin.SetMode(gin.TestMode)
	setupErrorHook()

	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)

	require.NoError(t, db.AutoMigrate(
		&models.Organisation{},
		&models.Server{},
		&models.Api{},
		&models.LintResult{},
		&models.LintMessage{},
		&models.LintMessageInfo{},
		&models.ApiArtifact{},
	))

	repo := repositories.NewApiRepository(db)
	svc := services.NewAPIsAPIService(repo)
	controller := handler.NewAPIsAPIController(svc)
	router := api_client.NewRouter("test-version", controller)

	server := httptest.NewServer(router)
	t.Cleanup(func() { server.Close() })

	return &integrationEnv{
		server:  server,
		repo:    repo,
		service: svc,
		client:  &http.Client{Timeout: 2 * time.Second},
	}
}

func (e *integrationEnv) doRequest(t *testing.T, method, path string) *http.Response {
	t.Helper()

	req, err := http.NewRequest(method, e.server.URL+path, nil)
	require.NoError(t, err)

	resp, err := e.client.Do(req)
	require.NoError(t, err)
	return resp
}

func (e *integrationEnv) doRequestWithHeaders(t *testing.T, method, path string, headers map[string]string) *http.Response {
	t.Helper()

	req, err := http.NewRequest(method, e.server.URL+path, nil)
	require.NoError(t, err)
	for key, value := range headers {
		req.Header.Set(key, value)
	}

	resp, err := e.client.Do(req)
	require.NoError(t, err)
	return resp
}

func (e *integrationEnv) doJSONRequest(t *testing.T, method, path string, payload any) *http.Response {
	t.Helper()

	var buf bytes.Buffer
	if payload != nil {
		require.NoError(t, json.NewEncoder(&buf).Encode(payload))
	}

	req, err := http.NewRequest(method, e.server.URL+path, &buf)
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := e.client.Do(req)
	require.NoError(t, err)
	return resp
}

func decodeBody[T any](t *testing.T, resp *http.Response) T {
	t.Helper()
	defer func() {
		require.NoError(t, resp.Body.Close())
	}()

	var out T
	data, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	err = json.Unmarshal(data, &out)
	require.NoErrorf(t, err, "body=%s", string(data))
	return out
}

func readRawBody(t *testing.T, resp *http.Response) []byte {
	t.Helper()
	defer func() {
		require.NoError(t, resp.Body.Close())
	}()

	data, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	return data
}

func TestRealtimeApplicationRun(t *testing.T) {
	env := newIntegrationEnv(t)
	ctx := context.Background()

	org, err := env.service.CreateOrganisation(ctx, &models.Organisation{
		Uri:   "https://voorbeelden.example.com/organisaties/realtime",
		Label: "Realtime Org",
	})
	require.NoError(t, err)

	adrScore := 4
	apiID := uuid.NewString()
	api := &models.Api{
		Id:             apiID,
		Title:          "Realtime API",
		Description:    "Geintegreerde test zonder mocks",
		OasUri:         "https://voorbeelden.example.com/apis/realtime/openapi.yaml",
		OasHash:        "hash-123",
		OAS:            models.OASMetadata{Version: "3.1.0"},
		DocsUrl:        "https://voorbeelden.example.com/apis/realtime/docs",
		ContactName:    "Realtime Team",
		ContactEmail:   "realtime@example.com",
		ContactUrl:     "https://voorbeelden.example.com/contact",
		OrganisationID: &org.Uri,
		AdrScore:       &adrScore,
		Version:        "1.2.3",
	}
	require.NoError(t, env.repo.Save(api))

	t.Run("list apis", func(t *testing.T) {
		resp := env.doRequest(t, http.MethodGet, "/v1/apis")
		require.Equal(t, http.StatusOK, resp.StatusCode)
		require.Equal(t, "test-version", resp.Header.Get("API-Version"))
		require.Equal(t, "1", resp.Header.Get("Total-Count"))
		require.Contains(t, resp.Header.Get("Link"), "rel=\"self\"")

		summaries := decodeBody[[]models.ApiSummary](t, resp)
		require.Len(t, summaries, 1)

		summary := summaries[0]
		require.Equal(t, apiID, summary.Id)
		require.Equal(t, "Realtime API", summary.Title)
		require.Equal(t, "Realtime Org", summary.Organisation.Label)
		require.NotNil(t, summary.AdrScore)
		require.Equal(t, adrScore, *summary.AdrScore)
		require.NotNil(t, summary.Links)
		require.NotNil(t, summary.Links.Self)
		require.Equal(t, "/v1/apis/"+apiID, summary.Links.Self.Href)
	})

	t.Run("retrieve api", func(t *testing.T) {
		resp := env.doRequest(t, http.MethodGet, "/v1/apis/"+apiID)
		require.Equal(t, http.StatusOK, resp.StatusCode)
		require.Equal(t, "test-version", resp.Header.Get("API-Version"))

		detail := decodeBody[models.ApiDetail](t, resp)
		require.Equal(t, apiID, detail.Id)
		require.Equal(t, "Realtime API", detail.Title)
		require.Equal(t, "https://voorbeelden.example.com/apis/realtime/docs", detail.DocsUrl)
		require.Equal(t, "Realtime Team", detail.Contact.Name)
		require.Equal(t, "realtime@example.com", detail.Contact.Email)
		require.Equal(t, "Realtime Org", detail.Organisation.Label)
		require.Empty(t, detail.LintResults)
		require.Nil(t, detail.Links)
	})

	t.Run("search apis", func(t *testing.T) {
		resp := env.doRequest(t, http.MethodGet, "/v1/apis/_search?q=Realtime")
		require.Equal(t, http.StatusOK, resp.StatusCode)
		require.Equal(t, "test-version", resp.Header.Get("API-Version"))
		require.Equal(t, "1", resp.Header.Get("Total-Count"))

		results := decodeBody[[]models.ApiSummary](t, resp)
		require.Len(t, results, 1)
		require.Equal(t, apiID, results[0].Id)
	})

	legacyScore := 88
	legacyID := uuid.NewString()
	legacy := &models.Api{
		Id:             legacyID,
		Title:          "Legacy API",
		Description:    "Deprecated API voor filtertests",
		OasUri:         "https://voorbeelden.example.com/apis/legacy/openapi.yaml",
		OasHash:        "hash-legacy",
		OAS:            models.OASMetadata{Version: "3.0.0"},
		ContactName:    "Legacy Team",
		ContactEmail:   "legacy@example.com",
		ContactUrl:     "https://voorbeelden.example.com/legacy-contact",
		OrganisationID: &org.Uri,
		AdrScore:       &legacyScore,
		Version:        "2.0.0",
		Auth:           "oauth2",
		Deprecated:     time.Now().AddDate(0, 0, -1).Format(time.DateOnly),
	}
	require.NoError(t, env.repo.Save(legacy))

	t.Run("list api filters", func(t *testing.T) {
		resp := env.doRequest(t, http.MethodGet, "/v1/apis/filters?status=deprecated")
		require.Equal(t, http.StatusOK, resp.StatusCode)
		require.Equal(t, "test-version", resp.Header.Get("API-Version"))

		groups := decodeBody[[]models.FilterGroup](t, resp)
		require.Len(t, groups, 5)

		byKey := map[string]models.FilterGroup{}
		for _, group := range groups {
			byKey[group.Key] = group
		}
		require.Contains(t, byKey, "organisation")
		require.Contains(t, byKey, "status")
		require.Contains(t, byKey, "oasVersion")
		require.Contains(t, byKey, "adrScore")
		require.Contains(t, byKey, "auth")

		var deprecatedSelected bool
		for _, option := range byKey["status"].Options {
			if option.Value == "deprecated" {
				deprecatedSelected = option.Selected
			}
		}
		require.True(t, deprecatedSelected)
	})

	t.Run("filter apis", func(t *testing.T) {
		resp := env.doRequest(t, http.MethodGet, "/v1/apis?status=deprecated&oasVersion=3.0.0&auth=oauth2&adrScore=88")
		require.Equal(t, http.StatusOK, resp.StatusCode)
		require.Equal(t, "1", resp.Header.Get("Total-Count"))

		summaries := decodeBody[[]models.ApiSummary](t, resp)
		require.Len(t, summaries, 1)
		require.Equal(t, legacyID, summaries[0].Id)
		require.Equal(t, "deprecated", summaries[0].Lifecycle.Status)
		require.Equal(t, "2.0.0", summaries[0].Lifecycle.Version)
	})

	t.Run("list organisations", func(t *testing.T) {
		resp := env.doRequest(t, http.MethodGet, "/v1/organisations")
		require.Equal(t, http.StatusOK, resp.StatusCode)
		require.Equal(t, "1", resp.Header.Get("Total-Count"))

		organisations := decodeBody[[]models.OrganisationSummary](t, resp)
		require.Len(t, organisations, 1)
		require.Equal(t, org.Uri, organisations[0].Uri)
		require.Equal(t, "Realtime Org", organisations[0].Label)
		require.NotNil(t, organisations[0].Links)
		require.NotNil(t, organisations[0].Links.Apis)
		require.Equal(t, "/v1/apis?organisation="+org.Uri, organisations[0].Links.Apis.Href)
	})

	t.Run("create organisation invalid uri", func(t *testing.T) {
		resp := env.doJSONRequest(t, http.MethodPost, "/v1/organisations", map[string]string{
			"uri":   "notaurl",
			"label": "Valid Label",
		})
		require.Equal(t, http.StatusBadRequest, resp.StatusCode)

		prob := decodeBody[problem.APIError](t, resp)
		require.Equal(t, "Request validation failed", prob.Title)
		require.Equal(t, 400, prob.Status)
		require.Len(t, prob.Errors, 1)
		require.Equal(t, "uri", prob.Errors[0].Code)
	})

	t.Run("create organisation missing label", func(t *testing.T) {
		resp := env.doJSONRequest(t, http.MethodPost, "/v1/organisations", map[string]string{
			"uri":   "https://voorbeelden.example.com/organisaties/ongeldig",
			"label": " ",
		})
		require.Equal(t, http.StatusBadRequest, resp.StatusCode)

		prob := decodeBody[problem.APIError](t, resp)
		require.Equal(t, "Request validation failed", prob.Title)
		require.Equal(t, 400, prob.Status)
		require.Len(t, prob.Errors, 1)
		require.Equal(t, "label", prob.Errors[0].Code)
	})

	t.Run("missing api gives problem json", func(t *testing.T) {
		resp := env.doRequest(t, http.MethodGet, "/v1/apis/"+uuid.NewString())
		require.Equal(t, http.StatusNotFound, resp.StatusCode)

		prob := decodeBody[problem.APIError](t, resp)
		require.Equal(t, "Resource Not Found", prob.Title)
		require.Equal(t, 404, prob.Status)
		require.Len(t, prob.Errors, 1)
		require.Contains(t, prob.Errors[0].Detail, "Api not found")
	})

	t.Run("missing artifact returns problem json", func(t *testing.T) {
		resp := env.doRequest(t, http.MethodGet, fmt.Sprintf("/v1/apis/%s/postman", apiID))
		require.Equal(t, http.StatusNotFound, resp.StatusCode)

		prob := decodeBody[problem.APIError](t, resp)
		require.Equal(t, "Resource Not Found", prob.Title)
		require.Equal(t, 404, prob.Status)
		require.Len(t, prob.Errors, 1)
		require.Contains(t, prob.Errors[0].Detail, "Postman artifact not found")
	})
}

func TestUpdateApi_LifecycleOnlyWithoutOAS(t *testing.T) {
	env := newIntegrationEnv(t)
	ctx := context.Background()

	org, err := env.service.CreateOrganisation(ctx, &models.Organisation{
		Uri:   "https://voorbeelden.example.com/organisaties/lifecycle",
		Label: "Lifecycle Org",
	})
	require.NoError(t, err)

	apiID := uuid.NewString()
	api := &models.Api{
		Id:             apiID,
		Title:          "Lifecycle API",
		OasUri:         "https://voorbeelden.example.com/apis/lifecycle/openapi.yaml",
		ContactName:    "Lifecycle Team",
		ContactEmail:   "lifecycle@example.com",
		ContactUrl:     "https://voorbeelden.example.com/contact",
		OrganisationID: &org.Uri,
		Organisation:   org,
		Version:        "1.2.3",
		Sunset:         "2027-01-01",
		Deprecated:     "2026-01-01",
	}
	require.NoError(t, env.repo.Save(api))

	t.Run("set lifecycle fields without oas", func(t *testing.T) {
		resp := env.doJSONRequest(t, http.MethodPut, "/v1/apis/"+apiID, map[string]string{
			"organisationUri": org.Uri,
			"sunset":          "2028-02-02",
			"deprecated":      "2027-02-02",
		})
		require.Equal(t, http.StatusOK, resp.StatusCode)

		summary := decodeBody[models.ApiSummary](t, resp)
		require.Equal(t, "2028-02-02", summary.Lifecycle.Sunset)
		require.Equal(t, "2027-02-02", summary.Lifecycle.Deprecated)

		saved, err := env.repo.GetApiByID(ctx, apiID)
		require.NoError(t, err)
		require.Equal(t, "2028-02-02", saved.Sunset)
		require.Equal(t, "2027-02-02", saved.Deprecated)
		require.Equal(t, "https://voorbeelden.example.com/apis/lifecycle/openapi.yaml", saved.OasUri)
	})

	t.Run("clear sunset without oas", func(t *testing.T) {
		resp := env.doJSONRequest(t, http.MethodPut, "/v1/apis/"+apiID, map[string]any{
			"organisationUri": org.Uri,
			"sunset":          nil,
		})
		require.Equal(t, http.StatusOK, resp.StatusCode)

		summary := decodeBody[models.ApiSummary](t, resp)
		require.Empty(t, summary.Lifecycle.Sunset)

		saved, err := env.repo.GetApiByID(ctx, apiID)
		require.NoError(t, err)
		require.Empty(t, saved.Sunset)
		require.Equal(t, "2027-02-02", saved.Deprecated)
	})
}

func TestOpenAPIJSONEndpoint(t *testing.T) {
	env := newIntegrationEnv(t)

	resp := env.doRequest(t, http.MethodGet, "/v1/openapi.json")
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Equal(t, "test-version", resp.Header.Get("API-Version"))

	body := readRawBody(t, resp)
	require.Contains(t, string(body), `"openapi"`)
	require.Contains(t, string(body), `"paths"`)
}

func TestRetrieveApiJsonLdEndpoint_SuccessAndNotFound(t *testing.T) {
	env := newIntegrationEnv(t)
	ctx := context.Background()

	org, err := env.service.CreateOrganisation(ctx, &models.Organisation{
		Uri:   "https://voorbeelden.example.com/organisaties/jsonld",
		Label: "JSON-LD Org",
	})
	require.NoError(t, err)

	apiID := uuid.NewString()
	require.NoError(t, env.repo.Save(&models.Api{
		Id:             apiID,
		OasUri:         "https://voorbeelden.example.com/apis/jsonld/openapi.json",
		Title:          "JSON-LD API",
		Description:    "JSON-LD beschrijving",
		ContactName:    "JSON-LD Team",
		ContactEmail:   "jsonld@example.com",
		ContactUrl:     "https://voorbeelden.example.com/contact",
		OrganisationID: &org.Uri,
		Organisation:   org,
		OAS:            models.OASMetadata{Version: "3.1.0"},
	}))

	t.Run("success", func(t *testing.T) {
		resp := env.doRequestWithHeaders(t, http.MethodGet, "/v1/apis/"+apiID, map[string]string{
			"Accept": "application/ld+json",
		})
		require.Equal(t, http.StatusOK, resp.StatusCode)
		require.Equal(t, "test-version", resp.Header.Get("API-Version"))
		require.Contains(t, resp.Header.Get("Content-Type"), "application/ld+json")

		body := decodeBody[models.ApiDetailJsonLd](t, resp)
		require.Equal(t, "dcat:DataService", body.Type)
		require.Equal(t, apiID, body.Identifier)
		require.Equal(t, "JSON-LD API", body.Title)
		require.Equal(t, "JSON-LD beschrijving", body.Description)
		require.Equal(t, "https://voorbeelden.example.com/apis/jsonld/openapi.json", body.EndpointDescription)
		require.Equal(t, "JSON-LD Team", body.ContactPoint.FN)
		require.Equal(t, "mailto:jsonld@example.com", body.ContactPoint.HasEmail)
		require.Equal(t, "https://voorbeelden.example.com/contact", body.ContactPoint.HasURL)
		require.Equal(t, org.Uri, body.Publisher)
		require.Equal(t, []string{"https://spec.openapis.org/oas/v3.1.0.html"}, body.ConformsTo)
	})

	t.Run("missing api", func(t *testing.T) {
		resp := env.doRequestWithHeaders(t, http.MethodGet, "/v1/apis/"+uuid.NewString(), map[string]string{
			"Accept": "application/ld+json",
		})
		require.Equal(t, http.StatusNotFound, resp.StatusCode)

		prob := decodeBody[problem.APIError](t, resp)
		require.Equal(t, 404, prob.Status)
		require.Contains(t, prob.Errors[0].Detail, "Api not found")
	})
}

func TestListLintResultsEndpoint(t *testing.T) {
	env := newIntegrationEnv(t)
	ctx := context.Background()

	org, err := env.service.CreateOrganisation(ctx, &models.Organisation{
		Uri:   "https://voorbeelden.example.com/organisaties/lint",
		Label: "Lint Org",
	})
	require.NoError(t, err)

	apiID := uuid.NewString()
	require.NoError(t, env.repo.Save(&models.Api{
		Id:             apiID,
		OasUri:         "https://voorbeelden.example.com/apis/lint/openapi.json",
		Title:          "Lint API",
		ContactName:    "Lint Team",
		ContactEmail:   "lint@example.com",
		ContactUrl:     "https://voorbeelden.example.com/contact",
		OrganisationID: &org.Uri,
		Organisation:   org,
	}))

	createdAt := time.Now().UTC().Truncate(time.Second)
	require.NoError(t, env.repo.SaveLintResult(ctx, &models.LintResult{
		ID:        uuid.NewString(),
		ApiID:     apiID,
		Successes: false,
		Failures:  1,
		Warnings:  2,
		CreatedAt: createdAt,
		Messages: []models.LintMessage{
			{
				ID:             uuid.NewString(),
				Line:           12,
				Column:         4,
				Severity:       "warning",
				Code:           "adr-001",
				RulesetVersion: "2026.04",
				CreatedAt:      createdAt,
				Infos: []models.LintMessageInfo{
					{
						ID:      uuid.NewString(),
						Message: "Gebruik een beschrijving",
						Path:    "$.info.description",
					},
				},
			},
		},
	}))

	resp := env.doRequest(t, http.MethodGet, "/v1/lint-results")
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Equal(t, "test-version", resp.Header.Get("API-Version"))

	results := decodeBody[[]models.LintResult](t, resp)
	require.Len(t, results, 1)
	require.Equal(t, apiID, results[0].ApiID)
	require.Len(t, results[0].Messages, 1)
	require.Equal(t, "adr-001", results[0].Messages[0].Code)
	require.Equal(t, "2026.04", results[0].Messages[0].RulesetVersion)
	require.Len(t, results[0].Messages[0].Infos, 1)
	require.Equal(t, "$.info.description", results[0].Messages[0].Infos[0].Path)
}

func TestPostmanEndpoint_Success(t *testing.T) {
	env := newIntegrationEnv(t)
	ctx := context.Background()

	org, err := env.service.CreateOrganisation(ctx, &models.Organisation{
		Uri:   "https://voorbeelden.example.com/organisaties/postman",
		Label: "Postman Org",
	})
	require.NoError(t, err)

	apiID := uuid.NewString()
	require.NoError(t, env.repo.Save(&models.Api{
		Id:             apiID,
		OasUri:         "https://voorbeelden.example.com/apis/postman/openapi.json",
		Title:          "Postman API",
		ContactName:    "Postman Team",
		ContactEmail:   "postman@example.com",
		ContactUrl:     "https://voorbeelden.example.com/contact",
		OrganisationID: &org.Uri,
		Organisation:   org,
	}))
	require.NoError(t, env.repo.SaveArtifact(ctx, &models.ApiArtifact{
		ID:          uuid.NewString(),
		ApiID:       apiID,
		Kind:        "postman",
		Filename:    "postman.json",
		ContentType: "application/json",
		Data:        []byte(`{"info":{"name":"Postman API"}}`),
		CreatedAt:   time.Now(),
	}))

	resp := env.doRequest(t, http.MethodGet, "/v1/apis/"+apiID+"/postman")
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Equal(t, "test-version", resp.Header.Get("API-Version"))
	require.Equal(t, "application/json", resp.Header.Get("Content-Type"))
	require.Contains(t, resp.Header.Get("Content-Disposition"), `filename="postman.json"`)
	require.JSONEq(t, `{"info":{"name":"Postman API"}}`, string(readRawBody(t, resp)))
}

func TestOASEndpoint_SuccessAndErrors(t *testing.T) {
	env := newIntegrationEnv(t)
	ctx := context.Background()

	org, err := env.service.CreateOrganisation(ctx, &models.Organisation{
		Uri:   "https://voorbeelden.example.com/organisaties/oas",
		Label: "OAS Org",
	})
	require.NoError(t, err)

	apiID := uuid.NewString()
	require.NoError(t, env.repo.Save(&models.Api{
		Id:             apiID,
		OasUri:         "https://voorbeelden.example.com/apis/oas/openapi.json",
		Title:          "OAS API",
		ContactName:    "OAS Team",
		ContactEmail:   "oas@example.com",
		ContactUrl:     "https://voorbeelden.example.com/contact",
		OrganisationID: &org.Uri,
		Organisation:   org,
	}))
	require.NoError(t, env.repo.SaveArtifact(ctx, &models.ApiArtifact{
		ID:          uuid.NewString(),
		ApiID:       apiID,
		Kind:        "oas",
		Version:     "3.1",
		Format:      "json",
		Source:      "converted",
		Filename:    "oas-3.1-converted.json",
		ContentType: "application/json",
		Data:        []byte(`{"openapi":"3.1.0"}`),
		CreatedAt:   time.Now(),
	}))

	t.Run("success", func(t *testing.T) {
		resp := env.doRequest(t, http.MethodGet, "/v1/apis/"+apiID+"/oas/3.1.json")
		require.Equal(t, http.StatusOK, resp.StatusCode)
		require.Equal(t, "test-version", resp.Header.Get("API-Version"))
		require.Equal(t, "3.1", resp.Header.Get("OAS-Version"))
		require.Equal(t, "converted", resp.Header.Get("OAS-Source"))
		require.Equal(t, "application/json", resp.Header.Get("Content-Type"))
		require.Contains(t, resp.Header.Get("Content-Disposition"), `filename="oas-3.1-converted.json"`)
		require.JSONEq(t, `{"openapi":"3.1.0"}`, string(readRawBody(t, resp)))
	})

	t.Run("invalid version", func(t *testing.T) {
		resp := env.doRequest(t, http.MethodGet, "/v1/apis/"+apiID+"/oas/3.2.json")
		require.Equal(t, http.StatusBadRequest, resp.StatusCode)
		prob := decodeBody[problem.APIError](t, resp)
		require.Equal(t, 400, prob.Status)
	})

	t.Run("missing artifact", func(t *testing.T) {
		resp := env.doRequest(t, http.MethodGet, "/v1/apis/"+apiID+"/oas/3.0.yaml")
		require.Equal(t, http.StatusNotFound, resp.StatusCode)
		prob := decodeBody[problem.APIError](t, resp)
		require.Equal(t, 404, prob.Status)
		require.Contains(t, prob.Errors[0].Detail, "OAS artifact not found")
	})
}

func TestCreateOrganisationEndpoint_Success(t *testing.T) {
	env := newIntegrationEnv(t)

	resp := env.doJSONRequest(t, http.MethodPost, "/v1/organisations", map[string]string{
		"uri":   "https://voorbeelden.example.com/organisaties/nieuw",
		"label": "Nieuwe Org",
	})
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	require.Equal(t, "test-version", resp.Header.Get("API-Version"))

	org := decodeBody[models.Organisation](t, resp)
	require.Equal(t, "https://voorbeelden.example.com/organisaties/nieuw", org.Uri)
	require.Equal(t, "Nieuwe Org", org.Label)
}

func TestCreateApiEndpoint_SuccessAndErrors(t *testing.T) {
	env := newIntegrationEnv(t)
	ctx := context.Background()

	org, err := env.service.CreateOrganisation(ctx, &models.Organisation{
		Uri:   "https://voorbeelden.example.com/organisaties/create-api",
		Label: "Create API Org",
	})
	require.NoError(t, err)

	spec := `{
  "openapi": "3.0.0",
  "info": {
    "title": "Created API",
    "version": "1.0.0",
    "contact": {
      "name": "API Team",
      "email": "api@example.com",
      "url": "https://example.com/contact"
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
	oasSrv := testutil.NewTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(spec))
	}))

	t.Run("success", func(t *testing.T) {
		resp := env.doJSONRequest(t, http.MethodPost, "/v1/apis", map[string]any{
			"oasUrl":          oasSrv.URL,
			"organisationUri": org.Uri,
		})
		require.Equal(t, http.StatusCreated, resp.StatusCode)
		require.Equal(t, "test-version", resp.Header.Get("API-Version"))

		summary := decodeBody[models.ApiSummary](t, resp)
		require.NotEmpty(t, summary.Id)
		require.Equal(t, "Created API", summary.Title)
		require.Equal(t, org.Uri, summary.Organisation.Uri)

		saved, err := env.repo.GetApiByID(ctx, summary.Id)
		require.NoError(t, err)
		require.Equal(t, "Created API", saved.Title)
		require.Equal(t, oasSrv.URL, saved.OasUri)
		require.Equal(t, "1.0.0", saved.Version)
	})

	t.Run("validation error", func(t *testing.T) {
		resp := env.doJSONRequest(t, http.MethodPost, "/v1/apis", map[string]any{
			"organisationUri": org.Uri,
		})
		require.Equal(t, http.StatusBadRequest, resp.StatusCode)
		prob := decodeBody[problem.APIError](t, resp)
		require.Equal(t, 400, prob.Status)
	})

	t.Run("duplicate api", func(t *testing.T) {
		resp := env.doJSONRequest(t, http.MethodPost, "/v1/apis", map[string]any{
			"oasUrl":          oasSrv.URL,
			"organisationUri": org.Uri,
		})
		require.Equal(t, http.StatusBadRequest, resp.StatusCode)
		prob := decodeBody[problem.APIError](t, resp)
		require.Equal(t, 400, prob.Status)
	})
}

func TestUpdateApiEndpoint_OASSuccessAndErrors(t *testing.T) {
	env := newIntegrationEnv(t)
	ctx := context.Background()

	org, err := env.service.CreateOrganisation(ctx, &models.Organisation{
		Uri:   "https://voorbeelden.example.com/organisaties/update-api",
		Label: "Update API Org",
	})
	require.NoError(t, err)

	apiID := uuid.NewString()
	require.NoError(t, env.repo.Save(&models.Api{
		Id:             apiID,
		OasUri:         "https://voorbeelden.example.com/apis/update-api/old.json",
		OasHash:        "outdated",
		Title:          "Old API",
		ContactName:    "Old Team",
		ContactEmail:   "old@example.com",
		ContactUrl:     "https://example.com/old-contact",
		OrganisationID: &org.Uri,
		Organisation:   org,
		Version:        "0.9.0",
	}))

	spec := `{
  "openapi": "3.0.0",
  "info": {
    "title": "Updated API",
    "version": "2.0.0",
    "contact": {
      "name": "Updated Team",
      "email": "updated@example.com",
      "url": "https://example.com/updated-contact"
    }
  },
  "externalDocs": {
    "url": "https://example.com/docs"
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
	oasSrv := testutil.NewTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(spec))
	}))

	t.Run("success", func(t *testing.T) {
		resp := env.doJSONRequest(t, http.MethodPut, "/v1/apis/"+apiID, map[string]any{
			"oasUrl":          oasSrv.URL,
			"organisationUri": org.Uri,
		})
		require.Equal(t, http.StatusOK, resp.StatusCode)
		require.Equal(t, "test-version", resp.Header.Get("API-Version"))

		summary := decodeBody[models.ApiSummary](t, resp)
		require.Equal(t, apiID, summary.Id)
		require.Equal(t, "Updated API", summary.Title)
		require.Equal(t, "2.0.0", summary.Lifecycle.Version)

		saved, err := env.repo.GetApiByID(ctx, apiID)
		require.NoError(t, err)
		require.Equal(t, "Updated API", saved.Title)
		require.Equal(t, "2.0.0", saved.Version)
		require.Equal(t, "Updated Team", saved.ContactName)
		require.Equal(t, oasSrv.URL, saved.OasUri)
	})

	t.Run("invalid lifecycle date", func(t *testing.T) {
		resp := env.doJSONRequest(t, http.MethodPut, "/v1/apis/"+apiID, map[string]any{
			"organisationUri": org.Uri,
			"sunset":          "morgen",
		})
		require.Equal(t, http.StatusBadRequest, resp.StatusCode)
		prob := decodeBody[problem.APIError](t, resp)
		require.Equal(t, 400, prob.Status)
	})

	t.Run("missing api", func(t *testing.T) {
		resp := env.doJSONRequest(t, http.MethodPut, "/v1/apis/"+uuid.NewString(), map[string]any{
			"oasUrl":          oasSrv.URL,
			"organisationUri": org.Uri,
		})
		require.Equal(t, http.StatusNotFound, resp.StatusCode)
		prob := decodeBody[problem.APIError](t, resp)
		require.Equal(t, 404, prob.Status)
	})
}

func TestListAndSearchEndpoints_InvalidPagination(t *testing.T) {
	env := newIntegrationEnv(t)

	t.Run("list apis invalid page", func(t *testing.T) {
		resp := env.doRequest(t, http.MethodGet, "/v1/apis?page=abc")
		require.Equal(t, http.StatusBadRequest, resp.StatusCode)
		prob := decodeBody[problem.APIError](t, resp)
		require.Equal(t, 400, prob.Status)
	})

	t.Run("search apis invalid page", func(t *testing.T) {
		resp := env.doRequest(t, http.MethodGet, "/v1/apis/_search?q=test&page=abc")
		require.Equal(t, http.StatusBadRequest, resp.StatusCode)
		prob := decodeBody[problem.APIError](t, resp)
		require.Equal(t, 400, prob.Status)
	})
}

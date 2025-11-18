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
	defer resp.Body.Close()

	var out T
	data, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	err = json.Unmarshal(data, &out)
	require.NoErrorf(t, err, "body=%s", string(data))
	return out
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
		require.Equal(t, "1", resp.Header.Get("X-Total-Count"))
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
		require.Equal(t, "1", resp.Header.Get("X-Total-Count"))

		results := decodeBody[[]models.ApiSummary](t, resp)
		require.Len(t, results, 1)
		require.Equal(t, apiID, results[0].Id)
	})

	t.Run("list organisations", func(t *testing.T) {
		resp := env.doRequest(t, http.MethodGet, "/v1/organisations")
		require.Equal(t, http.StatusOK, resp.StatusCode)
		require.Equal(t, "1", resp.Header.Get("X-Total-Count"))

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
		require.Equal(t, "Bad Request", prob.Title)
		require.Equal(t, 400, prob.Status)
		require.Len(t, prob.InvalidParams, 1)
		require.Equal(t, "uri", prob.InvalidParams[0].Name)
	})

	t.Run("create organisation missing label", func(t *testing.T) {
		resp := env.doJSONRequest(t, http.MethodPost, "/v1/organisations", map[string]string{
			"uri":   "https://voorbeelden.example.com/organisaties/ongeldig",
			"label": " ",
		})
		require.Equal(t, http.StatusBadRequest, resp.StatusCode)

		prob := decodeBody[problem.APIError](t, resp)
		require.Equal(t, "Bad Request", prob.Title)
		require.Equal(t, 400, prob.Status)
		require.Len(t, prob.InvalidParams, 1)
		require.Equal(t, "label", prob.InvalidParams[0].Name)
	})

	t.Run("missing api gives problem json", func(t *testing.T) {
		resp := env.doRequest(t, http.MethodGet, "/v1/apis/"+uuid.NewString())
		require.Equal(t, http.StatusNotFound, resp.StatusCode)

		prob := decodeBody[problem.APIError](t, resp)
		require.Equal(t, "Not Found", prob.Title)
		require.Equal(t, 404, prob.Status)
		require.Contains(t, prob.Detail, "Api not found")
	})

	t.Run("missing artifact returns problem json", func(t *testing.T) {
		resp := env.doRequest(t, http.MethodGet, fmt.Sprintf("/v1/apis/%s/postman", apiID))
		require.Equal(t, http.StatusNotFound, resp.StatusCode)

		prob := decodeBody[problem.APIError](t, resp)
		require.Equal(t, "Not Found", prob.Title)
		require.Equal(t, 404, prob.Status)
		require.Contains(t, prob.Detail, "Postman artifact not found")
	})
}

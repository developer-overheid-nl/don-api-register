package handler

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"testing"

	problem "github.com/developer-overheid-nl/don-api-register/pkg/api_client/helpers/problem"
	"github.com/developer-overheid-nl/don-api-register/pkg/api_client/models"
	"github.com/developer-overheid-nl/don-api-register/pkg/api_client/services"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAcceptsJsonLd(t *testing.T) {
	t.Run("accepts explicit jsonld", func(t *testing.T) {
		assert.True(t, AcceptsJsonLd("application/ld+json"))
	})

	t.Run("accepts jsonld between other media types", func(t *testing.T) {
		assert.True(t, AcceptsJsonLd("application/json, application/ld+json; q=0.9"))
	})

	t.Run("rejects plain json", func(t *testing.T) {
		assert.False(t, AcceptsJsonLd("application/json"))
	})

	t.Run("rejects wildcard only", func(t *testing.T) {
		assert.False(t, AcceptsJsonLd("*/*"))
	})
}

func TestRetrieveApiJsonLd_Handler(t *testing.T) {
	repo := &stubRepo{
		retrFunc: func(ctx context.Context, id string) (*models.Api, error) {
			return &models.Api{
				Id:           id,
				Title:        "JSON-LD API",
				Description:  "Beschrijving",
				OasUri:       "https://example.com/openapi.json",
				ContactName:  "API Team",
				ContactEmail: "team@example.com",
				ContactUrl:   "https://example.com/contact",
				OAS:          models.OASMetadata{Version: "3.1.0"},
				Organisation: &models.Organisation{
					Uri:   "https://example.com/orgs/1",
					Label: "Org 1",
				},
			}, nil
		},
		lintResFunc: func(ctx context.Context, apiID string) ([]models.LintResult, error) {
			return nil, nil
		},
	}
	svc := services.NewAPIsAPIService(repo)
	ctrl := NewAPIsAPIController(svc)

	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)
	req := httptest.NewRequest("GET", "/v1/apis/api-1", nil)
	req.Header.Set("Accept", "application/ld+json")
	ctx.Request = req

	err := ctrl.RetrieveApiJsonLd(ctx, &models.ApiParams{Id: "api-1"})
	require.NoError(t, err)
	assert.Equal(t, 200, w.Code)
	assert.Contains(t, w.Header().Get("Content-Type"), "application/ld+json")

	var body models.ApiDetailJsonLd
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, "dcat:DataService", body.Type)
	assert.Equal(t, "api-1", body.Identifier)
	assert.Equal(t, "JSON-LD API", body.Title)
	assert.Equal(t, "Beschrijving", body.Description)
	assert.Equal(t, "https://example.com/openapi.json", body.EndpointDescription)
	assert.Equal(t, "API Team", body.ContactPoint.FN)
	assert.Equal(t, "mailto:team@example.com", body.ContactPoint.HasEmail)
	assert.Equal(t, "https://example.com/contact", body.ContactPoint.HasURL)
	assert.Equal(t, "https://example.com/orgs/1", body.Publisher)
	assert.Equal(t, []string{"https://spec.openapis.org/oas/v3.1.0.html"}, body.ConformsTo)
}

func TestRetrieveApiJsonLd_NotFound(t *testing.T) {
	repo := &stubRepo{
		retrFunc: func(ctx context.Context, id string) (*models.Api, error) {
			return nil, nil
		},
	}
	svc := services.NewAPIsAPIService(repo)
	ctrl := NewAPIsAPIController(svc)

	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)
	req := httptest.NewRequest("GET", "/v1/apis/missing", nil)
	req.Header.Set("Accept", "application/ld+json")
	ctx.Request = req

	err := ctrl.RetrieveApiJsonLd(ctx, &models.ApiParams{Id: "missing"})
	apiErr, ok := err.(problem.APIError)
	require.True(t, ok)
	assert.Equal(t, 404, apiErr.Status)
	if assert.Len(t, apiErr.Errors, 1) {
		assert.Contains(t, apiErr.Errors[0].Detail, "Api not found")
	}
}

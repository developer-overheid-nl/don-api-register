package openapi

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/developer-overheid-nl/don-api-register/pkg/api_client/models"
	"github.com/pb33f/libopenapi"
	v3 "github.com/pb33f/libopenapi/datamodel/high/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func mustBuildV3Document(t *testing.T, spec string) *v3.Document {
	t.Helper()

	doc, err := libopenapi.NewDocument([]byte(spec))
	require.NoError(t, err)
	model, errs := doc.BuildV3Model()
	require.NoError(t, errs)
	return &model.Model
}

func TestCorsGetSendsOriginHeader(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "https://developer.overheid.nl", r.Header.Get("Origin"))
		_, _ = w.Write([]byte("ok"))
	}))
	t.Cleanup(server.Close)

	resp, err := CorsGet(server.Client(), server.URL, "https://developer.overheid.nl")

	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestAuthTypeFromSpec(t *testing.T) {
	tests := []struct {
		name string
		spec string
		want string
	}{
		{
			name: "api key",
			spec: `{"openapi":"3.1.0","info":{"title":"T","version":"1"},"paths":{},"components":{"securitySchemes":{"ApiKey":{"type":"apiKey","in":"header","name":"X-API-Key"}}}}`,
			want: "api_key",
		},
		{
			name: "http bearer",
			spec: `{"openapi":"3.1.0","info":{"title":"T","version":"1"},"paths":{},"components":{"securitySchemes":{"Bearer":{"type":"http","scheme":"bearer"}}}}`,
			want: "bearer",
		},
		{
			name: "oauth2",
			spec: `{"openapi":"3.1.0","info":{"title":"T","version":"1"},"paths":{},"components":{"securitySchemes":{"OAuth":{"type":"oauth2","flows":{}}}}}`,
			want: "oauth2",
		},
		{
			name: "openid",
			spec: `{"openapi":"3.1.0","info":{"title":"T","version":"1"},"paths":{},"components":{"securitySchemes":{"OIDC":{"type":"openIdConnect","openIdConnectUrl":"https://example.org/.well-known/openid-configuration"}}}}`,
			want: "openid",
		},
		{
			name: "no auth",
			spec: `{"openapi":"3.1.0","info":{"title":"T","version":"1"},"paths":{}}`,
			want: "",
		},
	}

	for _, tc := range tests {
		current := tc
		t.Run(current.name, func(t *testing.T) {
			got := AuthTypeFromSpec(mustBuildV3Document(t, current.spec))
			assert.Equal(t, current.want, got)
		})
	}
}

func TestBuildApiAndUpdateApiFromSpec(t *testing.T) {
	spec := mustBuildV3Document(t, `{
	  "openapi": "3.1.0",
	  "info": {
	    "title": "Spec title",
	    "summary": "",
	    "description": "**Spec** description",
	    "version": "1.0.0",
	    "contact": {
	      "name": "Spec Contact",
	      "email": "spec@example.org",
	      "url": "https://example.org/contact"
	    },
	    "x-sunset": "2027-01-01",
	    "x-deprecated": "2026-01-01"
	  },
	  "externalDocs": {"url": "https://example.org/docs"},
	  "servers": [{"url": "https://api.example.org", "description": "Production"}],
	  "components": {"securitySchemes": {"Bearer": {"type": "http", "scheme": "bearer"}}},
	  "paths": {}
	}`)
	body := models.ApiPost{
		OasUrl:          "https://example.org/openapi.json",
		OrganisationUri: "https://example.org/org",
		Contact: models.Contact{
			Name:  "Fallback Contact",
			Email: "fallback@example.org",
			URL:   "https://example.org/fallback",
		},
	}

	api := BuildApi(spec, body, "Example Org")

	require.NotEmpty(t, api.Id)
	assert.Equal(t, "Spec title", api.Title)
	assert.Equal(t, "Spec description", api.Summary)
	assert.Equal(t, "**Spec** description", api.Description)
	assert.Equal(t, "Spec Contact", api.ContactName)
	assert.Equal(t, "spec@example.org", api.ContactEmail)
	assert.Equal(t, "https://example.org/contact", api.ContactUrl)
	assert.Equal(t, "1.0.0", api.Version)
	assert.Equal(t, "2027-01-01", api.Sunset)
	assert.Equal(t, "2026-01-01", api.Deprecated)
	assert.Equal(t, "https://example.org/openapi.json", api.OasUri)
	assert.Equal(t, "https://example.org/org", *api.OrganisationID)
	assert.Equal(t, "Example Org", api.Organisation.Label)
	assert.Equal(t, "https://example.org/docs", api.DocsUrl)
	assert.Equal(t, "bearer", api.Auth)
	require.Len(t, api.Servers, 1)
	assert.Equal(t, "https://api.example.org", api.Servers[0].Uri)

	UpdateApiFromSpec(api, nil, models.ApiPost{OrganisationUri: " "}, "")
	assert.Empty(t, api.Title)
	assert.Nil(t, api.Organisation)
	assert.Empty(t, api.Servers)
}

func TestBuildApiFallsBackToRequestContact(t *testing.T) {
	spec := mustBuildV3Document(t, `{"openapi":"3.1.0","info":{"title":"T","version":"1"},"paths":{}}`)

	api := BuildApi(spec, models.ApiPost{
		OasBody:         `{"openapi":"3.1.0"}`,
		OrganisationUri: "https://example.org/org",
		Contact: models.Contact{
			Name:  "Request Contact",
			Email: "request@example.org",
			URL:   "https://example.org/request",
		},
	}, "Org")

	assert.Equal(t, "Request Contact", api.ContactName)
	assert.Equal(t, "request@example.org", api.ContactEmail)
	assert.Equal(t, "https://example.org/request", api.ContactUrl)
}

func TestValidateApi(t *testing.T) {
	invalids := ValidateApi(&models.Api{})

	require.Len(t, invalids, 5)
	assert.Equal(t, "contact.name", invalids[0].Name)
	assert.Equal(t, "contact.email", invalids[1].Name)
	assert.Equal(t, "contact.url", invalids[2].Name)
	assert.Equal(t, "oasUrl", invalids[3].Name)
	assert.Equal(t, "organisationUri", invalids[4].Name)

	orgID := "https://example.org/org"
	valids := ValidateApi(&models.Api{
		ContactName:    "Team",
		ContactEmail:   "team@example.org",
		ContactUrl:     "https://example.org/contact",
		OasUri:         "https://example.org/openapi.json",
		OrganisationID: &orgID,
	})
	assert.Empty(t, valids)
}

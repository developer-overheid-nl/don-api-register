package util

import (
	"testing"
	"time"

	"github.com/developer-overheid-nl/don-api-register/pkg/api_client/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestToApiSummaryAndDetail(t *testing.T) {
	score := 87
	orgID := "https://example.org/org"
	api := &models.Api{
		Id:             "api-1",
		OasUri:         "https://example.org/openapi.json",
		Title:          "Example API",
		Summary:        "",
		Description:    "**Markdown** description",
		ContactName:    "API Team",
		ContactEmail:   "api@example.org",
		ContactUrl:     "https://example.org/contact",
		OrganisationID: &orgID,
		Organisation:   &models.Organisation{Uri: orgID, Label: "Example Org"},
		AdrScore:       &score,
		Version:        "1.2.3",
		Deprecated:     time.Now().AddDate(0, 0, -1).Format(time.DateOnly),
		Auth:           "oauth2",
		DocsUrl:        "https://example.org/docs",
		OAS:            models.OASMetadata{Version: "3.1.0"},
		Servers: []models.Server{
			{Uri: "https://api.example.org", Description: "Production"},
		},
	}

	summary := ToApiSummary(api)

	require.NotNil(t, summary.Summary)
	assert.Equal(t, "Markdown description", *summary.Summary)
	assert.Equal(t, "deprecated", summary.Lifecycle.Status)
	assert.Equal(t, "1.2.3", summary.Lifecycle.Version)
	assert.Equal(t, score, *summary.AdrScore)
	assert.Equal(t, "/v1/apis/api-1", summary.Links.Self.Href)
	assert.Equal(t, "/v1/apis?organisation="+orgID, summary.Organisation.Links.Apis.Href)

	detail := ToApiDetail(api)

	require.NotNil(t, detail)
	assert.Nil(t, detail.Links)
	assert.Equal(t, []string{"oauth2"}, detail.Auth)
	assert.Equal(t, "https://example.org/docs", detail.DocsUrl)
	assert.Equal(t, "3.1.0", detail.OasVersion)
	require.Len(t, detail.Servers, 1)
	assert.Equal(t, "https://api.example.org", detail.Servers[0].Url)
	assert.Equal(t, "Production", detail.Servers[0].Description)
}

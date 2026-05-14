package util

import (
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestApiFeedURL_UsesPublicAPIBaseURL(t *testing.T) {
	t.Setenv("PUBLIC_API_BASE_URL", "https://api.don.projects.digilab.network/api-register/v1")

	got, err := ApiFeedURL("api-1")
	assert.NoError(t, err)
	assert.Equal(t,
		"https://api.don.projects.digilab.network/api-register/v1/apis/api-1/feed",
		got,
	)
}

func TestApiFeedURL_RequiresPublicAPIBaseURL(t *testing.T) {
	t.Setenv("PUBLIC_API_BASE_URL", "")

	got, err := ApiFeedURL("api-1")
	assert.EqualError(t, err, "PUBLIC_API_BASE_URL is niet ingesteld")
	assert.Empty(t, got)
}

func TestApiFeedURL_TrimsTrailingSlash(t *testing.T) {
	t.Setenv("PUBLIC_API_BASE_URL", "https://api.don.projects.digilab.network/api-register/v1/")

	got, err := ApiFeedURL("api-1")
	assert.NoError(t, err)
	assert.Equal(t,
		"https://api.don.projects.digilab.network/api-register/v1/apis/api-1/feed",
		got,
	)
}

func TestFrontendAPIURL(t *testing.T) {
	assert.Equal(t, "https://apis.developer.overheid.nl/apis/api-1", FrontendAPIURL("api-1"))
}

func TestAbsoluteCurrentRequestURL_UsesForwardedLocation(t *testing.T) {
	req := httptest.NewRequest("GET", "/internal/apis/api-1/feed?format=rss", nil)
	req.Header.Set("Forwarded", `proto=https;host="api.example.test"`)
	req.Header.Set("X-Forwarded-Uri", "/api-register/v1/apis/api-1/feed?format=rss")

	assert.Equal(t,
		"https://api.example.test/api-register/v1/apis/api-1/feed?format=rss",
		AbsoluteCurrentRequestURL(req),
	)
}

func TestAbsoluteCurrentRequestURL_NormalizesPublicAPIHost(t *testing.T) {
	req := httptest.NewRequest("GET", "/v1/apis/MBDp9RTvg/feed", nil)
	req.Host = "api.don.projects.digilab.network"
	req.Header.Set("X-Forwarded-Proto", "http")

	assert.Equal(t,
		"https://api.don.projects.digilab.network/api-register/v1/apis/MBDp9RTvg/feed",
		AbsoluteCurrentRequestURL(req),
	)
}

func TestAbsoluteCurrentRequestURL_NormalizesProductionAPIHost(t *testing.T) {
	req := httptest.NewRequest("GET", "/v1/apis/MBDp9RTvg/feed", nil)
	req.Host = "api.developer.overheid.nl"

	assert.Equal(t,
		"https://api.developer.overheid.nl/api-register/v1/apis/MBDp9RTvg/feed",
		AbsoluteCurrentRequestURL(req),
	)
}

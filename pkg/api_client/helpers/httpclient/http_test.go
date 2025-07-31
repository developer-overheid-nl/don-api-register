package httpclient_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/developer-overheid-nl/don-api-register/pkg/api_client/helpers/httpclient"
	"github.com/stretchr/testify/assert"
)

func TestCorsGet(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "https://example.com", r.Header.Get("Origin"))
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	_, err := httpclient.CorsGet(&http.Client{}, srv.URL, "https://example.com")
	assert.NoError(t, err)
}

func TestFetchOrganisationLabel(t *testing.T) {
	data := []httpclient.TooIGraph{{Graph: []httpclient.TooIObject{{ID: "https://identifier.overheid.nl/tooi/id/org/1", Label: []struct {
		Value    string `json:"@value"`
		Language string `json:"@language"`
	}{{Value: "Label", Language: "nl"}}}}}}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/ld+json")
		_ = json.NewEncoder(w).Encode(data)
	}))
	defer srv.Close()

	lbl, err := httpclient.FetchOrganisationLabel(context.Background(), srv.URL)
	assert.NoError(t, err)
	assert.Equal(t, "Label", lbl)
}

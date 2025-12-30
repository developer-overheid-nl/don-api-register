package httpclient_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"testing"

	"github.com/developer-overheid-nl/don-api-register/pkg/api_client/helpers/httpclient"
	"github.com/developer-overheid-nl/don-api-register/pkg/api_client/testutil"
	"github.com/stretchr/testify/assert"
)

func TestCorsGet(t *testing.T) {
	srv := testutil.NewTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "https://example.com", r.Header.Get("Origin"))
		w.WriteHeader(http.StatusOK)
	}))

	_, err := httpclient.CorsGet(&http.Client{}, srv.URL, "https://example.com")
	assert.NoError(t, err)
}

func TestFetchOrganisationLabel(t *testing.T) {
	// Let op: hoofdletter "Value" en "Language", anders werkt json unmarshalen niet!
	data := []httpclient.TooIGraph{{
		Graph: []httpclient.TooIObject{{
			ID: "https://identifier.overheid.nl/tooi/id/org/1",
			Label: []struct {
				Value    string `json:"@value"`
				Language string `json:"@language"`
			}{{Value: "Label", Language: "nl"}},
		}},
	}}

	srv := testutil.NewTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/ld+json")
		_ = json.NewEncoder(w).Encode(data)
	}))

	// Patch HTTPClient zodat requests naar je testserver gaan
	orig := httpclient.HTTPClient
	defer func() { httpclient.HTTPClient = orig }()

	httpclient.HTTPClient = &http.Client{
		Transport: rewriteHostTransport(srv.URL),
	}

	uri := "https://identifier.overheid.nl/tooi/id/org/1"
	lbl, err := httpclient.FetchOrganisationLabel(context.Background(), uri)
	assert.NoError(t, err)
	assert.Equal(t, "Label", lbl)
}

func rewriteHostTransport(targetBase string) http.RoundTripper {
	return &rewriteTransport{
		base:   http.DefaultTransport,
		target: targetBase,
	}
}

type rewriteTransport struct {
	base   http.RoundTripper
	target string
}

func (t *rewriteTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	u, _ := url.Parse(t.target)
	req.URL.Scheme = u.Scheme
	req.URL.Host = u.Host
	return t.base.RoundTrip(req)
}

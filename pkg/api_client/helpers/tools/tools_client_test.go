package tools

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	httpclient "github.com/developer-overheid-nl/don-api-register/pkg/api_client/helpers/httpclient"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func withToolsServer(t *testing.T, handler http.HandlerFunc) string {
	t.Helper()

	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	t.Setenv("TOOLS_API_ENDPOINT", server.URL+"/api/")
	t.Setenv("X_API_KEY", "secret")

	prevClient := httpclient.HTTPClient
	httpclient.HTTPClient = server.Client()
	t.Cleanup(func() { httpclient.HTTPClient = prevClient })

	return server.URL
}

func TestBuildToolsURL(t *testing.T) {
	t.Setenv("TOOLS_API_ENDPOINT", "https://tools.example.org/base/")

	got, err := buildToolsURL("oas/bundle")

	require.NoError(t, err)
	assert.Equal(t, "https://tools.example.org/base/oas/bundle", got.String())
}

func TestBuildToolsURLRequiresEndpoint(t *testing.T) {
	t.Setenv("TOOLS_API_ENDPOINT", "")

	_, err := buildToolsURL("oas/bundle")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing TOOLS_API_ENDPOINT")
}

func TestDoToolsJSONRequestSendsHeadersAndPayload(t *testing.T) {
	withToolsServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/oas/validate", r.URL.Path)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		assert.Equal(t, "application/json", r.Header.Get("Accept"))
		assert.Equal(t, "secret", r.Header.Get("X-api-key"))

		var body OASInput
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		assert.Equal(t, "https://example.org/openapi.json", body.OasUrl)

		w.Header().Set("X-Test", "ok")
		_, _ = w.Write([]byte(`{"id":"lint-1","successes":true,"score":100}`))
	})

	data, headers, err := doToolsJSONRequest(context.Background(), "oas/validate", OASInput{
		OasUrl: "https://example.org/openapi.json",
	}, "")

	require.NoError(t, err)
	assert.Equal(t, "ok", headers.Get("X-Test"))
	assert.JSONEq(t, `{"id":"lint-1","successes":true,"score":100}`, string(data))
}

func TestDoToolsJSONRequestReturnsResponseBodyOnError(t *testing.T) {
	withToolsServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte("upstream unavailable"))
	})

	_, _, err := doToolsJSONRequest(context.Background(), "oas/validate", OASInput{OasBody: "{}"}, "")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "502 Bad Gateway")
	assert.Contains(t, err.Error(), "upstream unavailable")
}

type repeatedToolByteReader struct{}

func (repeatedToolByteReader) Read(p []byte) (int, error) {
	for index := range p {
		p[index] = 'x'
	}
	return len(p), nil
}

func TestDoToolsJSONRequestRejectsOversizedResponse(t *testing.T) {
	withToolsServer(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.CopyN(w, repeatedToolByteReader{}, maxToolsResponseBytes+1)
	})

	_, _, err := doToolsJSONRequest(context.Background(), "oas/bundle", OASInput{OasBody: "{}"}, "")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "tools response exceeds 20 MiB")
}

func TestDoToolsJSONRequestAcceptsResponseAtSizeLimit(t *testing.T) {
	withToolsServer(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.CopyN(w, repeatedToolByteReader{}, maxToolsResponseBytes)
	})

	data, _, err := doToolsJSONRequest(context.Background(), "oas/bundle", OASInput{OasBody: "{}"}, "")

	require.NoError(t, err)
	assert.Len(t, data, int(maxToolsResponseBytes))
}

func TestBundleOAS(t *testing.T) {
	withToolsServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/oas/bundle", r.URL.Path)
		assert.Equal(t, "application/json", r.Header.Get("Accept"))
		w.Header().Set("Content-Type", "application/yaml")
		_, _ = w.Write([]byte("openapi: 3.1.0"))
	})

	data, contentType, err := BundleOAS(context.Background(), OASInput{OasBody: `{"openapi":"3.1.0"}`})

	require.NoError(t, err)
	assert.Equal(t, "openapi: 3.1.0", string(data))
	assert.Equal(t, "application/yaml", contentType)
}

func TestBundleOASRequiresInput(t *testing.T) {
	_, _, err := BundleOAS(context.Background(), OASInput{})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing OAS input")
}

func TestPostmanPostAndOasConverterPost(t *testing.T) {
	requests := make([]string, 0, 2)
	withToolsServer(t, func(w http.ResponseWriter, r *http.Request) {
		requests = append(requests, r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Content-Disposition", `attachment; filename="artifact.json"`)
		_, _ = io.WriteString(w, `{"ok":true}`)
	})

	data, filename, contentType, err := PostmanPost(context.Background(), OASInput{OasBody: "{}"})
	require.NoError(t, err)
	assert.JSONEq(t, `{"ok":true}`, string(data))
	assert.Equal(t, "artifact.json", filename)
	assert.Equal(t, "application/json", contentType)

	_, filename, _, err = OasConverterPost(context.Background(), OASInput{OasBody: "{}"})
	require.NoError(t, err)
	assert.Equal(t, "artifact.json", filename)
	assert.Equal(t, []string{"/api/oas/postman", "/api/oas/convert"}, requests)
}

func TestPostmanPostUsesDefaultFilename(t *testing.T) {
	withToolsServer(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{}`)
	})

	_, filename, contentType, err := PostmanPost(context.Background(), OASInput{OasBody: "{}"})

	require.NoError(t, err)
	assert.Equal(t, "postman-collection.json", filename)
	assert.Equal(t, "text/plain; charset=utf-8", contentType)
}

func TestArazzoMarkdownAndMermaid(t *testing.T) {
	requests := make([]string, 0, 2)
	withToolsServer(t, func(w http.ResponseWriter, r *http.Request) {
		requests = append(requests, r.URL.Path)
		switch r.URL.Path {
		case "/api/arazzo/markdown":
			w.Header().Set("Content-Type", "text/markdown")
			_, _ = io.WriteString(w, "# Workflow")
		case "/api/arazzo/mermaid":
			w.Header().Set("Content-Type", "text/plain")
			_, _ = io.WriteString(w, "graph TD")
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	})

	markdown, markdownCT, err := ArazzoMarkdown(context.Background(), ArazzoInput{ArazzoBody: "workflows: []"})
	require.NoError(t, err)
	assert.Equal(t, "# Workflow", string(markdown))
	assert.Equal(t, "text/markdown", markdownCT)

	mermaid, mermaidCT, err := ArazzoMermaid(context.Background(), ArazzoInput{ArazzoBody: "workflows: []"})
	require.NoError(t, err)
	assert.Equal(t, "graph TD", string(mermaid))
	assert.Equal(t, "text/plain", mermaidCT)
	assert.Equal(t, []string{"/api/arazzo/markdown", "/api/arazzo/mermaid"}, requests)
}

func TestArazzoRequiresInput(t *testing.T) {
	_, _, err := ArazzoMarkdown(context.Background(), ArazzoInput{})
	require.Error(t, err)

	_, _, err = ArazzoMermaid(context.Background(), ArazzoInput{})
	require.Error(t, err)
}

func TestLintGet(t *testing.T) {
	createdAt := time.Date(2026, 7, 8, 12, 0, 0, 0, time.UTC)
	withToolsServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/oas/validate", r.URL.Path)
		require.NoError(t, json.NewEncoder(w).Encode(LintResultDTO{
			ID:             "lint-1",
			Successes:      true,
			Score:          99,
			CreatedAt:      createdAt,
			RulesetVersion: "2026.07",
			Messages: []LintMessageDTO{{
				ID:        "msg-1",
				Code:      "adr-001",
				Severity:  "warning",
				CreatedAt: createdAt,
				Infos: []LintMessageInfoDTO{{
					ID:      "info-1",
					Message: "detail",
					Path:    "$.info",
				}},
			}},
		}))
	})

	got, err := LintGet(context.Background(), OASInput{OasUrl: "https://example.org/openapi.json"})

	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, "lint-1", got.ID)
	assert.Equal(t, 99, got.Score)
	require.Len(t, got.Messages, 1)
	assert.Equal(t, "adr-001", got.Messages[0].Code)
}

func TestLintGetRejectsInvalidJSON(t *testing.T) {
	withToolsServer(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{`)
	})

	_, err := LintGet(context.Background(), OASInput{OasBody: "{}"})

	require.Error(t, err)
}

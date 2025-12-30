package typesense_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"strings"
	"testing"

	httpclient "github.com/developer-overheid-nl/don-api-register/pkg/api_client/helpers/httpclient"
	"github.com/developer-overheid-nl/don-api-register/pkg/api_client/models"
	"github.com/developer-overheid-nl/don-api-register/pkg/api_client/services/typesense"
	"github.com/developer-overheid-nl/don-api-register/pkg/api_client/testutil"
)

func TestPublishApi_Disabled(t *testing.T) {
	t.Setenv("TYPESENSE_ENDPOINT", "")
	t.Setenv("TYPESENSE_API_KEY", "")
	t.Setenv("TYPESENSE_COLLECTION", "")

	err := typesense.PublishApi(context.Background(), &models.Api{Id: "api-1"})
	if !errors.Is(err, typesense.ErrDisabled) {
		t.Fatalf("expected ErrDisabled, got %v", err)
	}
}

func TestPublishApi_SendsDocument(t *testing.T) {
	var capturedBody []byte
	var capturedPath, capturedAction, capturedKey string

	server := testutil.NewTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedPath = r.URL.Path
		capturedAction = r.URL.Query().Get("action")
		capturedKey = r.Header.Get("X-TYPESENSE-API-KEY")
		defer r.Body.Close()
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("failed to read request body: %v", err)
		}
		capturedBody = body
		w.WriteHeader(http.StatusCreated)
	}))
	defer server.Close()

	t.Setenv("TYPESENSE_ENDPOINT", server.URL)
	t.Setenv("TYPESENSE_API_KEY", "secret")
	t.Setenv("TYPESENSE_COLLECTION", "apis")
	t.Setenv("TYPESENSE_DETAIL_BASE_URL", "https://frontend.test/apis")
	t.Setenv("TYPESENSE_LANGUAGE", "nl")
	t.Setenv("TYPESENSE_ITEM_PRIORITY", "5")
	t.Setenv("TYPESENSE_DEFAULT_TAGS", "api-register,api")

	prevClient := httpclient.HTTPClient
	httpclient.HTTPClient = server.Client()
	t.Cleanup(func() {
		httpclient.HTTPClient = prevClient
	})

	score := 88
	api := &models.Api{
		Id:           "api-1",
		Title:        "Test API",
		Description:  "Een korte omschrijving voor zoekresultaten.",
		DocsUrl:      "https://docs.example.com",
		Auth:         "OAuth2",
		ContactName:  "Jane Doe",
		ContactEmail: "jane@example.com",
		ContactUrl:   "https://example.org/contact",
		Organisation: &models.Organisation{
			Label: "Ministerie van Test",
			Uri:   "https://organisaties.example.com/min-test",
		},
		Version:  "1.0.0",
		AdrScore: &score,
		Servers: []models.Server{
			{Uri: "https://api.example.com", Description: "Productie"},
			{Uri: "https://api.test.example.com"},
		},
	}

	if err := typesense.PublishApi(context.Background(), api); err != nil {
		t.Fatalf("PublishApi returned error: %v", err)
	}

	if capturedPath != "/collections/apis/documents" {
		t.Fatalf("unexpected path %q", capturedPath)
	}
	if capturedAction != "upsert" {
		t.Fatalf("expected action=upsert, got %q", capturedAction)
	}
	if capturedKey != "secret" {
		t.Fatalf("expected api key %q, got %q", "secret", capturedKey)
	}

	var doc map[string]any
	if err := json.Unmarshal(capturedBody, &doc); err != nil {
		t.Fatalf("failed to parse payload: %v", err)
	}

	wantURL := "https://frontend.test/apis/api-1"
	if got := doc["url"]; got != wantURL {
		t.Fatalf("unexpected url: %v", got)
	}
	if got := doc["url_without_anchor"]; got != wantURL {
		t.Fatalf("unexpected url_without_anchor: %v", got)
	}
	if doc["anchor"] != nil {
		t.Fatalf("expected anchor to be nil")
	}
	if got := doc["hierarchy.lvl0"]; got != "Test API" {
		t.Fatalf("unexpected lvl0: %v", got)
	}
	if got := doc["hierarchy.lvl1"]; got != "https://example.org/contact" {
		t.Fatalf("unexpected lvl1: %v", got)
	}
	if got := doc["hierarchy.lvl2"]; got != "Ministerie van Test" {
		log.Println("debug:", got)
		t.Fatalf("unexpected lvl2: %v", got)
	}
	if got := doc["hierarchy.lvl3"]; got != "jane@example.com" {
		t.Fatalf("unexpected lvl3: %v", got)
	}
	if got := doc["hierarchy.lvl4"]; got != "Jane Doe" {
		t.Fatalf("unexpected lvl4: %v", got)
	}
	if got := doc["language"]; got != "nl" {
		t.Fatalf("unexpected language: %v", got)
	}
	if got := doc["item_priority"]; int(got.(float64)) != 5 {
		t.Fatalf("unexpected item_priority: %v", got)
	}

	content, ok := doc["content"].(string)
	if !ok || !strings.Contains(content, "Documentatie: https://docs.example.com") {
		t.Fatalf("content missing documentation: %v", doc["content"])
	}
	if !strings.Contains(content, "Contact: Jane Doe | jane@example.com | https://example.org/contact") {
		t.Fatalf("content missing contact info: %v", content)
	}
	if !strings.Contains(content, "Servers: https://api.example.com (Productie), https://api.test.example.com") {
		t.Fatalf("content missing server info: %v", content)
	}

	rawTags, ok := doc["tags"].([]any)
	if !ok {
		t.Fatalf("tags missing or wrong type: %T", doc["tags"])
	}

	gotTags := make([]string, 0, len(rawTags))
	for _, v := range rawTags {
		gotTags = append(gotTags, v.(string))
	}

	wantTags := []string{
		"api-register",
		"api",
		"api-id:api-1",
		"Ministerie van Test",
		"https://organisaties.example.com/min-test",
		"version:1.0.0",
		"adr:88",
	}
	if len(gotTags) != len(wantTags) {
		t.Fatalf("unexpected tag count: %v", gotTags)
	}
	for i, want := range wantTags {
		if gotTags[i] != want {
			t.Fatalf("unexpected tag at position %d: want %q got %q", i, want, gotTags[i])
		}
	}
}

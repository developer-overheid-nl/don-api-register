package openapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	toolslint "github.com/developer-overheid-nl/don-api-register/pkg/api_client/helpers/tools"
)

func TestFetchParseValidateAndHash_AllowsOpenAPI31(t *testing.T) {
	spec := `{
	  "openapi": "3.1.0",
	  "info": {
	    "title": "Ping",
	    "version": "1.0.0"
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

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(spec))
	}))
	t.Cleanup(server.Close)

	input := toolslint.OASInput{OasUrl: server.URL}
	res, err := FetchParseValidateAndHash(context.Background(), input, FetchOpts{})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if res == nil || res.Spec == nil {
		t.Fatalf("expected parsed spec, got %#v", res)
	}
	if got := res.Spec.Version; got != "3.1.0" {
		t.Fatalf("expected version 3.1.0, got %s", got)
	}
	if res.Hash == "" {
		t.Fatalf("expected hash, got empty string")
	}
}

func TestFetchParseValidateAndHash_RetriesWithoutOriginOnEmptyBody(t *testing.T) {
	spec := `{
	  "openapi": "3.0.1",
	  "info": {
	    "title": "Retry",
	    "version": "1.0.0"
	  },
	  "paths": {}
	}`

	origins := make([]string, 0, 2)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origins = append(origins, r.Header.Get("Origin"))
		if r.Header.Get("Origin") != "" {
			w.WriteHeader(http.StatusOK)
			return
		}
		_, _ = w.Write([]byte(spec))
	}))
	t.Cleanup(server.Close)

	input := toolslint.OASInput{OasUrl: server.URL}
	res, err := FetchParseValidateAndHash(context.Background(), input, FetchOpts{Origin: "https://developer.overheid.nl"})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if res == nil || res.Spec == nil {
		t.Fatalf("expected parsed spec, got %#v", res)
	}
	if got := res.Spec.Info.Title; got != "Retry" {
		t.Fatalf("expected title Retry, got %s", got)
	}
	if len(origins) < 2 {
		t.Fatalf("expected at least two attempts, got %d", len(origins))
	}
	if origins[0] == "" {
		t.Fatalf("expected first request to include Origin header")
	}
	if origins[1] != "" {
		t.Fatalf("expected retry without Origin header, got %q", origins[1])
	}
}

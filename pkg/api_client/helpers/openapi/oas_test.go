package openapi

import (
	"context"
	"net/http"
	"testing"

	toolslint "github.com/developer-overheid-nl/don-api-register/pkg/api_client/helpers/tools"
	"github.com/developer-overheid-nl/don-api-register/pkg/api_client/testutil"
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

	server := testutil.NewTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(spec))
	}))

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
	server := testutil.NewTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origins = append(origins, r.Header.Get("Origin"))
		if r.Header.Get("Origin") != "" {
			w.WriteHeader(http.StatusOK)
			return
		}
		_, _ = w.Write([]byte(spec))
	}))

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

func TestFetchParseValidateAndHash_RetriesRawWhenBundledYamlContainsRecursiveAnchor(t *testing.T) {
	rawSpec := `{
	  "openapi": "3.0.3",
	  "info": {
	    "title": "Recursive Raw",
	    "version": "1.0.0"
	  },
	  "paths": {},
	  "components": {
	    "schemas": {
	      "Node": {
	        "type": "object",
	        "properties": {
	          "children": {
	            "type": "array",
	            "items": {
	              "$ref": "#/components/schemas/Node"
	            }
	          }
	        }
	      }
	    }
	  }
	}`

	bundledYAML := `openapi: 3.0.3
info:
  title: Recursive Bundle
  version: 1.0.0
paths: {}
components:
  schemas:
    Node: &ref_10
      type: object
      properties:
        children:
          type: array
          items: *ref_10
`

	var rawRequests int
	rawServer := testutil.NewTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rawRequests++
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(rawSpec))
	}))

	var bundleRequests int
	toolsServer := testutil.NewTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/oas/bundle" {
			t.Fatalf("expected /oas/bundle path, got %s", r.URL.Path)
		}
		bundleRequests++
		w.Header().Set("Content-Type", "application/yaml")
		_, _ = w.Write([]byte(bundledYAML))
	}))

	t.Setenv("TOOLS_API_ENDPOINT", toolsServer.URL)
	t.Setenv("X_API_KEY", "")

	input := toolslint.OASInput{OasUrl: rawServer.URL}
	res, err := FetchParseValidateAndHash(context.Background(), input, FetchOpts{})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if bundleRequests == 0 {
		t.Fatalf("expected bundled attempt to be used")
	}
	if rawRequests == 0 {
		t.Fatalf("expected raw fetch fallback to be used")
	}
	if res == nil || res.Spec == nil {
		t.Fatalf("expected parsed spec, got %#v", res)
	}
	if got := res.Spec.Info.Title; got != "Recursive Raw" {
		t.Fatalf("expected fallback to raw spec, got title %q", got)
	}
}

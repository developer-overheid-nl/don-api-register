package openapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
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

	res, err := FetchParseValidateAndHash(context.Background(), server.URL, FetchOpts{})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if res == nil || res.Spec == nil {
		t.Fatalf("expected parsed spec, got %#v", res)
	}
	if got := res.Spec.OpenAPI; got != "3.1.0" {
		t.Fatalf("expected version 3.1.0, got %s", got)
	}
	if res.Hash == "" {
		t.Fatalf("expected hash, got empty string")
	}
}

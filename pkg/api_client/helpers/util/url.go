package util

import (
	"fmt"
	"net/http"
	"os"
	"strings"
)

const frontendAPIBaseURL = "https://apis.developer.overheid.nl/apis"

func FrontendAPIURL(id string) string {
	return fmt.Sprintf("%s/%s", frontendAPIBaseURL, id)
}

// ApiFeedURL returns the absolute URL for an API's RSS feed.
// PUBLIC_API_BASE_URL is required so forwarded/proxy headers cannot be spoofed
// by a client.
func ApiFeedURL(id string) (string, error) {
	if base := strings.TrimSpace(os.Getenv("PUBLIC_API_BASE_URL")); base != "" {
		return fmt.Sprintf("%s/apis/%s/feed", strings.TrimRight(base, "/"), id), nil
	}
	return "", fmt.Errorf("PUBLIC_API_BASE_URL is niet ingesteld")
}

func AbsoluteCurrentRequestURL(r *http.Request) string {
	if r == nil || r.URL == nil {
		return ""
	}
	if forwardedURI := strings.TrimSpace(r.Header.Get("X-Forwarded-Uri")); forwardedURI != "" {
		return AbsoluteRequestURL(r, forwardedURI)
	}
	if originalURI := strings.TrimSpace(r.Header.Get("X-Original-URI")); originalURI != "" {
		return AbsoluteRequestURL(r, originalURI)
	}
	path := r.URL.RequestURI()
	if strings.TrimSpace(path) == "" {
		path = r.URL.Path
	}
	return AbsoluteRequestURL(r, path)
}

func AbsoluteRequestURL(r *http.Request, path string) string {
	if r == nil {
		return ""
	}
	scheme := "https"
	host := r.Host

	if forwardedScheme, forwardedHost := parseForwardedHeader(r.Header.Get("Forwarded")); forwardedScheme != "" || forwardedHost != "" {
		if forwardedScheme != "" {
			scheme = forwardedScheme
		}
		if forwardedHost != "" {
			host = forwardedHost
		}
	}
	if forwarded := strings.TrimSpace(r.Header.Get("X-Forwarded-Proto")); forwarded != "" {
		scheme = strings.Split(forwarded, ",")[0]
	}
	if forwarded := strings.TrimSpace(r.Header.Get("X-Forwarded-Host")); forwarded != "" {
		host = strings.Split(forwarded, ",")[0]
	}
	if prefix := strings.TrimRight(strings.TrimSpace(r.Header.Get("X-Forwarded-Prefix")), "/"); prefix != "" && !strings.HasPrefix(path, prefix+"/") {
		path = prefix + path
	}
	scheme, path = normalizePublicAPIURL(scheme, host, path)
	return fmt.Sprintf("%s://%s%s", strings.TrimSpace(scheme), strings.TrimSpace(host), path)
}

func normalizePublicAPIURL(scheme, host, path string) (string, string) {
	hostWithoutPort := strings.Split(strings.TrimSpace(host), ":")[0]
	switch hostWithoutPort {
	case "api.developer.overheid.nl", "api.don.projects.digilab.network":
		scheme = "https"
		if path == "/v1" || strings.HasPrefix(path, "/v1/") {
			path = "/api-register" + path
		}
	}
	return scheme, path
}

func parseForwardedHeader(value string) (string, string) {
	first := strings.TrimSpace(strings.Split(value, ",")[0])
	if first == "" {
		return "", ""
	}

	var proto, host string
	for _, part := range strings.Split(first, ";") {
		key, val, ok := strings.Cut(strings.TrimSpace(part), "=")
		if !ok {
			continue
		}
		val = strings.Trim(strings.TrimSpace(val), `"`)
		switch strings.ToLower(strings.TrimSpace(key)) {
		case "proto":
			proto = val
		case "host":
			host = val
		}
	}
	return proto, host
}

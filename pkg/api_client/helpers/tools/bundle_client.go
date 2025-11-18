package tools

import (
	"context"
	"errors"
	"strings"
)

// BundleOAS requests a bundled OpenAPI document from the tools API.
// It returns the bundled bytes and content type.
func BundleOAS(ctx context.Context, input OASInput) ([]byte, string, error) {
	input.Normalize()
	if input.IsEmpty() {
		return nil, "", errors.New("missing OAS input")
	}
	data, headers, err := doToolsJSONRequest(ctx, "oas/bundle", input, "application/json")
	if err != nil {
		return nil, "", err
	}
	contentType := strings.TrimSpace(headers.Get("Content-Type"))
	if contentType == "" {
		contentType = "application/json"
	}
	return data, contentType, nil
}

package tools

import (
	"context"
	"errors"
	"mime"
	"strings"
)

// BrunoPost calls the tools API /bruno endpoint with the provided OAS input
// and returns the raw bytes, filename (if any), and content type.
func BrunoPost(ctx context.Context, input OASInput) ([]byte, string, string, error) {
	return postBinary(ctx, input, "bruno/convert", "application/octet-stream", "bruno.zip")
}

// PostmanPost calls the tools API /postman endpoint with the provided OAS input
// and returns the raw bytes, filename (if any), and content type.
func PostmanPost(ctx context.Context, input OASInput) ([]byte, string, string, error) {
	return postBinary(ctx, input, "postman/convert", "application/json", "postman-collection.json")
}

func OasConverterPost(ctx context.Context, input OASInput) ([]byte, string, string, error) {
	return postBinary(ctx, input, "oas/convert", "application/json", "converted-oas.json")
}

// postBinary is a helper to call the given tools endpoint with body {oasUrl}
// and returns the raw bytes, filename (if any), and content type.
func postBinary(ctx context.Context, input OASInput, endpoint, wantCT, defaultName string) ([]byte, string, string, error) {
	input.Normalize()
	if input.IsEmpty() {
		return nil, "", "", errors.New("missing OAS input")
	}
	data, headers, err := doToolsJSONRequest(ctx, endpoint, input, wantCT)
	if err != nil {
		return nil, "", "", err
	}
	ct := headers.Get("Content-Type")
	if ct == "" {
		ct = wantCT
	}
	filename := defaultName
	if cd := headers.Get("Content-Disposition"); cd != "" {
		if _, params, err := mime.ParseMediaType(cd); err == nil {
			if fn, ok := params["filename"]; ok && strings.TrimSpace(fn) != "" {
				filename = fn
			}
		}
	}
	return data, filename, ct, nil
}

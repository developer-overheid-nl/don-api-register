package tools

import (
	"context"
	"errors"
	"strings"
)

func ArazzoMarkdown(ctx context.Context, input ArazzoInput) ([]byte, string, error) {
	input.Normalize()
	if input.IsEmpty() {
		return nil, "", errors.New("missing Arazzo input")
	}
	data, headers, err := doToolsJSONRequest(ctx, "arazzo/markdown", input, "text/markdown; charset=utf-8")
	if err != nil {
		return nil, "", err
	}
	contentType := strings.TrimSpace(headers.Get("Content-Type"))
	if contentType == "" {
		contentType = "text/markdown; charset=utf-8"
	}
	return data, contentType, nil
}

func ArazzoMermaid(ctx context.Context, input ArazzoInput) ([]byte, string, error) {
	input.Normalize()
	if input.IsEmpty() {
		return nil, "", errors.New("missing Arazzo input")
	}
	data, headers, err := doToolsJSONRequest(ctx, "arazzo/mermaid", input, "text/plain; charset=utf-8")
	if err != nil {
		return nil, "", err
	}
	contentType := strings.TrimSpace(headers.Get("Content-Type"))
	if contentType == "" {
		contentType = "text/plain; charset=utf-8"
	}
	return data, contentType, nil
}

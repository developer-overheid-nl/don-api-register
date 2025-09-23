package tools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path"
	"strings"

	httpclient "github.com/developer-overheid-nl/don-api-register/pkg/api_client/helpers/httpclient"
)

type oasBody struct {
	OasUrl string `json:"oasUrl"`
}

// BrunoPost calls the tools API /bruno endpoint with body {oasUrl}
// and returns the raw bytes, filename (if any), and content type.
func BrunoPost(ctx context.Context, oasURL string) ([]byte, string, string, error) {
	return postBinary(ctx, oasURL, "bruno/convert", "application/octet-stream", "bruno.zip")
}

// PostmanPost calls the tools API /postman endpoint with body {oasUrl}
// and returns the raw bytes, filename (if any), and content type.
func PostmanPost(ctx context.Context, oasURL string) ([]byte, string, string, error) {
	return postBinary(ctx, oasURL, "postman/convert", "application/json", "postman-collection.json")
}

func postBinary(ctx context.Context, oasUrl, endpoint, wantCT, defaultName string) ([]byte, string, string, error) {
	base := strings.TrimSpace(os.Getenv("TOOLS_API_ENDPOINT"))
	if base == "" {
		log.Printf("[tools:%s] TOOLS_API_ENDPOINT is leeg", endpoint)
		return nil, "", "", errors.New("missing TOOLS_API_ENDPOINT env var")
	}
	pu, err := url.Parse(base)
	if err != nil {
		return nil, "", "", fmt.Errorf("invalid TOOLS_API_ENDPOINT: %w", err)
	}

	pu.Path = path.Join(pu.Path, endpoint)

	body := oasBody{OasUrl: oasUrl}
	buf, err := json.Marshal(body)
	if err != nil {
		return nil, "", "", err
	}
	token, _ := fetchToken(ctx)
	log.Println(pu.String())
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, pu.String(), strings.NewReader(string(buf)))
	if err != nil {
		return nil, "", "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", wantCT)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := httpclient.HTTPClient.Do(req)
	if err != nil {
		return nil, "", "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		return nil, "", "", fmt.Errorf("tools %s request failed: %s body=%s", endpoint, resp.Status, string(b))
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", "", err
	}

	ct := resp.Header.Get("Content-Type")
	if ct == "" {
		ct = wantCT
	}
	filename := defaultName
	if cd := resp.Header.Get("Content-Disposition"); cd != "" {
		if _, params, err := mime.ParseMediaType(cd); err == nil {
			if fn, ok := params["filename"]; ok && strings.TrimSpace(fn) != "" {
				filename = fn
			}
		}
	}
	return data, filename, ct, nil
}

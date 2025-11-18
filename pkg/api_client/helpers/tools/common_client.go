package tools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"strings"

	httpclient "github.com/developer-overheid-nl/don-api-register/pkg/api_client/helpers/httpclient"
)

func buildToolsURL(endpoint string) (*url.URL, error) {
	base := strings.TrimSpace(os.Getenv("TOOLS_API_ENDPOINT"))
	if base == "" {
		return nil, errors.New("missing TOOLS_API_ENDPOINT env var")
	}
	pu, err := url.Parse(base)
	if err != nil {
		return nil, fmt.Errorf("invalid TOOLS_API_ENDPOINT: %w", err)
	}
	pu.Path = path.Join(pu.Path, endpoint)
	return pu, nil
}

func doToolsJSONRequest(ctx context.Context, endpoint string, payload any, accept string) ([]byte, http.Header, error) {
	pu, err := buildToolsURL(endpoint)
	if err != nil {
		return nil, nil, err
	}
	buf, err := json.Marshal(payload)
	if err != nil {
		return nil, nil, err
	}
	reader := strings.NewReader(string(buf))
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, pu.String(), reader)
	if err != nil {
		return nil, nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if accept == "" {
		accept = "application/json"
	}
	req.Header.Set("Accept", accept)
	if token := strings.TrimSpace(os.Getenv("X_API_KEY")); token != "" {
		req.Header.Set("X-api-key", token)
	}

	resp, err := httpclient.HTTPClient.Do(req)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, nil, fmt.Errorf("tools %s request failed: %s body=%s", endpoint, resp.Status, string(data))
	}
	return data, resp.Header.Clone(), nil
}

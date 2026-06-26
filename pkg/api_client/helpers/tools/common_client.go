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

type toolsProblem struct {
	Title  string `json:"title"`
	Detail string `json:"detail"`
	Status int    `json:"status"`
}

const maxToolsResponseBytes int64 = 32 << 20

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

func toolsErrorMessage(status string, data []byte) string {
	var problem toolsProblem
	if err := json.Unmarshal(data, &problem); err == nil {
		if detail := strings.TrimSpace(problem.Detail); detail != "" {
			return detail
		}
		if title := strings.TrimSpace(problem.Title); title != "" {
			return title
		}
	}

	body := strings.TrimSpace(string(data))
	if body == "" {
		return status
	}
	return fmt.Sprintf("%s body=%s", status, body)
}

func doToolsJSONRequest(ctx context.Context, endpoint string, payload any, accept string) (data []byte, headers http.Header, err error) {
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
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil && err == nil {
			err = fmt.Errorf("close tools response body: %w", closeErr)
		}
	}()

	data, err = readResponseBody(resp.Body, maxToolsResponseBytes)
	if err != nil {
		return nil, nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, nil, errors.New(toolsErrorMessage(resp.Status, data))
	}
	return data, resp.Header.Clone(), nil
}

func readResponseBody(body io.Reader, limit int64) ([]byte, error) {
	if limit <= 0 {
		return io.ReadAll(body)
	}
	limited := io.LimitReader(body, limit+1)
	data, err := io.ReadAll(limited)
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, fmt.Errorf("tools response too large: exceeds %d bytes", limit)
	}
	return data, nil
}

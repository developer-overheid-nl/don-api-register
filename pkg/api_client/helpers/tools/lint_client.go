package tools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path"
	"strings"
	"time"

	httpclient "github.com/developer-overheid-nl/don-api-register/pkg/api_client/helpers/httpclient"
)

// DTOs that match the tools lint response
type LintMessageInfoDTO struct {
	ID            string `json:"id"`
	LintMessageID string `json:"lintMessageId,omitempty"`
	Message       string `json:"message"`
	Path          string `json:"path,omitempty"`
}

type LintMessageDTO struct {
	ID        string               `json:"id"`
	Code      string               `json:"code"`
	Severity  string               `json:"severity"`
	CreatedAt time.Time            `json:"createdAt"`
	Infos     []LintMessageInfoDTO `json:"infos,omitempty"`
}

type LintResultDTO struct {
	ID        string           `json:"id"`
	ApiID     string           `json:"apiId,omitempty"`
	Successes bool             `json:"successes"`
	Failures  int              `json:"failures"`
	Warnings  int              `json:"warnings"`
	Score     int              `json:"score"`
	Messages  []LintMessageDTO `json:"messages"`
	CreatedAt time.Time        `json:"createdAt"`
}

// LintGet calls the tools API to lint the given OAS URL and returns the result DTO.
func LintGet(ctx context.Context, oasURL string) (*LintResultDTO, error) {
	base := strings.TrimSpace(os.Getenv("TOOLS_API_ENDPOINT"))
	if base == "" {
		return nil, errors.New("missing TOOLS_API_ENDPOINT env var")
	}
	pu, err := url.Parse(base)
	if err != nil {
		return nil, fmt.Errorf("invalid TOOLS_API_ENDPOINT: %w", err)
	}
	dir := path.Dir(pu.Path)
	pu.Path = path.Join(dir, "lint")

	q := pu.Query()
	q.Set("oasUrl", oasURL)
	pu.RawQuery = q.Encode()

	// Optional bearer token via client credentials, if configured
	token, _ := fetchToken(ctx)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, pu.String(), nil)
	if err != nil {
		return nil, err
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := httpclient.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("tools lint request failed: %s", resp.Status)
	}

	var out LintResultDTO
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("decode tools lint response: %w", err)
	}
	return &out, nil
}

// fetchToken tries to obtain a client credentials token using AUTH_* env vars.
func fetchToken(ctx context.Context) (string, error) {
	tokenURL := strings.TrimSpace(os.Getenv("AUTH_TOKEN_URL"))
	clientID := strings.TrimSpace(os.Getenv("AUTH_CLIENT_ID"))
	clientSecret := strings.TrimSpace(os.Getenv("AUTH_CLIENT_SECRET"))
	if tokenURL == "" || clientID == "" || clientSecret == "" {
		return "", errors.New("missing auth configuration")
	}

	form := url.Values{}
	form.Set("grant_type", "client_credentials")
	form.Set("client_id", clientID)
	form.Set("client_secret", clientSecret)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := httpclient.HTTPClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("token request failed: %s", resp.Status)
	}
	var tok struct {
		AccessToken string `json:"access_token"`
		TokenType   string `json:"token_type"`
		ExpiresIn   int    `json:"expires_in"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tok); err != nil {
		return "", err
	}
	if tok.AccessToken == "" {
		return "", errors.New("empty access_token in response")
	}
	return tok.AccessToken, nil
}

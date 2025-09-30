package tools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
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
func LintGet(ctx context.Context, oasUrl string) (*LintResultDTO, error) {
	base := strings.TrimSpace(os.Getenv("TOOLS_API_ENDPOINT"))
	if base == "" {
		log.Printf("[LintGet] TOOLS_API_ENDPOINT is leeg")
		return nil, errors.New("missing TOOLS_API_ENDPOINT env var")
	}
	log.Printf("[LintGet] TOOLS_API_ENDPOINT=%s", base)

	pu, err := url.Parse(base)
	if err != nil {
		log.Printf("[LintGet] Fout bij parsen base URL: %v", err)
		return nil, fmt.Errorf("invalid TOOLS_API_ENDPOINT: %w", err)
	}

	pu.Path = path.Join(pu.Path, "oas/validate")

	body := oasBody{OasUrl: oasUrl}
	buf, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	log.Printf("[LintGet] Opgebouwde lint-URL: %s", pu.String())

	// Optional bearer token via client credentials, if configured
	token := strings.TrimSpace(os.Getenv("X_API_KEY"))

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, pu.String(), strings.NewReader(string(buf)))
	if err != nil {
		log.Printf("[LintGet] Fout bij aanmaken request: %v", err)
		return nil, err
	}

	if token != "" {
		req.Header.Set("X-api-key", token)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := httpclient.HTTPClient.Do(req)
	if err != nil {
		log.Printf("[LintGet] HTTP-fout: %v", err)
		return nil, err
	}
	defer resp.Body.Close()
	log.Printf("[LintGet] Response status: %s", resp.Status)

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		// Body uitlezen voor debug
		body, _ := io.ReadAll(resp.Body)
		log.Printf("[LintGet] Non-2xx status, body: %s", string(body))
		return nil, fmt.Errorf("tools lint request failed: %s", resp.Status)
	}

	var out LintResultDTO
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		log.Printf("[LintGet] Fout bij decoderen response: %v", err)
		return nil, fmt.Errorf("decode tools lint response: %w", err)
	}

	log.Printf("[LintGet] Lint-resultaat succesvol ontvangen: %+v", out)
	return &out, nil
}

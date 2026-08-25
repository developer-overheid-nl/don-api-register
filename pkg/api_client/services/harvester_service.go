package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	problem "github.com/developer-overheid-nl/don-api-register/pkg/api_client/helpers/problem"
	"github.com/developer-overheid-nl/don-api-register/pkg/api_client/models"
	"golang.org/x/time/rate"
)

const (
	// Defaults for PDOK-like sources
	defaultUISuffix = "ui/"
	defaultOASPath  = "openapi.json"
)

// HarvesterService haalt index.json op, leidt OAS-URLs af en slaat ze op
type HarvesterService struct {
	httpClient *http.Client
	apiService *APIsAPIService
}

type HarvestError struct {
	Result models.HarvestResult
	OASURL string
	Cause  error
	Detail string
}

func (e *HarvestError) Error() string {
	if e == nil {
		return "harvest failed"
	}
	return fmt.Sprintf("%d failures; first: %s: %s", e.Result.FailedCount, e.OASURL, e.Detail)
}

func (e *HarvestError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Cause
}

// NewHarvesterService maakt een nieuwe service met een verplichte api service
func NewHarvesterService(apiService *APIsAPIService) *HarvesterService {
	return &HarvesterService{
		httpClient: &http.Client{Timeout: 30 * time.Second},
		apiService: apiService,
	}
}

// RunOnce voert een harvest uit voor één bron
func (s *HarvesterService) RunOnce(ctx context.Context, src models.HarvestSource) (result models.HarvestResult, err error) {
	startedAt := time.Now()
	defer func() {
		attrs := []any{
			"component", "harvest",
			"operation", "run",
			"source", src.Name,
			"candidate_count", result.CandidateCount,
			"created_count", result.CreatedCount,
			"skipped_count", result.SkippedCount,
			"failed_count", result.FailedCount,
			"duration_ms", time.Since(startedAt).Milliseconds(),
		}
		if err == nil {
			slog.InfoContext(ctx, "harvest completed", attrs...)
			return
		}
		var harvestErr *HarvestError
		if errors.As(err, &harvestErr) {
			attrs = append(attrs,
				"first_failure_oas_url", harvestErr.OASURL,
				"first_failure_error", harvestErr.Detail,
			)
		}
		attrs = append(attrs, "error", err)
		slog.ErrorContext(ctx, "harvest failed", attrs...)
	}()

	if s.apiService == nil {
		return result, errors.New("api service is not configured")
	}
	if strings.TrimSpace(src.IndexURL) == "" {
		return result, errors.New("source indexUrl is empty")
	}

	// Fetch index
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, src.IndexURL, nil)
	if err != nil {
		return result, err
	}
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return result, err
	}

	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil && err == nil {
			err = fmt.Errorf("close harvester response body: %w", closeErr)
		}
	}()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<10))
		return result, fmt.Errorf("unexpected status %d from index: %s", resp.StatusCode, string(b))
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return result, err
	}

	hrefs, err := extractIndexHrefs(body)
	if err != nil {
		return result, err
	}
	result.CandidateCount = len(hrefs)
	slog.InfoContext(
		ctx,
		"harvest started",
		"component", "harvest",
		"operation", "run",
		"source", src.Name,
		"candidate_count", result.CandidateCount,
	)
	slog.DebugContext(
		ctx,
		"harvest index loaded",
		"component", "harvest",
		"operation", "load_index",
		"source", src.Name,
		"candidate_count", len(hrefs),
	)
	if len(hrefs) == 0 {
		return result, nil
	}

	uiSuffix := src.UISuffix
	if strings.TrimSpace(uiSuffix) == "" {
		uiSuffix = defaultUISuffix
	}
	oasPath := src.OASPath
	if strings.TrimSpace(oasPath) == "" {
		oasPath = defaultOASPath
	}

	var firstFailure *HarvestError
	// (2 requests per seconde, burst van 1)
	limiter := rate.NewLimiter(rate.Limit(2), 1)

	for _, href := range hrefs {
		oasURL := deriveOASURLWith(href, uiSuffix, oasPath)
		exists, lookupErr := s.apiService.HasAPIWithOASURL(ctx, oasURL)
		if lookupErr != nil {
			result.FailedCount++
			if firstFailure == nil {
				firstFailure = newHarvestError(result, oasURL, lookupErr)
			}
			continue
		}
		if exists {
			result.SkippedCount++
			continue
		}

		payload := models.ApiPost{
			OasUrl:          oasURL,
			OrganisationUri: src.OrganisationUri,
			Contact:         src.Contact,
		}

		if err := limiter.Wait(ctx); err != nil {
			result.FailedCount++
			return result, newHarvestError(result, oasURL, fmt.Errorf("limiter error: %w", err))
		}

		if _, err := s.apiService.CreateApiFromOas(payload); err != nil {
			result.FailedCount++
			if firstFailure == nil {
				firstFailure = newHarvestError(result, oasURL, fmt.Errorf("create api from oas failed: %w", err))
			}
			continue
		}
		result.CreatedCount++
	}

	if result.FailedCount > 0 {
		firstFailure.Result = result
		return result, firstFailure
	}
	return result, nil
}

func newHarvestError(result models.HarvestResult, oasURL string, cause error) *HarvestError {
	return &HarvestError{
		Result: result,
		OASURL: oasURL,
		Cause:  cause,
		Detail: harvestErrorDetail(cause),
	}
}

func harvestErrorDetail(err error) string {
	var apiErr problem.APIError
	if errors.As(err, &apiErr) && len(apiErr.Errors) > 0 {
		return apiErr.Errors[0].Detail
	}
	if err == nil {
		return "unknown harvest failure"
	}
	return err.Error()
}

// deriveOASURLWith bepaalt de OAS-URL op basis van href, uiSuffix en oasPath
func deriveOASURLWith(href, uiSuffix, oasPath string) string {
	h := strings.TrimSpace(href)
	sfx := strings.TrimSpace(uiSuffix)
	if sfx == "" {
		sfx = defaultUISuffix
	}
	op := strings.TrimSpace(oasPath)
	if op == "" {
		op = defaultOASPath
	}
	// normaliseer suffix: zonder leading slash en met trailing slash
	if !strings.HasSuffix(sfx, "/") {
		sfx = sfx + "/"
	}
	if strings.HasSuffix(h, sfx) {
		return strings.TrimSuffix(h, sfx) + op
	}
	if strings.HasSuffix(h, "/"+strings.TrimSuffix(sfx, "/")) { // ook varianten zonder slash
		return strings.TrimSuffix(h, "/"+strings.TrimSuffix(sfx, "/")) + "/" + op
	}
	if strings.HasSuffix(h, "/") {
		return h + op
	}
	return h + "/" + op
}

// extractIndexHrefs parseert verschillende mogelijke vormen van index.json en retourneert hrefs
func extractIndexHrefs(data []byte) ([]string, error) {
	var r models.HarvestIndexRoot
	if err := json.Unmarshal(data, &r); err != nil {
		return nil, fmt.Errorf("parse index.json: %w", err)
	}

	var out []string
	for _, e := range r.Apis {
		// 1) links als array van objecten
		var arr []models.HarvestIndexLink
		if err := json.Unmarshal(e.Links, &arr); err == nil {
			for _, l := range arr {
				if strings.TrimSpace(l.Href) != "" {
					out = append(out, l.Href)
				}
			}
			continue
		}
		// 2) links als enkel object
		var obj models.HarvestIndexLink
		if err := json.Unmarshal(e.Links, &obj); err == nil {
			if strings.TrimSpace(obj.Href) != "" {
				out = append(out, obj.Href)
			}
		}
	}
	return out, nil
}

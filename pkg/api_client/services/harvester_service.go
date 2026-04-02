package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

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

// NewHarvesterService maakt een nieuwe service met een verplichte api service
func NewHarvesterService(apiService *APIsAPIService) *HarvesterService {
	return &HarvesterService{
		httpClient: &http.Client{Timeout: 30 * time.Second},
		apiService: apiService,
	}
}

// RunOnce voert een harvest uit voor één bron
func (s *HarvesterService) RunOnce(ctx context.Context, src models.HarvestSource) error {
	if s.apiService == nil {
		return errors.New("api service is not configured")
	}
	if strings.TrimSpace(src.IndexURL) == "" {
		return errors.New("source indexUrl is empty")
	}

	// Fetch index
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, src.IndexURL, nil)
	if err != nil {
		return err
	}
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return err
	}

	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil && err == nil {
			err = fmt.Errorf("close harvester response body: %w", closeErr)
		}
	}()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<10))
		return fmt.Errorf("unexpected status %d from index: %s", resp.StatusCode, string(b))
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	hrefs, err := extractIndexHrefs(body)
	if err != nil {
		return err
	}
	log.Printf("[harvest %s] gevonden in index: %d", src.Name, len(hrefs))
	if len(hrefs) == 0 {
		return nil
	}

	uiSuffix := src.UISuffix
	if strings.TrimSpace(uiSuffix) == "" {
		uiSuffix = defaultUISuffix
	}
	oasPath := src.OASPath
	if strings.TrimSpace(oasPath) == "" {
		oasPath = defaultOASPath
	}

	var aggErrs []string
	successCount := 0
	// (2 requests per seconde, burst van 1)
	limiter := rate.NewLimiter(rate.Limit(2), 1)

	for _, href := range hrefs {
		oasURL := deriveOASURLWith(href, uiSuffix, oasPath)
		payload := models.ApiPost{
			OasUrl:          oasURL,
			OrganisationUri: src.OrganisationUri,
			Contact:         src.Contact,
		}

		if err := limiter.Wait(ctx); err != nil {
			return fmt.Errorf("limiter error: %w", err)
		}

		if _, err := s.apiService.CreateApiFromOas(payload); err != nil {
			aggErrs = append(aggErrs, fmt.Sprintf("%s: create api from oas failed: %v", oasURL, err))
			continue
		}
		successCount++
	}

	log.Printf("[harvest %s] afgerond: candidates=%d success=%d failures=%d", src.Name, len(hrefs), successCount, len(aggErrs))

	if len(aggErrs) > 0 {
		return fmt.Errorf("%d failures; first: %s", len(aggErrs), aggErrs[0])
	}
	return nil
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
	type linkObj struct {
		Href string `json:"href"`
	}
	type apiEntryFlexible struct {
		Links json.RawMessage `json:"links"`
	}
	type root struct {
		Apis []apiEntryFlexible `json:"apis"`
	}

	var r root
	if err := json.Unmarshal(data, &r); err != nil {
		return nil, fmt.Errorf("parse index.json: %w", err)
	}

	var out []string
	for _, e := range r.Apis {
		// 1) links als array van objecten
		var arr []linkObj
		if err := json.Unmarshal(e.Links, &arr); err == nil {
			for _, l := range arr {
				if strings.TrimSpace(l.Href) != "" {
					out = append(out, l.Href)
				}
			}
			continue
		}
		// 2) links als enkel object
		var obj linkObj
		if err := json.Unmarshal(e.Links, &obj); err == nil {
			if strings.TrimSpace(obj.Href) != "" {
				out = append(out, obj.Href)
			}
		}
	}
	return out, nil
}

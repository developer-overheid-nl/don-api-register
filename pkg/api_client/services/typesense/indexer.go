package typesense

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"

	httpclient "github.com/developer-overheid-nl/don-api-register/pkg/api_client/helpers/httpclient"
	"github.com/developer-overheid-nl/don-api-register/pkg/api_client/models"
)

const (
	defaultDetailBaseURL = "https://api-register.don.apps.digilab.network/apis"
	defaultLanguage      = "nl"
	defaultItemPriority  = 1
)

// ErrDisabled is returned when Typesense configuration is missing.
var ErrDisabled = errors.New("typesense indexing disabled: missing endpoint, api key or collection name")

type config struct {
	endpoint       string
	apiKey         string
	collection     string
	detailBaseURL  string
	language       string
	itemPriority   int
	defaultTags    []string
	featureEnabled bool
}

func loadConfigFromEnv() config {
	endpoint := strings.TrimSpace(os.Getenv("TYPESENSE_ENDPOINT"))
	if endpoint == "" {
		endpoint = strings.TrimSpace(os.Getenv("TYPESENSE_BASE_URL"))
	}

	apiKey := strings.TrimSpace(os.Getenv("TYPESENSE_API_KEY"))
	collection := strings.TrimSpace(os.Getenv("TYPESENSE_COLLECTION"))
	if collection == "" {
		collection = "api_register"
	}

	detailBase := strings.TrimSpace(os.Getenv("TYPESENSE_DETAIL_BASE_URL"))
	if detailBase == "" {
		detailBase = defaultDetailBaseURL
	}

	language := strings.TrimSpace(os.Getenv("TYPESENSE_LANGUAGE"))
	if language == "" {
		language = defaultLanguage
	}

	itemPriority := defaultItemPriority
	if raw := strings.TrimSpace(os.Getenv("TYPESENSE_ITEM_PRIORITY")); raw != "" {
		if v, err := strconv.Atoi(raw); err == nil {
			itemPriority = v
		}
	}

	tags := parseDefaultTags()

	return config{
		endpoint:       endpoint,
		apiKey:         apiKey,
		collection:     collection,
		detailBaseURL:  detailBase,
		language:       language,
		itemPriority:   itemPriority,
		defaultTags:    tags,
		featureEnabled: isFeatureEnabled(),
	}
}

func (c config) enabled() bool {
	return c.featureEnabled && c.endpoint != "" && c.apiKey != "" && c.collection != ""
}

func isFeatureEnabled() bool {
	raw := strings.TrimSpace(os.Getenv("ENABLE_TYPESENSE"))
	if raw == "" {
		return true
	}
	switch strings.ToLower(raw) {
	case "0", "false", "no", "off":
		return false
	default:
		return true
	}
}

// Enabled reports whether Typesense indexing is active based on env vars.
func Enabled() bool {
	return loadConfigFromEnv().enabled()
}

func parseDefaultTags() []string {
	raw := os.Getenv("TYPESENSE_DEFAULT_TAGS")
	if strings.TrimSpace(raw) == "" {
		return []string{"api-register", "api"}
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	if len(out) == 0 {
		return []string{"api-register", "api"}
	}
	return out
}

// PublishApi pushes the provided API to Typesense for full-text search.
func PublishApi(ctx context.Context, api *models.Api) error {
	if api == nil {
		return fmt.Errorf("typesense: api is nil")
	}

	cfg := loadConfigFromEnv()
	if !cfg.enabled() {
		return ErrDisabled
	}

	payload, err := json.Marshal(buildDocument(cfg, api))
	if err != nil {
		return fmt.Errorf("typesense: marshal payload: %w", err)
	}

	base := strings.TrimRight(cfg.endpoint, "/")
	target := fmt.Sprintf("%s/collections/%s/documents?action=upsert", base, url.PathEscape(cfg.collection))

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, target, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("typesense: create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-TYPESENSE-API-KEY", cfg.apiKey)

	resp, err := httpclient.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("typesense: request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= http.StatusMultipleChoices {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("typesense: indexing failed with status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	return nil
}

func buildDocument(cfg config, api *models.Api) map[string]any {
	doc := map[string]any{
		"type":          "doc",
		"language":      cfg.language,
		"item_priority": cfg.itemPriority,
	}

	if id := strings.TrimSpace(api.Id); id != "" {
		doc["id"] = id
	}

	detailBase := strings.TrimRight(cfg.detailBaseURL, "/")
	if detailBase != "" && api.Id != "" {
		detailURL := fmt.Sprintf("%s/%s", detailBase, api.Id)
		doc["url"] = detailURL
		doc["url_without_anchor"] = detailURL
		doc["anchor"] = nil
	}

	if title := strings.TrimSpace(api.Title); title != "" {
		doc["hierarchy.lvl0"] = title
	}
	if name := strings.TrimSpace(api.ContactName); name != "" {
		doc["hierarchy.lvl4"] = name
	}
	if org := api.Organisation; org != nil {
		if label := strings.TrimSpace(org.Label); label != "" {
			doc["hierarchy.lvl2"] = label
		}
	} else if api.OrganisationID != nil {
		if id := strings.TrimSpace(*api.OrganisationID); id != "" {
			doc["hierarchy.lvl2"] = id
		}
	}
	if email := strings.TrimSpace(api.ContactEmail); email != "" {
		doc["hierarchy.lvl3"] = email
	}
	if contactURL := strings.TrimSpace(api.ContactUrl); contactURL != "" {
		doc["hierarchy.lvl1"] = contactURL
	}

	if content := buildContent(api); content != "" {
		doc["content"] = content
	}

	if tags := buildTags(cfg, api); len(tags) > 0 {
		doc["tags"] = tags
	}

	return doc
}

func buildContent(api *models.Api) string {
	parts := make([]string, 0)
	if desc := strings.TrimSpace(api.Description); desc != "" {
		parts = append(parts, desc)
	}
	if docs := strings.TrimSpace(api.DocsUrl); docs != "" {
		parts = append(parts, fmt.Sprintf("Documentatie: %s", docs))
	}
	if auth := strings.TrimSpace(api.Auth); auth != "" {
		parts = append(parts, fmt.Sprintf("Authenticatie: %s", auth))
	}
	if org := api.Organisation; org != nil {
		if label := strings.TrimSpace(org.Label); label != "" {
			parts = append(parts, fmt.Sprintf("Organisatie: %s", label))
		}
	}
	if name := strings.TrimSpace(api.ContactName); name != "" {
		contactBits := []string{name}
		if email := strings.TrimSpace(api.ContactEmail); email != "" {
			contactBits = append(contactBits, email)
		}
		if link := strings.TrimSpace(api.ContactUrl); link != "" {
			contactBits = append(contactBits, link)
		}
		parts = append(parts, fmt.Sprintf("Contact: %s", strings.Join(contactBits, " | ")))
	}
	if len(api.Servers) > 0 {
		serverParts := make([]string, 0, len(api.Servers))
		for _, srv := range api.Servers {
			if srv.Uri == "" {
				continue
			}
			if desc := strings.TrimSpace(srv.Description); desc != "" {
				serverParts = append(serverParts, fmt.Sprintf("%s (%s)", srv.Uri, desc))
			} else {
				serverParts = append(serverParts, srv.Uri)
			}
		}
		if len(serverParts) > 0 {
			parts = append(parts, fmt.Sprintf("Servers: %s", strings.Join(serverParts, ", ")))
		}
	}

	if len(parts) == 0 {
		return strings.TrimSpace(api.Title)
	}
	return strings.Join(parts, "\n\n")
}

func buildTags(cfg config, api *models.Api) []string {
	seen := make(map[string]struct{})
	out := make([]string, 0, len(cfg.defaultTags)+5)

	for _, tag := range cfg.defaultTags {
		out = appendUnique(out, tag, seen)
	}

	out = appendUnique(out, fmt.Sprintf("api-id:%s", api.Id), seen)

	if org := api.Organisation; org != nil {
		out = appendUnique(out, org.Label, seen)
		out = appendUnique(out, org.Uri, seen)
	}

	if api.Version != "" {
		out = appendUnique(out, fmt.Sprintf("version:%s", api.Version), seen)
	}
	if api.AdrScore != nil {
		out = appendUnique(out, fmt.Sprintf("adr:%d", *api.AdrScore), seen)
	}

	return out
}

func appendUnique(tags []string, value string, seen map[string]struct{}) []string {
	value = strings.TrimSpace(value)
	if value == "" {
		return tags
	}
	if _, ok := seen[value]; ok {
		return tags
	}
	seen[value] = struct{}{}
	return append(tags, value)
}

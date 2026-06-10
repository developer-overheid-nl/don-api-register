package typesense

import (
	"context"
	"fmt"
	"strings"

	httpclient "github.com/developer-overheid-nl/don-api-register/pkg/api_client/helpers/httpclient"
	"github.com/developer-overheid-nl/don-api-register/pkg/api_client/models"
	commontypesense "github.com/developer-overheid-nl/don-register-common/typesense"
)

const (
	defaultDetailBaseURL = "https://api-register.don.apps.digilab.network/apis"
	defaultLanguage      = "nl"
	defaultItemPriority  = 1
)

// ErrDisabled is returned when Typesense configuration is missing.
var ErrDisabled = commontypesense.ErrDisabled

type config = commontypesense.Config

func loadConfigFromEnv() config {
	return commontypesense.LoadConfigFromEnv(commontypesense.Defaults{
		Collection:    "api_register",
		DetailBaseURL: defaultDetailBaseURL,
		Language:      defaultLanguage,
		ItemPriority:  defaultItemPriority,
		DefaultTags:   []string{"api-register", "api"},
	})
}

// Enabled reports whether Typesense indexing is active based on env vars.
func Enabled() bool {
	return loadConfigFromEnv().Enabled()
}

// PublishApi pushes the provided API to Typesense for full-text search.
func PublishApi(ctx context.Context, api *models.Api) (err error) {
	if api == nil {
		return fmt.Errorf("typesense: api is nil")
	}

	cfg := loadConfigFromEnv()
	if !cfg.Enabled() {
		return ErrDisabled
	}

	return commontypesense.UpsertDocument(ctx, httpclient.HTTPClient, cfg, buildDocument(cfg, api))
}

func buildDocument(cfg config, api *models.Api) map[string]any {
	doc := commontypesense.BaseDocument(cfg, api.Id)

	if title := strings.TrimSpace(api.Title); title != "" {
		doc["hierarchy.lvl0"] = title
	}
	doc["hierarchy.lvl1"] = "API"
	if org := api.Organisation; org != nil {
		if label := strings.TrimSpace(org.Label); label != "" {
			doc["hierarchy.lvl2"] = label
		}
	} else if api.OrganisationID != nil {
		if id := strings.TrimSpace(*api.OrganisationID); id != "" {
			doc["hierarchy.lvl2"] = id
		}
	}
	if name := strings.TrimSpace(api.ContactName); name != "" {
		doc["hierarchy.lvl3"] = name
	}
	if version := strings.TrimSpace(api.Version); version != "" {
		doc["hierarchy.lvl4"] = version
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
	out := make([]string, 0, len(cfg.DefaultTags)+5)

	for _, tag := range cfg.DefaultTags {
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
	return commontypesense.AppendUnique(tags, value, seen)
}

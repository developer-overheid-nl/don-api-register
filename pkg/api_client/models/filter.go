package models

import "fmt"

type FilterOption struct {
	Value       string  `json:"value"`
	Label       string  `json:"label"`
	Description *string `json:"description"`
	Count       int     `json:"count"`
	Selected    bool    `json:"selected"`
}

type FilterGroup struct {
	Key         string         `json:"key"`
	Label       string         `json:"label"`
	Description string         `json:"description"`
	Type        string         `json:"type"`
	Value       any            `json:"value,omitempty"`
	Count       *int           `json:"count,omitempty"`
	Options     []FilterOption `json:"options,omitempty"`
}

func (f FilterGroup) Validate() error {
	switch f.Type {
	case "toggle":
		if _, ok := f.Value.(bool); !ok {
			return fmt.Errorf("filter %q: toggle value must be bool, got %T", f.Key, f.Value)
		}
	case "date":
		if f.Value != nil {
			if _, ok := f.Value.(string); !ok {
				return fmt.Errorf("filter %q: date value must be string, got %T", f.Key, f.Value)
			}
		}
	}
	return nil
}

type FilterCount struct {
	Value string
	Count int
}

type ApiFilterCounts struct {
	Status     []FilterCount
	OasVersion []FilterCount
	AdrScore   []FilterCount
	Auth       []FilterCount
}

type ApiFiltersParams struct {
	Organisation *string  `query:"organisation"`
	Ids          *string  `query:"ids"`
	Status       []string `query:"status"`
	OasVersion   []string `query:"oasVersion"`
	Version      []string `query:"version"`
	AdrScore     []string `query:"adrScore"`
	Auth         []string `query:"auth"`
}

var LifecycleStatusLabels = map[string][2]string{
	"active":     {"Actief", "De API is actief beschikbaar."},
	"deprecated": {"Deprecated", "De API is verouderd, maar nog beschikbaar."},
	"sunset":     {"Sunset", "De API heeft een toekomstige uitfaseringsdatum."},
	"retired":    {"Retired", "De API is uitgefaseerd."},
}

var AuthLabels = map[string][2]string{
	"none":    {"Geen beveiliging", "Er is geen security-definitie in de OAS gevonden."},
	"api_key": {"API key", "De API gebruikt een API key."},
	"oauth2":  {"OAuth 2.0", "De API gebruikt OAuth 2.0."},
	"openid":  {"OpenID Connect", "De API gebruikt OpenID Connect."},
	"bearer":  {"Bearer token", "De API gebruikt HTTP bearer authenticatie."},
	"basic":   {"Basic auth", "De API gebruikt HTTP basic authenticatie."},
	"http":    {"HTTP auth", "De API gebruikt HTTP authenticatie."},
	"unknown": {"Onbekend", "De OAS bevat een security-definitie die niet herkend is."},
}

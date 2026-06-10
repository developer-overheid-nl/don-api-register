package models

import commonfilters "github.com/developer-overheid-nl/don-register-common/filters"

type FilterOption = commonfilters.FilterOption

type FilterGroup = commonfilters.FilterGroup

type FilterCount = commonfilters.FilterCount

type ApiFilterCounts struct {
	Organisation []FilterCount
	Status       []FilterCount
	OasVersion   []FilterCount
	AdrScore     []FilterCount
	Auth         []FilterCount
}

type ApiFiltersParams struct {
	Organisation *string  `query:"organisation"`
	Query        string   `query:"q"`
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

package services

import (
	"strings"

	"github.com/developer-overheid-nl/don-api-register/pkg/api_client/models"
)

func buildStatusGroup(p *models.ApiFiltersParams, counts *models.ApiFilterCounts) models.FilterGroup {
	return models.FilterGroup{
		Key:         "status",
		Label:       "Lifecycle status",
		Description: "De lifecycle status van de API.",
		Type:        "multi-select",
		Options:     buildLabeledOptions(counts.Status, selectedLowerSet(p.Status), models.LifecycleStatusLabels),
	}
}

func buildOasVersionGroup(p *models.ApiFiltersParams, counts *models.ApiFilterCounts) models.FilterGroup {
	selected := selectedSet(p.OasVersion, p.Version)
	options := make([]models.FilterOption, 0, len(counts.OasVersion))
	for _, fc := range counts.OasVersion {
		label := fc.Value
		var desc *string
		if fc.Value == "unknown" {
			label = "Onbekend"
			d := "Er is geen versie uit de OAS bekend."
			desc = &d
		}
		options = append(options, models.FilterOption{
			Value:       fc.Value,
			Label:       label,
			Description: desc,
			Count:       fc.Count,
			Selected:    selected[fc.Value],
		})
	}
	return models.FilterGroup{
		Key:         "oasVersion",
		Label:       "OAS versie",
		Description: "De API versie uit de OAS info.version.",
		Type:        "multi-select",
		Options:     options,
	}
}

func buildAdrScoreGroup(p *models.ApiFiltersParams, counts *models.ApiFilterCounts) models.FilterGroup {
	selected := selectedSet(p.AdrScore)
	options := make([]models.FilterOption, 0, len(counts.AdrScore))
	for _, fc := range counts.AdrScore {
		label := fc.Value
		var desc *string
		if fc.Value == "unknown" {
			label = "Niet bekend"
			d := "Er is nog geen ADR score opgeslagen."
			desc = &d
		}
		options = append(options, models.FilterOption{
			Value:       fc.Value,
			Label:       label,
			Description: desc,
			Count:       fc.Count,
			Selected:    selected[fc.Value],
		})
	}
	return models.FilterGroup{
		Key:         "adrScore",
		Label:       "ADR score",
		Description: "De opgeslagen API Design Rules score.",
		Type:        "multi-select",
		Options:     options,
	}
}

func buildAuthGroup(p *models.ApiFiltersParams, counts *models.ApiFilterCounts) models.FilterGroup {
	return models.FilterGroup{
		Key:         "auth",
		Label:       "Beveiliging",
		Description: "De authenticatievorm die uit de OAS security-definitie is afgeleid.",
		Type:        "multi-select",
		Options:     buildLabeledOptions(counts.Auth, selectedSet(normalizeAuthSelection(p.Auth)), models.AuthLabels),
	}
}

func buildLabeledOptions(counts []models.FilterCount, selected map[string]bool, labels map[string][2]string) []models.FilterOption {
	options := make([]models.FilterOption, 0, len(counts))
	for _, fc := range counts {
		label := fc.Value
		var desc *string
		if meta, ok := labels[fc.Value]; ok {
			label = meta[0]
			d := meta[1]
			desc = &d
		}
		options = append(options, models.FilterOption{
			Value:       fc.Value,
			Label:       label,
			Description: desc,
			Count:       fc.Count,
			Selected:    selected[fc.Value],
		})
	}
	return options
}

func selectedSet(groups ...[]string) map[string]bool {
	m := make(map[string]bool)
	for _, values := range groups {
		for _, raw := range values {
			for _, val := range strings.Split(raw, ",") {
				trimmed := strings.TrimSpace(val)
				if trimmed != "" {
					m[trimmed] = true
				}
			}
		}
	}
	return m
}

func selectedLowerSet(groups ...[]string) map[string]bool {
	values := selectedSet(groups...)
	lowered := make(map[string]bool, len(values))
	for val := range values {
		lowered[strings.ToLower(val)] = true
	}
	return lowered
}

func normalizeAuthSelection(values []string) []string {
	normalized := make([]string, 0, len(values))
	for _, raw := range values {
		for _, val := range strings.Split(raw, ",") {
			trimmed := strings.ToLower(strings.TrimSpace(val))
			switch trimmed {
			case "":
				continue
			case "apikey", "api-key", "api key":
				normalized = append(normalized, "api_key")
			case "openidconnect", "openid-connect":
				normalized = append(normalized, "openid")
			default:
				normalized = append(normalized, trimmed)
			}
		}
	}
	return normalized
}

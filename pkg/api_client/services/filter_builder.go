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

func buildOrganisationGroup(p *models.ApiFiltersParams, counts *models.ApiFilterCounts) models.FilterGroup {
	selected := map[string]bool{}
	if p != nil && p.Organisation != nil {
		if value := strings.TrimSpace(*p.Organisation); value != "" {
			selected[value] = true
		}
	}

	options := make([]models.FilterOption, 0, len(counts.Organisation))
	for _, fc := range counts.Organisation {
		label := fc.Label
		if label == "" {
			label = fc.Value
		}
		options = append(options, models.FilterOption{
			Value:    fc.Value,
			Label:    label,
			Count:    fc.Count,
			Selected: selected[fc.Value],
		})
	}
	options = appendMissingSelectedOptions(options, selected, func(value string) models.FilterOption {
		return models.FilterOption{
			Value:    value,
			Label:    value,
			Selected: true,
		}
	})

	return models.FilterGroup{
		Key:         "organisation",
		Label:       "Organisatie",
		Description: "De organisatie die eigenaar is van de API.",
		Type:        "single-select",
		Options:     options,
	}
}

func buildOasVersionGroup(p *models.ApiFiltersParams, counts *models.ApiFilterCounts) models.FilterGroup {
	selected := selectedSet(p.OasVersion, p.Version)
	options := make([]models.FilterOption, 0, len(counts.OasVersion))
	for _, fc := range counts.OasVersion {
		options = append(options, oasVersionOption(fc.Value, fc.Count, selected[fc.Value]))
	}
	options = appendMissingSelectedOptions(options, selected, func(value string) models.FilterOption {
		return oasVersionOption(value, 0, true)
	})
	return models.FilterGroup{
		Key:         "oasVersion",
		Label:       "OAS versie",
		Description: "De OpenAPI-versie van het document.",
		Type:        "multi-select",
		Options:     options,
	}
}

func buildAdrScoreGroup(p *models.ApiFiltersParams, counts *models.ApiFilterCounts) models.FilterGroup {
	selected := selectedSet(p.AdrScore)
	options := make([]models.FilterOption, 0, len(counts.AdrScore))
	for _, fc := range counts.AdrScore {
		options = append(options, adrScoreOption(fc.Value, fc.Count, selected[fc.Value]))
	}
	options = appendMissingSelectedOptions(options, selected, func(value string) models.FilterOption {
		return adrScoreOption(value, 0, true)
	})
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
		options = append(options, labeledOption(fc.Value, fc.Count, selected[fc.Value], labels))
	}
	options = appendMissingSelectedOptions(options, selected, func(value string) models.FilterOption {
		return labeledOption(value, 0, true, labels)
	})
	return options
}

func oasVersionOption(value string, count int, selected bool) models.FilterOption {
	label := value
	var desc *string
	if value == "unknown" {
		label = "Onbekend"
		d := "Er is geen OpenAPI-versie bekend."
		desc = &d
	}
	return models.FilterOption{
		Value:       value,
		Label:       label,
		Description: desc,
		Count:       count,
		Selected:    selected,
	}
}

func adrScoreOption(value string, count int, selected bool) models.FilterOption {
	label := value
	var desc *string
	if value == "unknown" {
		label = "Niet bekend"
		d := "Er is nog geen ADR score opgeslagen."
		desc = &d
	}
	return models.FilterOption{
		Value:       value,
		Label:       label,
		Description: desc,
		Count:       count,
		Selected:    selected,
	}
}

func labeledOption(value string, count int, selected bool, labels map[string][2]string) models.FilterOption {
	label := value
	var desc *string
	if meta, ok := labels[value]; ok {
		label = meta[0]
		d := meta[1]
		desc = &d
	}
	return models.FilterOption{
		Value:       value,
		Label:       label,
		Description: desc,
		Count:       count,
		Selected:    selected,
	}
}

func appendMissingSelectedOptions(options []models.FilterOption, selected map[string]bool, build func(string) models.FilterOption) []models.FilterOption {
	seen := make(map[string]bool, len(options))
	for _, option := range options {
		seen[option.Value] = true
	}
	for value, isSelected := range selected {
		if value == "" || !isSelected || seen[value] {
			continue
		}
		options = append(options, build(value))
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

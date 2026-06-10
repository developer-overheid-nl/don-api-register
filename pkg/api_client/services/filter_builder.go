package services

import (
	"strings"

	"github.com/developer-overheid-nl/don-api-register/pkg/api_client/models"
	commonfilters "github.com/developer-overheid-nl/don-register-common/filters"
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
	return commonfilters.LabeledOptions(counts, selected, labels, false)
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

func appendMissingSelectedOptions(options []models.FilterOption, selected map[string]bool, build func(string) models.FilterOption) []models.FilterOption {
	return commonfilters.AppendMissingSelectedOptions(options, selected, build)
}

func selectedSet(groups ...[]string) map[string]bool {
	return commonfilters.SelectedSet(groups...)
}

func selectedLowerSet(groups ...[]string) map[string]bool {
	return commonfilters.SelectedLowerSet(groups...)
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

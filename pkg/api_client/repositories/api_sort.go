package repositories

import (
	"cmp"
	"sort"
	"strings"

	"github.com/developer-overheid-nl/don-api-register/pkg/api_client/models"
	"golang.org/x/mod/semver"
)

func sortApis(apis []models.Api, sorting models.ApiSort) {
	sort.SliceStable(apis, func(i, j int) bool {
		primary, leftValid, rightValid := compareApiSortField(apis[i], apis[j], sorting.Field)
		if leftValid != rightValid {
			return leftValid
		}
		if leftValid && primary != 0 {
			if sorting.Order == models.ApiSortDescending {
				return primary > 0
			}
			return primary < 0
		}

		return compareApiTieBreakers(apis[i], apis[j]) < 0
	})
}

func compareApiSortField(left, right models.Api, field models.ApiSortField) (comparison int, leftValid, rightValid bool) {
	switch field {
	case models.ApiSortADRScore:
		if left.AdrScore == nil || right.AdrScore == nil {
			return 0, left.AdrScore != nil, right.AdrScore != nil
		}
		return cmp.Compare(*left.AdrScore, *right.AdrScore), true, true
	case models.ApiSortVersion:
		leftVersion, leftOK := normalizedAPIVersion(left.Version)
		rightVersion, rightOK := normalizedAPIVersion(right.Version)
		if !leftOK || !rightOK {
			return 0, leftOK, rightOK
		}
		return semver.Compare(leftVersion, rightVersion), true, true
	default:
		return strings.Compare(strings.ToLower(left.Title), strings.ToLower(right.Title)), true, true
	}
}

func compareApiTieBreakers(left, right models.Api) int {
	if titleComparison := strings.Compare(strings.ToLower(left.Title), strings.ToLower(right.Title)); titleComparison != 0 {
		return titleComparison
	}
	return strings.Compare(left.Id, right.Id)
}

func normalizedAPIVersion(raw string) (string, bool) {
	value := strings.TrimSpace(raw)
	if len(value) > 0 && (value[0] == 'v' || value[0] == 'V') {
		value = value[1:]
	}
	if value == "" {
		return "", false
	}

	coreEnd := len(value)
	if suffixStart := strings.IndexAny(value, "-+"); suffixStart >= 0 {
		coreEnd = suffixStart
	}
	core := value[:coreEnd]
	suffix := value[coreEnd:]
	parts := strings.Split(core, ".")
	if len(parts) == 0 || len(parts) > 3 {
		return "", false
	}
	for _, part := range parts {
		if part == "" {
			return "", false
		}
		for _, character := range part {
			if character < '0' || character > '9' {
				return "", false
			}
		}
	}
	for len(parts) < 3 {
		parts = append(parts, "0")
	}

	normalized := "v" + strings.Join(parts, ".") + suffix
	if !semver.IsValid(normalized) {
		return "", false
	}
	return normalized, true
}

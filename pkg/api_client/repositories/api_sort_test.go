package repositories

import (
	"testing"

	"github.com/developer-overheid-nl/don-api-register/pkg/api_client/models"
	"github.com/stretchr/testify/assert"
)

func TestSortApisOrdersTitlesCaseInsensitively(t *testing.T) {
	tests := []struct {
		name  string
		order models.ApiSortOrder
		want  []string
	}{
		{
			name:  "ascending",
			order: models.ApiSortAscending,
			want:  []string{"alpha-lower", "alpha-upper", "beta", "zulu"},
		},
		{
			name:  "descending",
			order: models.ApiSortDescending,
			want:  []string{"zulu", "beta", "alpha-lower", "alpha-upper"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			apis := []models.Api{
				{Id: "zulu", Title: "Zulu"},
				{Id: "alpha-upper", Title: "Alpha"},
				{Id: "beta", Title: "beta"},
				{Id: "alpha-lower", Title: "alpha"},
			}

			sortApis(apis, models.ApiSort{Field: models.ApiSortTitle, Order: tt.order})

			assert.Equal(t, tt.want, apiIDs(apis))
		})
	}
}

func TestSortApisOrdersADRScoresNumericallyWithMissingLast(t *testing.T) {
	tests := []struct {
		name  string
		order models.ApiSortOrder
		want  []string
	}{
		{
			name:  "ascending",
			order: models.ApiSortAscending,
			want:  []string{"score-10", "score-90", "score-missing"},
		},
		{
			name:  "descending",
			order: models.ApiSortDescending,
			want:  []string{"score-90", "score-10", "score-missing"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ten := 10
			ninety := 90
			apis := []models.Api{
				{Id: "score-missing", Title: "Charlie"},
				{Id: "score-90", Title: "Bravo", AdrScore: &ninety},
				{Id: "score-10", Title: "Alpha", AdrScore: &ten},
			}

			sortApis(apis, models.ApiSort{Field: models.ApiSortADRScore, Order: tt.order})

			assert.Equal(t, tt.want, apiIDs(apis))
		})
	}
}

func TestSortApisOrdersVersionsSemanticallyWithInvalidLast(t *testing.T) {
	tests := []struct {
		name  string
		order models.ApiSortOrder
		want  []string
	}{
		{
			name:  "ascending",
			order: models.ApiSortAscending,
			want: []string{
				"version-prerelease",
				"version-1-9",
				"version-1-10",
				"version-2",
				"version-invalid",
				"version-empty",
			},
		},
		{
			name:  "descending",
			order: models.ApiSortDescending,
			want: []string{
				"version-2",
				"version-1-10",
				"version-1-9",
				"version-prerelease",
				"version-invalid",
				"version-empty",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			apis := []models.Api{
				{Id: "version-empty", Title: "Echo", Version: ""},
				{Id: "version-2", Title: "Charlie", Version: "2"},
				{Id: "version-invalid", Title: "Delta", Version: "release-next"},
				{Id: "version-1-10", Title: "Bravo", Version: "V1.10"},
				{Id: "version-prerelease", Title: "Aardvark", Version: "v1.9.0-rc.1"},
				{Id: "version-1-9", Title: "Alpha", Version: "1.9"},
			}

			sortApis(apis, models.ApiSort{Field: models.ApiSortVersion, Order: tt.order})

			assert.Equal(t, tt.want, apiIDs(apis))
		})
	}
}

func TestSortApisUsesTitleThenIDAsTieBreakers(t *testing.T) {
	fifty := 50
	apis := []models.Api{
		{Id: "api-b", Title: "Same", AdrScore: &fifty},
		{Id: "api-c", Title: "Zulu", AdrScore: &fifty},
		{Id: "api-a", Title: "same", AdrScore: &fifty},
	}

	sortApis(apis, models.ApiSort{Field: models.ApiSortADRScore, Order: models.ApiSortDescending})

	assert.Equal(t, []string{"api-a", "api-b", "api-c"}, apiIDs(apis))
}

func apiIDs(apis []models.Api) []string {
	ids := make([]string, len(apis))
	for i := range apis {
		ids[i] = apis[i].Id
	}
	return ids
}

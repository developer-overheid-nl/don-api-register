package models

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseApiSortDefaultsToTitleAscending(t *testing.T) {
	got, err := ParseApiSort("", "")

	require.NoError(t, err)
	assert.Equal(t, ApiSort{Field: ApiSortTitle, Order: ApiSortAscending}, got)
}

func TestParseApiSortAcceptsSupportedValues(t *testing.T) {
	tests := []struct {
		name      string
		sortBy    string
		sortOrder string
		want      ApiSort
	}{
		{
			name:      "title descending",
			sortBy:    "title",
			sortOrder: "desc",
			want:      ApiSort{Field: ApiSortTitle, Order: ApiSortDescending},
		},
		{
			name:      "ADR score ascending",
			sortBy:    "adrScore",
			sortOrder: "asc",
			want:      ApiSort{Field: ApiSortADRScore, Order: ApiSortAscending},
		},
		{
			name:      "version descending",
			sortBy:    "version",
			sortOrder: "desc",
			want:      ApiSort{Field: ApiSortVersion, Order: ApiSortDescending},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseApiSort(tt.sortBy, tt.sortOrder)

			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestParseApiSortRejectsUnsupportedSortBy(t *testing.T) {
	_, err := ParseApiSort("createdAt", "asc")

	var invalid InvalidApiSortError
	require.ErrorAs(t, err, &invalid)
	assert.Equal(t, "sortBy", invalid.Parameter)
	assert.Equal(t, "createdAt", invalid.Value)
}

func TestParseApiSortRejectsUnsupportedSortOrder(t *testing.T) {
	_, err := ParseApiSort("title", "sideways")

	var invalid InvalidApiSortError
	require.ErrorAs(t, err, &invalid)
	assert.Equal(t, "sortOrder", invalid.Parameter)
	assert.Equal(t, "sideways", invalid.Value)
}

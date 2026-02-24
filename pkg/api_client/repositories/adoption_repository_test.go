package repositories

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func setupAdoptionRepoTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	return db
}

func TestLatestResultsCTE_BuildsSQLWithFiltersAndArgsOrder(t *testing.T) {
	end := time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)
	org := "  org-1  "
	cte, args := latestResultsCTE(AdoptionQueryParams{
		AdrVersion:   "ADR-2.0",
		EndDate:      end,
		ApiIds:       []string{"api-1", "api-2"},
		Organisation: &org,
	}, "lr.api_id, lr.successes")

	assert.Contains(t, cte, "WITH latest_results AS")
	assert.Contains(t, cte, "SELECT DISTINCT ON (lr.api_id) lr.api_id, lr.successes")
	assert.Contains(t, cte, "lr.created_at <= ? AND lr.adr_version = ?")
	assert.Contains(t, cte, "lr.api_id IN (?,?)")
	assert.Contains(t, cte, "SELECT id FROM apis WHERE organisation_id = ?")
	assert.Contains(t, cte, "ORDER BY lr.api_id, lr.created_at DESC")

	require.Len(t, args, 5)
	assert.Equal(t, end, args[0])
	assert.Equal(t, "ADR-2.0", args[1])
	assert.Equal(t, "api-1", args[2])
	assert.Equal(t, "api-2", args[3])
	assert.Equal(t, "org-1", args[4])
}

func TestLatestResultsCTE_IgnoresBlankOrganisation(t *testing.T) {
	org := "   "
	cte, args := latestResultsCTE(AdoptionQueryParams{
		AdrVersion:   "ADR-2.0",
		EndDate:      time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC),
		Organisation: &org,
	}, "lr.api_id")

	assert.NotContains(t, cte, "organisation_id = ?")
	require.Len(t, args, 2)
	assert.Equal(t, "ADR-2.0", args[1])
}

func TestAdoptionRepositoryGetSummary_WrapsQueryError(t *testing.T) {
	repo := &adoptionRepository{db: setupAdoptionRepoTestDB(t)}

	_, _, err := repo.GetSummary(context.Background(), AdoptionQueryParams{
		AdrVersion: "ADR-2.0",
		StartDate:  time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		EndDate:    time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC),
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "summary query failed")
}

func TestAdoptionRepositoryGetRules_WrapsQueryError(t *testing.T) {
	repo := &adoptionRepository{db: setupAdoptionRepoTestDB(t)}

	_, _, err := repo.GetRules(context.Background(), AdoptionQueryParams{
		AdrVersion: "ADR-2.0",
		EndDate:    time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC),
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "rules query failed")
}

func TestAdoptionRepositoryGetTimeline_WrapsRuleCodesQueryError(t *testing.T) {
	repo := &adoptionRepository{db: setupAdoptionRepoTestDB(t)}

	_, err := repo.GetTimeline(context.Background(), TimelineQueryParams{
		AdoptionQueryParams: AdoptionQueryParams{
			AdrVersion: "ADR-2.0",
			StartDate:  time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
			EndDate:    time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC),
		},
		Granularity: "month",
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "rule codes query failed")
}

func TestAdoptionRepositoryGetApis_WrapsCountQueryError(t *testing.T) {
	repo := &adoptionRepository{db: setupAdoptionRepoTestDB(t)}

	_, _, err := repo.GetApis(context.Background(), ApisQueryParams{
		AdoptionQueryParams: AdoptionQueryParams{
			AdrVersion: "ADR-2.0",
			EndDate:    time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC),
		},
		Page:    1,
		PerPage: 20,
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "apis count query failed")
}

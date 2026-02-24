package services_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/developer-overheid-nl/don-api-register/pkg/api_client/models"
	"github.com/developer-overheid-nl/don-api-register/pkg/api_client/repositories"
	"github.com/developer-overheid-nl/don-api-register/pkg/api_client/services"
	"github.com/lib/pq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type adoptionRepoStub struct {
	getSummaryFunc  func(ctx context.Context, params repositories.AdoptionQueryParams) (repositories.SummaryResult, int, error)
	getRulesFunc    func(ctx context.Context, params repositories.AdoptionQueryParams) ([]repositories.RuleRow, int, error)
	getTimelineFunc func(ctx context.Context, params repositories.TimelineQueryParams) ([]repositories.TimelineRow, error)
	getApisFunc     func(ctx context.Context, params repositories.ApisQueryParams) ([]repositories.ApiRow, int, error)
}

func (s *adoptionRepoStub) GetSummary(ctx context.Context, params repositories.AdoptionQueryParams) (repositories.SummaryResult, int, error) {
	if s.getSummaryFunc != nil {
		return s.getSummaryFunc(ctx, params)
	}
	return repositories.SummaryResult{}, 0, nil
}

func (s *adoptionRepoStub) GetRules(ctx context.Context, params repositories.AdoptionQueryParams) ([]repositories.RuleRow, int, error) {
	if s.getRulesFunc != nil {
		return s.getRulesFunc(ctx, params)
	}
	return nil, 0, nil
}

func (s *adoptionRepoStub) GetTimeline(ctx context.Context, params repositories.TimelineQueryParams) ([]repositories.TimelineRow, error) {
	if s.getTimelineFunc != nil {
		return s.getTimelineFunc(ctx, params)
	}
	return nil, nil
}

func (s *adoptionRepoStub) GetApis(ctx context.Context, params repositories.ApisQueryParams) ([]repositories.ApiRow, int, error) {
	if s.getApisFunc != nil {
		return s.getApisFunc(ctx, params)
	}
	return nil, 0, nil
}

func strPtr(s string) *string { return &s }
func boolPtr(v bool) *bool    { return &v }

func TestAdoptionServiceGetSummary_Success(t *testing.T) {
	var captured repositories.AdoptionQueryParams
	repo := &adoptionRepoStub{
		getSummaryFunc: func(ctx context.Context, params repositories.AdoptionQueryParams) (repositories.SummaryResult, int, error) {
			captured = params
			return repositories.SummaryResult{
				TotalApis:     10,
				CompliantApis: 7,
				AdoptionRate:  70.0,
			}, 42, nil
		},
	}
	svc := services.NewAdoptionService(repo)

	in := &models.AdoptionBaseParams{
		AdrVersion:   "ADR-2.0",
		StartDate:    "2026-01-01",
		EndDate:      "2026-01-31",
		ApiIds:       strPtr(" api-1, , api-2 "),
		Organisation: strPtr("  org-1  "),
	}

	out, err := svc.GetSummary(context.Background(), in)
	require.NoError(t, err)
	require.NotNil(t, out)

	assert.Equal(t, "ADR-2.0", captured.AdrVersion)
	assert.Equal(t, []string{"api-1", "api-2"}, captured.ApiIds)
	if assert.NotNil(t, captured.Organisation) {
		assert.Equal(t, "org-1", *captured.Organisation)
	}
	assert.Equal(t, time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), captured.StartDate)
	assert.Equal(t, time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC), captured.EndDate)

	assert.Equal(t, "ADR-2.0", out.AdrVersion)
	assert.Equal(t, models.Period{Start: "2026-01-01", End: "2026-01-31"}, out.Period)
	assert.Equal(t, 10, out.TotalApis)
	assert.Equal(t, 7, out.CompliantApis)
	assert.Equal(t, 70.0, out.OverallAdoptionRate)
	assert.Equal(t, 42, out.TotalLintRuns)
}

func TestAdoptionServiceGetSummary_InvalidDate(t *testing.T) {
	called := false
	repo := &adoptionRepoStub{
		getSummaryFunc: func(ctx context.Context, params repositories.AdoptionQueryParams) (repositories.SummaryResult, int, error) {
			called = true
			return repositories.SummaryResult{}, 0, nil
		},
	}
	svc := services.NewAdoptionService(repo)

	_, err := svc.GetSummary(context.Background(), &models.AdoptionBaseParams{
		AdrVersion: "ADR-2.0",
		StartDate:  "not-a-date",
		EndDate:    "2026-01-31",
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid startDate")
	assert.False(t, called)
}

func TestAdoptionServiceGetRules_Success(t *testing.T) {
	var captured repositories.AdoptionQueryParams
	repo := &adoptionRepoStub{
		getRulesFunc: func(ctx context.Context, params repositories.AdoptionQueryParams) ([]repositories.RuleRow, int, error) {
			captured = params
			return []repositories.RuleRow{
				{Code: "ADR-001", Severity: "warning", ViolatingApis: 2},
				{Code: "ADR-002", Severity: "warning", ViolatingApis: 10},
			}, 10, nil
		},
	}
	svc := services.NewAdoptionService(repo)

	out, err := svc.GetRules(context.Background(), &models.AdoptionRulesParams{
		AdoptionBaseParams: models.AdoptionBaseParams{
			AdrVersion:   "ADR-2.0",
			StartDate:    "2026-01-01",
			EndDate:      "2026-01-31",
			ApiIds:       strPtr("api-1, api-2"),
			Organisation: strPtr(" org-1 "),
		},
		RuleCodes: strPtr("ADR-001, ADR-002"),
		Severity:  strPtr(" Warning "),
	})
	require.NoError(t, err)

	if assert.NotNil(t, captured.Severity) {
		assert.Equal(t, "warning", *captured.Severity)
	}
	assert.Equal(t, []string{"ADR-001", "ADR-002"}, captured.RuleCodes)
	assert.Equal(t, []string{"api-1", "api-2"}, captured.ApiIds)
	if assert.NotNil(t, captured.Organisation) {
		assert.Equal(t, "org-1", *captured.Organisation)
	}
	assert.Equal(t, time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC), captured.EndDate)

	require.Len(t, out.Rules, 2)
	assert.Equal(t, 10, out.TotalApis)
	assert.Equal(t, 8, out.Rules[0].CompliantApis)
	assert.Equal(t, 80.0, out.Rules[0].AdoptionRate)
	assert.Equal(t, 0, out.Rules[1].CompliantApis)
	assert.Equal(t, 0.0, out.Rules[1].AdoptionRate)
}

func TestAdoptionServiceGetRules_InvalidSeverity(t *testing.T) {
	called := false
	repo := &adoptionRepoStub{
		getRulesFunc: func(ctx context.Context, params repositories.AdoptionQueryParams) ([]repositories.RuleRow, int, error) {
			called = true
			return nil, 0, nil
		},
	}
	svc := services.NewAdoptionService(repo)

	_, err := svc.GetRules(context.Background(), &models.AdoptionRulesParams{
		AdoptionBaseParams: models.AdoptionBaseParams{AdrVersion: "ADR-2.0", StartDate: "2026-01-01", EndDate: "2026-01-31"},
		Severity:           strPtr("fatal"),
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid severity")
	assert.False(t, called)
}

func TestAdoptionServiceGetTimeline_Success_DefaultGranularityAndGrouping(t *testing.T) {
	var captured repositories.TimelineQueryParams
	repo := &adoptionRepoStub{
		getTimelineFunc: func(ctx context.Context, params repositories.TimelineQueryParams) ([]repositories.TimelineRow, error) {
			captured = params
			return []repositories.TimelineRow{
				{RuleCode: "ADR-001", Period: "2026-01", TotalApis: 10, CompliantApis: 7},
				{RuleCode: "ADR-001", Period: "2026-02", TotalApis: 12, CompliantApis: 9},
				{RuleCode: "ADR-002", Period: "2026-01", TotalApis: 10, CompliantApis: 10},
			}, nil
		},
	}
	svc := services.NewAdoptionService(repo)

	out, err := svc.GetTimeline(context.Background(), &models.AdoptionTimelineParams{
		AdoptionBaseParams: models.AdoptionBaseParams{AdrVersion: "ADR-2.0", StartDate: "2026-01-01", EndDate: "2026-02-28"},
		RuleCodes:          strPtr("ADR-001,ADR-002"),
	})
	require.NoError(t, err)

	assert.Equal(t, "month", captured.Granularity)
	assert.Equal(t, []string{"ADR-001", "ADR-002"}, captured.RuleCodes)
	assert.Equal(t, "month", out.Granularity)
	require.Len(t, out.Series, 2)
	assert.Equal(t, "ADR-001", out.Series[0].RuleCode)
	require.Len(t, out.Series[0].DataPoints, 2)
	assert.Equal(t, 70.0, out.Series[0].DataPoints[0].AdoptionRate)
	assert.Equal(t, 75.0, out.Series[0].DataPoints[1].AdoptionRate)
	assert.Equal(t, "ADR-002", out.Series[1].RuleCode)
	assert.Equal(t, 100.0, out.Series[1].DataPoints[0].AdoptionRate)
}

func TestAdoptionServiceGetTimeline_InvalidGranularity(t *testing.T) {
	svc := services.NewAdoptionService(&adoptionRepoStub{})

	_, err := svc.GetTimeline(context.Background(), &models.AdoptionTimelineParams{
		AdoptionBaseParams: models.AdoptionBaseParams{AdrVersion: "ADR-2.0", StartDate: "2026-01-01", EndDate: "2026-01-31"},
		Granularity:        "quarter",
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid granularity")
}

func TestAdoptionServiceGetApis_Success_PaginationAndMapping(t *testing.T) {
	var captured repositories.ApisQueryParams
	repo := &adoptionRepoStub{
		getApisFunc: func(ctx context.Context, params repositories.ApisQueryParams) ([]repositories.ApiRow, int, error) {
			captured = params
			return []repositories.ApiRow{
				{
					ApiId:         "api-1",
					ApiTitle:      "API One",
					Organisation:  "Org 1",
					IsCompliant:   false,
					TotalFailures: 2,
					TotalWarnings: 1,
					ViolatedRules: pq.StringArray{"ADR-001", "ADR-002"},
					LastLintDate:  time.Date(2026, 1, 15, 12, 0, 0, 0, time.UTC),
				},
			}, 205, nil
		},
	}
	svc := services.NewAdoptionService(repo)

	out, pagination, err := svc.GetApis(context.Background(), &models.AdoptionApisParams{
		AdoptionBaseParams: models.AdoptionBaseParams{AdrVersion: "ADR-2.0", StartDate: "2026-01-01", EndDate: "2026-01-31"},
		Compliant:          boolPtr(false),
		RuleCodes:          strPtr("ADR-001, ADR-002"),
		Page:               0,
		PerPage:            999,
	})
	require.NoError(t, err)
	require.NotNil(t, pagination)

	assert.Equal(t, 1, captured.Page)
	assert.Equal(t, 100, captured.PerPage)
	if assert.NotNil(t, captured.Compliant) {
		assert.False(t, *captured.Compliant)
	}
	assert.Equal(t, []string{"ADR-001", "ADR-002"}, captured.RuleCodes)
	assert.Equal(t, time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC), captured.EndDate)

	require.Len(t, out.Apis, 1)
	assert.Equal(t, "api-1", out.Apis[0].ApiId)
	assert.Equal(t, []string{"ADR-001", "ADR-002"}, out.Apis[0].ViolatedRules)
	assert.Equal(t, 2, out.Apis[0].TotalViolations)
	assert.Equal(t, 1, out.Apis[0].TotalWarnings)

	assert.Equal(t, 1, pagination.CurrentPage)
	assert.Equal(t, 100, pagination.RecordsPerPage)
	assert.Equal(t, 3, pagination.TotalPages)
	assert.Equal(t, 205, pagination.TotalRecords)
	if assert.NotNil(t, pagination.Next) {
		assert.Equal(t, 2, *pagination.Next)
	}
	assert.Nil(t, pagination.Previous)
}

func TestAdoptionServiceGetApis_RepoError(t *testing.T) {
	repo := &adoptionRepoStub{
		getApisFunc: func(ctx context.Context, params repositories.ApisQueryParams) ([]repositories.ApiRow, int, error) {
			return nil, 0, errors.New("db kapot")
		},
	}
	svc := services.NewAdoptionService(repo)

	out, pagination, err := svc.GetApis(context.Background(), &models.AdoptionApisParams{
		AdoptionBaseParams: models.AdoptionBaseParams{AdrVersion: "ADR-2.0", StartDate: "2026-01-01", EndDate: "2026-01-31"},
	})

	require.Error(t, err)
	assert.Nil(t, out)
	assert.Nil(t, pagination)
	assert.Contains(t, err.Error(), "db kapot")
}

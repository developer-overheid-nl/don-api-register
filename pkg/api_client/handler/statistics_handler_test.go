package handler

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/developer-overheid-nl/don-api-register/pkg/api_client/models"
	"github.com/developer-overheid-nl/don-api-register/pkg/api_client/repositories"
	"github.com/developer-overheid-nl/don-api-register/pkg/api_client/services"
	"github.com/gin-gonic/gin"
	"github.com/lib/pq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type statisticsAdoptionRepoStub struct {
	getSummaryFunc func(ctx context.Context, params repositories.AdoptionQueryParams) (repositories.SummaryResult, int, error)
	getApisFunc    func(ctx context.Context, params repositories.ApisQueryParams) ([]repositories.ApiRow, int, error)
}

func (s *statisticsAdoptionRepoStub) GetSummary(ctx context.Context, params repositories.AdoptionQueryParams) (repositories.SummaryResult, int, error) {
	if s.getSummaryFunc != nil {
		return s.getSummaryFunc(ctx, params)
	}
	return repositories.SummaryResult{}, 0, nil
}

func (s *statisticsAdoptionRepoStub) GetRules(ctx context.Context, params repositories.AdoptionQueryParams) ([]repositories.RuleRow, int, error) {
	return nil, 0, nil
}

func (s *statisticsAdoptionRepoStub) GetTimeline(ctx context.Context, params repositories.TimelineQueryParams) ([]repositories.TimelineRow, error) {
	return nil, nil
}

func (s *statisticsAdoptionRepoStub) GetApis(ctx context.Context, params repositories.ApisQueryParams) ([]repositories.ApiRow, int, error) {
	if s.getApisFunc != nil {
		return s.getApisFunc(ctx, params)
	}
	return nil, 0, nil
}

func TestStatisticsControllerGetSummary_Success(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &statisticsAdoptionRepoStub{
		getSummaryFunc: func(ctx context.Context, params repositories.AdoptionQueryParams) (repositories.SummaryResult, int, error) {
			return repositories.SummaryResult{
				TotalApis:     5,
				CompliantApis: 4,
				AdoptionRate:  80,
			}, 11, nil
		},
	}
	ctrl := NewStatisticsController(services.NewAdoptionService(repo))

	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/v1/statistics/summary", nil)

	out, err := ctrl.GetSummary(ctx, &models.AdoptionBaseParams{
		AdrVersion: "ADR-2.0",
		StartDate:  "2026-01-01",
		EndDate:    "2026-01-31",
	})
	require.NoError(t, err)
	require.NotNil(t, out)
	assert.Equal(t, 5, out.TotalApis)
	assert.Equal(t, 4, out.CompliantApis)
	assert.Equal(t, 80.0, out.OverallAdoptionRate)
	assert.Equal(t, 11, out.TotalLintRuns)
}

func TestStatisticsControllerGetApis_SetsPaginationHeaders(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &statisticsAdoptionRepoStub{
		getApisFunc: func(ctx context.Context, params repositories.ApisQueryParams) ([]repositories.ApiRow, int, error) {
			return []repositories.ApiRow{
				{
					ApiId:         "api-1",
					ApiTitle:      "API 1",
					Organisation:  "Org",
					IsCompliant:   true,
					TotalFailures: 0,
					TotalWarnings: 1,
					ViolatedRules: pq.StringArray{},
					LastLintDate:  time.Date(2026, 1, 10, 9, 0, 0, 0, time.UTC),
				},
			}, 3, nil
		},
	}
	ctrl := NewStatisticsController(services.NewAdoptionService(repo))

	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)
	req := httptest.NewRequest(http.MethodGet, "http://api.example.test/v1/statistics/apis?page=2&perPage=1", nil)
	req.Host = "api.example.test"
	ctx.Request = req

	out, err := ctrl.GetApis(ctx, &models.AdoptionApisParams{
		AdoptionBaseParams: models.AdoptionBaseParams{AdrVersion: "ADR-2.0", StartDate: "2026-01-01", EndDate: "2026-01-31"},
		Page:               2,
		PerPage:            1,
	})
	require.NoError(t, err)
	require.NotNil(t, out)
	require.Len(t, out.Apis, 1)

	assert.Equal(t, "3", w.Header().Get("Total-Count"))
	assert.Equal(t, "3", w.Header().Get("Total-Pages"))
	assert.Equal(t, "1", w.Header().Get("Per-Page"))
	assert.Equal(t, "2", w.Header().Get("Current-Page"))
	assert.Contains(t, w.Header().Get("Link"), `rel="prev"`)
	assert.Contains(t, w.Header().Get("Link"), `rel="next"`)
	assert.Contains(t, w.Header().Get("Link"), `page=2`)
}

func TestStatisticsControllerGetApis_PropagatesErrorWithoutHeaders(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &statisticsAdoptionRepoStub{
		getApisFunc: func(ctx context.Context, params repositories.ApisQueryParams) ([]repositories.ApiRow, int, error) {
			return nil, 0, errors.New("query failed")
		},
	}
	ctrl := NewStatisticsController(services.NewAdoptionService(repo))

	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/v1/statistics/apis", nil)

	out, err := ctrl.GetApis(ctx, &models.AdoptionApisParams{
		AdoptionBaseParams: models.AdoptionBaseParams{AdrVersion: "ADR-2.0", StartDate: "2026-01-01", EndDate: "2026-01-31"},
	})

	require.Error(t, err)
	assert.Nil(t, out)
	assert.Contains(t, err.Error(), "query failed")
	assert.Empty(t, w.Header().Get("Total-Count"))
	assert.Empty(t, w.Header().Get("Link"))
}

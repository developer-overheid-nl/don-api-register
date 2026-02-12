package services

import (
	"context"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/developer-overheid-nl/don-api-register/pkg/api_client/models"
	"github.com/developer-overheid-nl/don-api-register/pkg/api_client/repositories"
)

type AdoptionService struct {
	repo repositories.AdoptionRepository
}

func NewAdoptionService(repo repositories.AdoptionRepository) *AdoptionService {
	return &AdoptionService{repo: repo}
}

func (s *AdoptionService) GetSummary(ctx context.Context, p *models.AdoptionBaseParams) (*models.AdoptionSummary, error) {
	startDate, endDate, err := parseDateRange(p.StartDate, p.EndDate)
	if err != nil {
		return nil, err
	}

	params := repositories.AdoptionQueryParams{
		AdrVersion:   p.AdrVersion,
		StartDate:    startDate,
		EndDate:      endDate,
		ApiIds:       splitCSV(p.ApiIds),
		Organisation: trimOptional(p.Organisation),
	}

	result, totalRuns, err := s.repo.GetSummary(ctx, params)
	if err != nil {
		return nil, err
	}

	return &models.AdoptionSummary{
		AdrVersion: p.AdrVersion,
		Period: models.Period{
			Start: p.StartDate,
			End:   p.EndDate,
		},
		TotalApis:           result.TotalApis,
		CompliantApis:       result.CompliantApis,
		OverallAdoptionRate: result.AdoptionRate,
		TotalLintRuns:       totalRuns,
	}, nil
}

func (s *AdoptionService) GetRules(ctx context.Context, p *models.AdoptionRulesParams) (*models.AdoptionRules, error) {
	_, endDate, err := parseDateRange(p.StartDate, p.EndDate)
	if err != nil {
		return nil, err
	}

	params := repositories.AdoptionQueryParams{
		AdrVersion:   p.AdrVersion,
		EndDate:      endDate,
		ApiIds:       splitCSV(p.ApiIds),
		Organisation: trimOptional(p.Organisation),
		RuleCodes:    splitCSV(p.RuleCodes),
		Severity:     trimOptional(p.Severity),
	}

	rows, totalApis, err := s.repo.GetRules(ctx, params)
	if err != nil {
		return nil, err
	}

	rules := make([]models.RuleAdoption, len(rows))
	for i, row := range rows {
		compliant := totalApis - row.ViolatingApis
		rules[i] = models.RuleAdoption{
			Code:          row.Code,
			Severity:      row.Severity,
			ViolatingApis: row.ViolatingApis,
			CompliantApis: compliant,
			AdoptionRate:  adoptionRate(compliant, totalApis),
		}
	}

	return &models.AdoptionRules{
		AdrVersion: p.AdrVersion,
		Period: models.Period{
			Start: p.StartDate,
			End:   p.EndDate,
		},
		TotalApis: totalApis,
		Rules:     rules,
	}, nil
}

func (s *AdoptionService) GetTimeline(ctx context.Context, p *models.AdoptionTimelineParams) (*models.AdoptionTimeline, error) {
	startDate, endDate, err := parseDateRange(p.StartDate, p.EndDate)
	if err != nil {
		return nil, err
	}

	granularity := p.Granularity
	if granularity == "" {
		granularity = "month"
	}
	if granularity != "day" && granularity != "week" && granularity != "month" {
		return nil, fmt.Errorf("invalid granularity: %s (must be day, week, or month)", granularity)
	}

	params := repositories.TimelineQueryParams{
		AdoptionQueryParams: repositories.AdoptionQueryParams{
			AdrVersion:   p.AdrVersion,
			StartDate:    startDate,
			EndDate:      endDate,
			ApiIds:       splitCSV(p.ApiIds),
			Organisation: trimOptional(p.Organisation),
			RuleCodes:    splitCSV(p.RuleCodes),
		},
		Granularity: granularity,
	}

	rows, err := s.repo.GetTimeline(ctx, params)
	if err != nil {
		return nil, err
	}

	// Group rows by rule code into series
	seriesMap := make(map[string]*models.TimelineSeries)
	var seriesOrder []string
	for _, row := range rows {
		series, ok := seriesMap[row.RuleCode]
		if !ok {
			series = &models.TimelineSeries{
				Type:     "rule",
				RuleCode: row.RuleCode,
			}
			seriesMap[row.RuleCode] = series
			seriesOrder = append(seriesOrder, row.RuleCode)
		}
		series.DataPoints = append(series.DataPoints, models.TimelinePoint{
			Period:        row.Period,
			TotalApis:     row.TotalApis,
			CompliantApis: row.CompliantApis,
			AdoptionRate:  adoptionRate(row.CompliantApis, row.TotalApis),
		})
	}

	series := make([]models.TimelineSeries, 0, len(seriesOrder))
	for _, code := range seriesOrder {
		series = append(series, *seriesMap[code])
	}

	return &models.AdoptionTimeline{
		AdrVersion:  p.AdrVersion,
		Granularity: granularity,
		Series:      series,
	}, nil
}

func (s *AdoptionService) GetApis(ctx context.Context, p *models.AdoptionApisParams) (*models.AdoptionApis, *models.Pagination, error) {
	_, endDate, err := parseDateRange(p.StartDate, p.EndDate)
	if err != nil {
		return nil, nil, err
	}

	if p.Page < 1 {
		p.Page = 1
	}
	if p.PerPage < 1 {
		p.PerPage = 20
	}
	if p.PerPage > 100 {
		p.PerPage = 100
	}

	params := repositories.ApisQueryParams{
		AdoptionQueryParams: repositories.AdoptionQueryParams{
			AdrVersion:   p.AdrVersion,
			EndDate:      endDate,
			ApiIds:       splitCSV(p.ApiIds),
			Organisation: trimOptional(p.Organisation),
			RuleCodes:    splitCSV(p.RuleCodes),
		},
		Compliant: p.Compliant,
		Page:      p.Page,
		PerPage:   p.PerPage,
	}

	rows, totalCount, err := s.repo.GetApis(ctx, params)
	if err != nil {
		return nil, nil, err
	}

	apis := make([]models.ApiAdoption, len(rows))
	for i, row := range rows {
		apis[i] = models.ApiAdoption{
			ApiId:           row.ApiId,
			ApiTitle:        row.ApiTitle,
			Organisation:    row.Organisation,
			IsCompliant:     row.IsCompliant,
			TotalViolations: row.TotalFailures,
			TotalWarnings:   row.TotalWarnings,
			ViolatedRules:   []string(row.ViolatedRules),
			LastLintDate:    row.LastLintDate,
		}
	}

	totalPages := 0
	if totalCount > 0 {
		totalPages = int(math.Ceil(float64(totalCount) / float64(p.PerPage)))
	}

	pagination := models.Pagination{
		CurrentPage:    p.Page,
		RecordsPerPage: p.PerPage,
		TotalPages:     totalPages,
		TotalRecords:   totalCount,
	}
	if p.Page < totalPages {
		next := p.Page + 1
		pagination.Next = &next
	}
	if p.Page > 1 {
		prev := p.Page - 1
		pagination.Previous = &prev
	}

	return &models.AdoptionApis{
		AdrVersion: p.AdrVersion,
		Period: models.Period{
			Start: p.StartDate,
			End:   p.EndDate,
		},
		Apis: apis,
	}, &pagination, nil
}

func parseDateRange(startStr, endStr string) (time.Time, time.Time, error) {
	start, err := time.Parse("2006-01-02", startStr)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("invalid startDate: %w", err)
	}
	end, err := time.Parse("2006-01-02", endStr)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("invalid endDate: %w", err)
	}
	// Set end to end of day
	end = end.Add(23*time.Hour + 59*time.Minute + 59*time.Second)
	return start, end, nil
}

func splitCSV(s *string) []string {
	if s == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*s)
	if trimmed == "" {
		return nil
	}
	parts := strings.Split(trimmed, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		t := strings.TrimSpace(p)
		if t != "" {
			result = append(result, t)
		}
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

func trimOptional(s *string) *string {
	if s == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*s)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

func adoptionRate(compliant, total int) float64 {
	if total == 0 {
		return 0
	}
	return math.Round(float64(compliant)/float64(total)*1000) / 10
}

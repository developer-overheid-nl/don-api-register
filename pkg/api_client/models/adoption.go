package models

import "time"

type Period struct {
	Start string `json:"start"`
	End   string `json:"end"`
}

// Summary endpoint

type AdoptionSummary struct {
	AdrVersion          string  `json:"adrVersion"`
	Period              Period  `json:"period"`
	TotalApis           int     `json:"totalApis"`
	CompliantApis       int     `json:"compliantApis"`
	OverallAdoptionRate float64 `json:"overallAdoptionRate"`
	TotalLintRuns       int     `json:"totalLintRuns"`
}

// Rules endpoint

type AdoptionRules struct {
	AdrVersion string         `json:"adrVersion"`
	Period     Period         `json:"period"`
	TotalApis  int            `json:"totalApis"`
	Rules      []RuleAdoption `json:"rules"`
}

type RuleAdoption struct {
	Code          string  `json:"code"`
	Severity      string  `json:"severity"`
	ViolatingApis int     `json:"violatingApis"`
	CompliantApis int     `json:"compliantApis"`
	AdoptionRate  float64 `json:"adoptionRate"`
}

// Timeline endpoint

type AdoptionTimeline struct {
	AdrVersion  string           `json:"adrVersion"`
	Granularity string           `json:"granularity"`
	Series      []TimelineSeries `json:"series"`
}

type TimelineSeries struct {
	Type       string          `json:"type"`
	RuleCode   string          `json:"ruleCode,omitempty"`
	DataPoints []TimelinePoint `json:"dataPoints"`
}

type TimelinePoint struct {
	Period        string  `json:"period"`
	TotalApis     int     `json:"totalApis"`
	CompliantApis int     `json:"compliantApis"`
	AdoptionRate  float64 `json:"adoptionRate"`
}

// APIs endpoint

type AdoptionApis struct {
	AdrVersion string        `json:"adrVersion"`
	Period     Period        `json:"period"`
	Apis       []ApiAdoption `json:"apis"`
}

type ApiAdoption struct {
	ApiId           string    `json:"apiId"`
	ApiTitle        string    `json:"apiTitle"`
	Organisation    string    `json:"organisation"`
	IsCompliant     bool      `json:"isCompliant"`
	TotalViolations int       `json:"totalViolations"`
	TotalWarnings   int       `json:"totalWarnings"`
	ViolatedRules   []string  `json:"violatedRules"`
	LastLintDate    time.Time `json:"lastLintDate"`
}

// Query parameter structs

type AdoptionBaseParams struct {
	AdrVersion   string  `query:"adrVersion" binding:"required"`
	StartDate    string  `query:"startDate" binding:"required"`
	EndDate      string  `query:"endDate" binding:"required"`
	ApiIds       *string `query:"apiIds"`
	Organisation *string `query:"organisation"`
}

type AdoptionRulesParams struct {
	AdoptionBaseParams
	RuleCodes *string `query:"ruleCodes"`
	Severity  *string `query:"severity"`
}

type AdoptionTimelineParams struct {
	AdoptionBaseParams
	Granularity string  `query:"granularity"`
	RuleCodes   *string `query:"ruleCodes"`
}

type AdoptionApisParams struct {
	AdoptionBaseParams
	Compliant *bool   `query:"compliant"`
	RuleCodes *string `query:"ruleCodes"`
	Page      int     `query:"page"`
	PerPage   int     `query:"perPage"`
}

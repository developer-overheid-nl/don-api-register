package models

import "strings"

const (
	DefaultSearchLimit = 10
	MaxSearchLimit     = 50
)

type SearchApisParams struct {
	Query string `query:"q" binding:"required"`
	Limit int    `query:"limit"`
}

func (p *SearchApisParams) EffectiveLimit() int {
	if p == nil {
		return DefaultSearchLimit
	}
	if p.Limit <= 0 {
		return DefaultSearchLimit
	}
	if p.Limit > MaxSearchLimit {
		return MaxSearchLimit
	}
	return p.Limit
}

func (p *SearchApisParams) NormalizedQuery() string {
	if p == nil {
		return ""
	}
	return strings.TrimSpace(p.Query)
}

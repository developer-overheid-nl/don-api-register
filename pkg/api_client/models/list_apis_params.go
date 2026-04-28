package models

import "strings"

type ListApisParams struct {
	Page         int      `query:"page"`
	PerPage      int      `query:"perPage"`
	Organisation *string  `query:"organisation"`
	Ids          *string  `query:"ids"`
	Status       []string `query:"status"`
	OasVersion   []string `query:"oasVersion"`
	Version      []string `query:"version"`
	AdrScore     []string `query:"adrScore"`
	Auth         []string `query:"auth"`
	BaseURL      string
}

// FilterIDs returns the sanitized ID list from the `ids` query parameter.
func (p *ListApisParams) FilterIDs() *string {
	if p == nil {
		return nil
	}
	return trimPointer(p.Ids)
}

func (p *ListApisParams) ApiFilters() *ApiFiltersParams {
	if p == nil {
		return &ApiFiltersParams{}
	}
	return &ApiFiltersParams{
		Organisation: p.Organisation,
		Ids:          p.FilterIDs(),
		Status:       append([]string(nil), p.Status...),
		OasVersion:   append([]string(nil), p.OasVersion...),
		Version:      append([]string(nil), p.Version...),
		AdrScore:     append([]string(nil), p.AdrScore...),
		Auth:         append([]string(nil), p.Auth...),
	}
}

func trimPointer(val *string) *string {
	if val == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*val)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

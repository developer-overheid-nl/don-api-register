package models

import "strings"

type ListApisParams struct {
	Page         int     `query:"page"`
	PerPage      int     `query:"perPage"`
	Organisation *string `query:"organisation"`
	Ids          *string `query:"ids"`
	BaseURL      string
}

// FilterIDs returns the preferred ID list from either the legacy `ids` query or the new `apis` alias.
func (p *ListApisParams) FilterIDs() *string {
	if p == nil {
		return nil
	}
	return trimPointer(p.Ids)
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

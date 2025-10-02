package models

import "strings"

type ListApisParams struct {
	Page         int     `query:"page"`
	PerPage      int     `query:"perPage"`
	Organisation *string `query:"organisation"`
	Ids          *string `query:"ids"`
	Apis         *string `query:"apis"`
	BaseURL      string  // not from query, set in handler
}

// FilterIDs returns the preferred ID list from either the legacy `ids` query or the new `apis` alias.
func (p *ListApisParams) FilterIDs() *string {
	if p == nil {
		return nil
	}
	if trimmed := trimPointer(p.Apis); trimmed != nil {
		return trimmed
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

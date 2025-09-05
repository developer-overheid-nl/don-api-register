package models

type ListApisParams struct {
	Page         int     `query:"page"`
	PerPage      int     `query:"perPage"`
	Organisation *string `query:"organisation"`
	Ids          *string `query:"ids"`
	BaseURL      string  // not from query, set in handler
}

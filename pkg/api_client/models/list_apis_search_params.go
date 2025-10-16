package models

type ListApisSearchParams struct {
	Page         int     `query:"page"`
	PerPage      int     `query:"perPage"`
	Organisation *string `query:"organisation"`
	Query        string  `query:"q" binding:"required"`
	BaseURL      string
}

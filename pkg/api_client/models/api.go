/*
 * API register API v1
 *
 * API van het API register (apis.developer.overheid.nl)
 *
 * API version: 1.0.0
 * Contact: developer.overheid@geonovum.nl
 */

package models

type Api struct {
	Id            string           `gorm:"primaryKey"`
	Type          string           `json:"type,omitempty"`
	OasUri        string           `json:"oasUri,omitempty"`
	DocsUri       string           `json:"docsUri,omitempty"`
	Title         string           `json:"title,omitempty"`
	Description   string           `json:"description,omitempty"`
	Auth          string           `json:"auth,omitempty"`
	AdrScore      *string          `json:"adrScore,omitempty"`
	RepositoryUri *string          `json:"repositoryUri,omitempty"`
	Organisation  *ApiOrganisation `json:"organisation" gorm:"embedded"`
}

type ApiOrganisation struct {
	Label string `json:"label,omitempty"`
	Uri   string `json:"uri,omitempty"`
}

type PaginatedResponse struct {
	Pagination Pagination `json:"pagination"`
	Results    []Api      `json:"results"`
}

type Pagination struct {
	Next           *int `json:"next,omitempty"`
	Previous       *int `json:"previous,omitempty"`
	CurrentPage    int  `json:"currentPage"`
	RecordsPerPage int  `json:"recordsPerPage"`
	TotalPages     int  `json:"totalPages"`
	TotalRecords   int  `json:"totalRecords"`
}

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
	Id             string   `gorm:"primaryKey"`
	OasUri         string   `json:"oasUri,omitempty"`
	DocsUri        string   `json:"docsUri,omitempty"`
	Title          string   `json:"title,omitempty"`
	Description    string   `json:"description,omitempty"`
	Auth           string   `json:"auth,omitempty"` //Niet verplicht
	AdrScore       string   `json:"adrScore,omitempty"`
	RepositoryUri  string   `json:"repositoryUri,omitempty"`
	ContactName    string   `json:"contact_name,omitempty"`
	ContactUrl     string   `json:"contact_url,omitempty"`
	ContactEmail   string   `json:"contact_email,omitempty"`
	OrganisationId string   `json:"organisationId,omitempty"` //Niet verplicht
	Servers        []Server `gorm:"many2many:api_servers;" json:"servers,omitempty"`
}

type ApiOrganisation struct {
	Id    string `gorm:"primaryKey"`
	Label string `json:"label,omitempty"`
	Uri   string `json:"uri,omitempty"`
}

type Server struct {
	Id          string `gorm:"primaryKey"`
	Description string `json:"description,omitempty"`
	Uri         string `json:"uri,omitempty"`
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

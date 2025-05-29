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
	Id             string        `gorm:"primaryKey"`
	OasUri         string        `json:"oasUri,omitempty"`
	DocsUri        string        `json:"docsUri,omitempty"`
	Title          string        `json:"title,omitempty"`
	Description    string        `json:"description,omitempty"`
	Auth           string        `json:"auth,omitempty"`
	AdrScore       string        `json:"adrScore,omitempty"`
	RepositoryUri  string        `json:"repositoryUri,omitempty"`
	ContactName    string        `json:"contact_name,omitempty"`
	ContactUrl     string        `json:"contact_url,omitempty"`
	ContactEmail   string        `json:"contact_email,omitempty"`
	Organisation   *Organisation `json:"organisation,omitempty" gorm:"foreignKey:OrganisationID"`
	OrganisationID string        `json:"organisationId,omitempty"`
	Servers        []Server      `gorm:"many2many:api_servers;" json:"servers,omitempty"`
}

type Organisation struct {
	Id    string `gorm:"primarykey"`
	Label string `json:"label,omitempty"`
	Uri   string `json:"uri,omitempty"` //dit moet primairy worden.
}

type Server struct {
	Id          string `gorm:"primaryKey"`
	Description string `json:"description,omitempty"`
	Uri         string `json:"uri,omitempty"`
}

// Link representeert een hypermedia‐link
type Link struct {
	Href string `json:"href"`
}

// Links bevat self/next/prev links volgens HAL-stijl
type Links struct {
	Self *Link `json:"self"`
	Next *Link `json:"next,omitempty"`
	Prev *Link `json:"prev,omitempty"`
}

// Contact bundelt de contactgegevens
type Contact struct {
	Name  string `json:"name"`
	URL   string `json:"url,omitempty"`
	Email string `json:"email,omitempty"`
}

// ApiResponse is de externe view van een API
type ApiResponse struct {
	Title   string  `json:"title"`
	OasUri  string  `json:"oasUri"`
	Contact Contact `json:"contact"`
}

// ApiListResponse is het nieuwe root-object
type ApiListResponse struct {
	Links Links         `json:"_links"`
	Apis  []ApiResponse `json:"apis"`
}

type Pagination struct {
	Next           *int `json:"next,omitempty"`
	Previous       *int `json:"previous,omitempty"`
	CurrentPage    int  `json:"currentPage"`
	RecordsPerPage int  `json:"recordsPerPage"`
	TotalPages     int  `json:"totalPages"`
	TotalRecords   int  `json:"totalRecords"`
}

type OasParams struct {
	OasUrl string `json:"oasUrl" binding:"required,url"`
}

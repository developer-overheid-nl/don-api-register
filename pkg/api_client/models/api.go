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
	Id             string        `gorm:"column:id;primaryKey"`
	OasUri         string        `json:"oasUrl,omitempty"`
	OasHash        string        `json:"-" gorm:"column:oas_hash"`
	DocsUri        string        `json:"docsUri,omitempty"`
	Title          string        `json:"title,omitempty"`
	Description    string        `json:"description,omitempty"`
	Auth           string        `json:"auth,omitempty"`
	AdrScore       *int          `gorm:"column:adr_score" json:"adrScore,omitempty"`
	RepositoryUri  string        `json:"repositoryUri,omitempty"`
	ContactName    string        `json:"contact_name,omitempty"`
	ContactUrl     string        `json:"contact_url,omitempty"`
	ContactEmail   string        `json:"contact_email,omitempty"`
	Organisation   *Organisation `json:"organisation,omitempty" gorm:"foreignKey:OrganisationID;references:Uri"`
	OrganisationID *string       `json:"organisationId,omitempty" gorm:"column:organisation_id"`
	Servers        []Server      `gorm:"many2many:api_servers;" json:"servers,omitempty"`
}

type Organisation struct {
	Uri   string `gorm:"column:uri;primaryKey" json:"uri"`
	Label string `gorm:"column:label" json:"label"`
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
type Links struct {
	First *Link `json:"first,omitempty"`
	Prev  *Link `json:"prev,omitempty"`
	Self  *Link `json:"self"`
	Next  *Link `json:"next,omitempty"`
	Last  *Link `json:"last,omitempty"`
}

// Contact bundelt de contactgegevens
type Contact struct {
	Name  string `json:"name"`
	URL   string `json:"url,omitempty"`
	Email string `json:"email,omitempty"`
}

// ApiResponse is de externe view van een API
type ApiResponse struct {
	Id      string  `json:"id"`
	Title   string  `json:"title"`
	OasUri  string  `json:"oasUri"`
	Contact Contact `json:"contact"`
}

// ApiListResponse is het nieuwe root-object
type ApiListResponse struct {
	Apis  []ApiSummary `json:"apis"`
	Links Links        `json:"_links"`
}

type Pagination struct {
	Next           *int `json:"next,omitempty"`
	Previous       *int `json:"previous,omitempty"`
	CurrentPage    int  `json:"currentPage"`
	RecordsPerPage int  `json:"recordsPerPage"`
	TotalPages     int  `json:"totalPages"`
	TotalRecords   int  `json:"totalRecords"`
}
type ApiSummary struct {
	Id           string       `json:"id"`
	OasUrl       string       `json:"oasUrl"`
	Title        string       `json:"title"`
	Description  string       `json:"description,omitempty"`
	Contact      Contact      `json:"contact"`
	Organisation Organisation `json:"organisation"`
	AdrScore     *int         `json:"adrScore,omitempty"`
	Links        *Links       `json:"_links,omitempty"`
}

type ApiDetail struct {
	ApiSummary          // embed alles van ApiSummary
	Auth       []string `json:"auth,omitempty"`
	DocsUri    string   `json:"docsUri,omitempty"`
	Servers    []Server `json:"servers,omitempty"`
}

type ApiPost struct {
	Id              string  `json:"id,omitempty"`
	OasUrl          string  `json:"oasUrl" binding:"required,url"`
	OrganisationUri string  `json:"organisationUri" binding:"required,url"`
	Contact         Contact `json:"contact"`
}

type ApiParams struct {
	Id string `path:"id"`
}

type UpdateApiInput struct {
	Id              string  `path:"id"` // <-- uit path param
	OasUrl          string  `json:"oasUrl" binding:"required,url"`
	OrganisationUri string  `json:"organisationUri" binding:"required,url"`
	Contact         Contact `json:"contact"`
}

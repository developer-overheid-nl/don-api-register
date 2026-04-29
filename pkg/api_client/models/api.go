/*
 * API register API v1
 *
 * API van het API register (apis.developer.overheid.nl)
 *
 * API version: 1.0.0
 * Contact: developer.overheid@geonovum.nl
 */

package models

import (
	"bytes"
	"encoding/json"
	"time"
)

type Api struct {
	Id             string        `gorm:"column:id;primaryKey"`
	OasUri         string        `json:"oasUrl,omitempty"`
	OasHash        string        `json:"-" gorm:"column:oas_hash"`
	OAS            OASMetadata   `gorm:"embedded;embeddedPrefix:oas_" json:"-"`
	DocsUrl        string        `json:"docsUrl,omitempty"`
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
	Version        string        `json:"version,omitempty"`
	Sunset         string        `json:"sunset,omitempty"`
	Deprecated     string        `json:"deprecated,omitempty"`
}

type OASMetadata struct {
	Version string `json:"version,omitempty"`
	Status  string `json:"status,omitempty"`
	Auth    string `json:"auth,omitempty"`
}

const (
	OASStatusUnknown     = "unknown"
	OASStatusValid       = "valid"
	OASStatusInvalid     = "invalid"
	OASStatusUnreachable = "unreachable"
)

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
	Self  *Link `json:"self,omitempty"`
	Next  *Link `json:"next,omitempty"`
	Last  *Link `json:"last,omitempty"`
	Apis  *Link `json:"apis,omitempty"` // link naar de lijst van APIs
}

// Contact bundelt de contactgegevens
type Contact struct {
	Name  string `json:"name"`
	URL   string `json:"url,omitempty"`
	Email string `json:"email,omitempty"`
}

type OptionalString struct {
	Set   bool
	Value *string
}

func NewOptionalString(value string) OptionalString {
	return OptionalString{
		Set:   true,
		Value: &value,
	}
}

func NewNullString() OptionalString {
	return OptionalString{Set: true}
}

func (s *OptionalString) UnmarshalJSON(data []byte) error {
	s.Set = true
	if bytes.Equal(bytes.TrimSpace(data), []byte("null")) {
		s.Value = nil
		return nil
	}

	var value string
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	s.Value = &value
	return nil
}

type Lifecycle struct {
	Status     string `json:"status"`
	Version    string `json:"version"`
	Sunset     string `json:"sunset,omitempty"`
	Deprecated string `json:"deprecated,omitempty"`
}

func (api Api) LifecycleStatus(now time.Time) string {
	switch {
	case api.Sunset != "" && parseLifecycleDate(api.Sunset).After(now):
		return "sunset"
	case api.Sunset != "" && parseLifecycleDate(api.Sunset).Before(now):
		return "retired"
	case api.Deprecated != "" && parseLifecycleDate(api.Deprecated).Before(now):
		return "deprecated"
	default:
		return "active"
	}
}

func parseLifecycleDate(value string) time.Time {
	t, err := time.Parse(time.DateOnly, value)
	if err != nil {
		return time.Time{}
	}
	return t
}

// ApiResponse is de externe view van een API
type ApiResponse struct {
	Id        string    `json:"id"`
	Title     string    `json:"title"`
	OasUri    string    `json:"oasUri"`
	Contact   Contact   `json:"contact"`
	Lifecycle Lifecycle `json:"lifecycle"`
}

type Pagination struct {
	Next           *int
	Previous       *int
	CurrentPage    int
	RecordsPerPage int
	TotalPages     int
	TotalRecords   int
}
type ApiSummary struct {
	Id           string              `json:"id"`
	OasUrl       string              `json:"oasUrl"`
	Title        string              `json:"title"`
	Description  string              `json:"description,omitempty"`
	Contact      Contact             `json:"contact"`
	Organisation OrganisationSummary `json:"organisation"`
	AdrScore     *int                `json:"adrScore"`
	Links        *Links              `json:"_links,omitempty"`
	Lifecycle    Lifecycle           `json:"lifecycle"`
}

type ServerInfo struct {
	Url         string `json:"url,omitempty"`
	Description string `json:"description,omitempty"`
}

type ApiDetail struct {
	ApiSummary               // embed alles van ApiSummary
	Auth        []string     `json:"auth,omitempty"`
	DocsUrl     string       `json:"docsUrl,omitempty"`
	Servers     []ServerInfo `json:"servers,omitempty"`
	LintResults []LintResult `json:"lintResults,omitempty"`
	OasVersion  string       `json:"-"`
}

type ContactJsonLd struct {
	FN       string `json:"vcard:fn"`
	HasEmail string `json:"vcard:hasEmail,omitempty"`
	HasURL   string `json:"vcard:hasURL,omitempty"`
}

type ApiDetailJsonLd struct {
	Context             json.RawMessage `json:"@context"`
	Type                string          `json:"@type"`
	ConformsTo          []string        `json:"dct:conformsTo"`
	Identifier          string          `json:"dct:identifier"`
	Title               string          `json:"dct:title"`
	Description         string          `json:"dct:description,omitempty"`
	EndpointDescription string          `json:"dcat:endpointDescription,omitempty"`
	ContactPoint        ContactJsonLd   `json:"dcat:contactPoint"`
	Publisher           string          `json:"dct:publisher,omitempty"`
}

type ApiPost struct {
	Id              string  `json:"id,omitempty"`
	OasUrl          string  `json:"oasUrl" binding:"required_without=OasBody,omitempty,url"`
	OasBody         string  `json:"oasBody,omitempty" binding:"required_without=OasUrl"`
	ArazzoUrl       string  `json:"arazzoUrl,omitempty"`
	ArazzoBody      string  `json:"arazzoBody,omitempty"`
	OrganisationUri string  `json:"organisationUri" binding:"required,url"`
	Contact         Contact `json:"contact"`
}

type ApiParams struct {
	Id string `path:"id"`
}

type ApiOasParams struct {
	Id      string `path:"id"`
	Version string `path:"version"`
}

type UpdateApiInput struct {
	Id              string         `path:"id"` // <-- uit path param
	OasUrl          string         `json:"oasUrl" binding:"required_without_all=OasBody Sunset Deprecated,omitempty,url"`
	OasBody         string         `json:"oasBody,omitempty" binding:"required_without_all=OasUrl Sunset Deprecated"`
	ArazzoUrl       string         `json:"arazzoUrl,omitempty"`
	ArazzoBody      string         `json:"arazzoBody,omitempty"`
	OrganisationUri string         `json:"organisationUri" binding:"required,url"`
	Contact         Contact        `json:"contact"`
	Sunset          OptionalString `json:"sunset,omitempty"`
	Deprecated      OptionalString `json:"deprecated,omitempty"`
}

type OrganisationSummary struct {
	Uri   string `json:"uri"`
	Label string `json:"label"`
	Links *Links `json:"_links,omitempty"`
}

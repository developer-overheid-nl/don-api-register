package openapi

import (
	"net/http"
	"strings"

	"github.com/developer-overheid-nl/don-api-register/pkg/api_client/helpers/problem"
	"github.com/developer-overheid-nl/don-api-register/pkg/api_client/models"
	"github.com/google/uuid"
	v3 "github.com/pb33f/libopenapi/datamodel/high/v3"
	"github.com/pb33f/libopenapi/orderedmap"
	"github.com/teris-io/shortid"
	"go.yaml.in/yaml/v4"
)

// CorsGet performs a GET request including an Origin header.
func CorsGet(c *http.Client, u string, corsurl string) (*http.Response, error) {
	req, err := http.NewRequest(http.MethodGet, u, http.NoBody)
	if err != nil {
		return nil, err
	}
	req.Header.Add("Origin", corsurl)
	return c.Do(req)
}

// DeriveAuthType determines authentication type from security schemes (pb33f v3).
func DeriveAuthType(spec *v3.Document) string {
	if spec == nil || spec.Components == nil || spec.Components.SecuritySchemes == nil {
		return "unknown"
	}
	for pair := spec.Components.SecuritySchemes.First(); pair != nil; pair = pair.Next() {
		scheme := pair.Value()
		if scheme == nil {
			continue
		}
		typ := strings.ToLower(scheme.Type)
		switch typ {
		case "apikey", "api_key", "api-key":
			return "api_key"
		case "http":
			if scheme.Scheme != "" {
				return strings.ToLower(scheme.Scheme)
			}
			return "http"
		case "oauth2":
			return "oauth2"
		case "openidconnect", "openid", "openIdConnect", "openid-connect":
			return "openid"
		}
	}
	return "unknown"
}

// extString safely extracts a string extension (e.g. x-sunset) from pb33f YAML node map.
func extString(m *orderedmap.Map[string, *yaml.Node], key string) string {
	if m == nil {
		return ""
	}
	if n, ok := m.Get(key); ok && n != nil {
		// alleen simpele scalar strings teruggeven
		if n.Kind == yaml.ScalarNode && (n.Tag == "" || n.Tag == "!!str") {
			return n.Value
		}
	}
	return ""
}

func populateApiFromSpec(api *models.Api, spec *v3.Document, requestBody models.ApiPost, label string) {
	if api == nil {
		return
	}

	if spec != nil && spec.Info != nil {
		api.Title = spec.Info.Title
		api.Description = spec.Info.Description

		api.ContactName = ""
		api.ContactEmail = ""
		api.ContactUrl = ""
		if spec.Info.Contact != nil {
			api.ContactName = spec.Info.Contact.Name
			api.ContactEmail = spec.Info.Contact.Email
			api.ContactUrl = spec.Info.Contact.URL
		}
		if spec.Info.Version != "" {
			api.Version = spec.Info.Version
		} else {
			api.Version = ""
		}
		api.Sunset = extString(spec.Info.Extensions, "x-sunset")
		api.Deprecated = extString(spec.Info.Extensions, "x-deprecated")
	} else {
		api.Title = ""
		api.Description = ""
		api.ContactName = ""
		api.ContactEmail = ""
		api.ContactUrl = ""
		api.Version = ""
		api.Sunset = ""
		api.Deprecated = ""
	}

	api.OasUri = requestBody.OasUrl

	if strings.TrimSpace(requestBody.OrganisationUri) != "" {
		api.Organisation = &models.Organisation{
			Uri:   requestBody.OrganisationUri,
			Label: label,
		}
		api.OrganisationID = &requestBody.OrganisationUri
	} else {
		api.Organisation = nil
		api.OrganisationID = nil
	}

	if spec != nil && spec.ExternalDocs != nil {
		api.DocsUrl = spec.ExternalDocs.URL
	} else {
		api.DocsUrl = ""
	}

	if spec != nil {
		hasSecuritySchemes := false
		if spec.Components != nil && spec.Components.SecuritySchemes != nil {
			hasSecuritySchemes = orderedmap.Len(spec.Components.SecuritySchemes) > 0
		}
		if len(spec.Security) > 0 || hasSecuritySchemes {
			api.Auth = DeriveAuthType(spec)
		} else {
			api.Auth = ""
		}
	} else {
		api.Auth = ""
	}

	if spec != nil && len(spec.Servers) > 0 {
		serversToSave := make([]models.Server, 0, len(spec.Servers))
		for _, s := range spec.Servers {
			if s == nil {
				continue
			}
			if s.URL != "" {
				server := models.Server{
					Id:          uuid.New().String(),
					Uri:         s.URL,
					Description: s.Description,
				}
				serversToSave = append(serversToSave, server)
			}
		}
		api.Servers = serversToSave
	} else {
		api.Servers = []models.Server{}
	}

	if strings.TrimSpace(api.ContactName) == "" {
		api.ContactName = requestBody.Contact.Name
	}
	if strings.TrimSpace(api.ContactEmail) == "" {
		api.ContactEmail = requestBody.Contact.Email
	}
	if strings.TrimSpace(api.ContactUrl) == "" {
		api.ContactUrl = requestBody.Contact.URL
	}
}

// BuildApi constructs a models.Api based on the OpenAPI spec (pb33f v3) and request body.
func BuildApi(spec *v3.Document, requestBody models.ApiPost, label string) *models.Api {
	api := &models.Api{
		Id: shortid.MustGenerate(),
	}

	populateApiFromSpec(api, spec, requestBody, label)

	return api
}

// UpdateApiFromSpec mutates an existing models.Api with values derived from the OpenAPI spec.
func UpdateApiFromSpec(api *models.Api, spec *v3.Document, requestBody models.ApiPost, label string) {
	populateApiFromSpec(api, spec, requestBody, label)
}

// ValidateApi fills missing fields from the request body and collects missing errors.
func ValidateApi(api *models.Api) []problem.InvalidParam {
	var invalids []problem.InvalidParam

	if strings.TrimSpace(api.ContactName) == "" {
		invalids = append(invalids, problem.InvalidParam{
			Name:   "contact.name",
			Reason: "contact.name is verplicht",
		})
	}
	if strings.TrimSpace(api.ContactEmail) == "" {
		invalids = append(invalids, problem.InvalidParam{
			Name:   "contact.email",
			Reason: "contact.email is verplicht",
		})
	}
	if strings.TrimSpace(api.ContactUrl) == "" {
		invalids = append(invalids, problem.InvalidParam{
			Name:   "contact.url",
			Reason: "contact.url is verplicht",
		})
	}
	if strings.TrimSpace(api.OasUri) == "" {
		invalids = append(invalids, problem.InvalidParam{
			Name:   "oasUrl",
			Reason: "oasUrl is verplicht",
		})
	}
	if api.OrganisationID == nil || strings.TrimSpace(*api.OrganisationID) == "" {
		invalids = append(invalids, problem.InvalidParam{
			Name:   "organisationUri",
			Reason: "organisationUri is verplicht",
		})
	}
	return invalids
}

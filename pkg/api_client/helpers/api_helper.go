package helpers

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/google/uuid"

	"github.com/developer-overheid-nl/don-api-register/pkg/api_client/models"
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

// ComputeOASHash downloads an OAS document, parses it and returns a hash.
func ComputeOASHash(oasURL string) (string, error) {
	resp, err := http.Get(oasURL)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("OAS download failed with status %d", resp.StatusCode)
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	spec, err := ParseAndValidateOAS(data)
	if err != nil {
		return "", err
	}
	serialized, err := json.Marshal(spec)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(serialized)
	return hex.EncodeToString(sum[:]), nil
}

func DeriveAuthType(spec *openapi3.T) string {
	for _, schemeRef := range spec.Components.SecuritySchemes {
		scheme := schemeRef.Value
		if scheme == nil {
			continue
		}

		switch scheme.Type {
		case "apiKey":
			return "api_key"
		case "http":
			return scheme.Scheme
		case "oauth2":
			return "oauth2"
		case "openIdConnect":
			return "openid"
		}
	}
	return "unknown"
}

// BuildApi constructs a models.Api based on the OpenAPI spec and request body.
func BuildApi(spec *openapi3.T, requestBody models.Api) *models.Api {
	api := &models.Api{}
	api.Id = uuid.New().String()
	if spec.Info != nil {
		api.Title = spec.Info.Title
		api.Description = spec.Info.Description
		if spec.Info.Contact != nil {
			api.ContactName = spec.Info.Contact.Name
			api.ContactEmail = spec.Info.Contact.Email
			api.ContactUrl = spec.Info.Contact.URL
		}
	}

	api.OasUri = requestBody.OasUri
	if spec.ExternalDocs != nil {
		api.DocsUri = spec.ExternalDocs.URL
	}

	if len(spec.Security) > 0 {
		api.Auth = DeriveAuthType(spec)
	} else if spec.Components != nil && len(spec.Components.SecuritySchemes) > 0 {
		api.Auth = DeriveAuthType(spec)
	}

	var serversToSave []models.Server
	for _, s := range spec.Servers {
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
	api.OrganisationID = nil
	return api
}

// ValidateApi fills missing fields from the request body and collects missing errors.
func ValidateApi(api *models.Api, requestBody models.Api) []InvalidParam {
	var invalids []InvalidParam
	if api.ContactUrl == "" {
		if requestBody.ContactUrl != "" {
			api.ContactUrl = requestBody.ContactUrl
		} else {
			invalids = append(invalids, InvalidParam{
				Name:   "contact.url",
				Reason: "contact.url is verplicht",
			})
		}
	}
	if api.ContactName == "" {
		if requestBody.ContactName != "" {
			api.ContactName = requestBody.ContactName
		} else {
			invalids = append(invalids, InvalidParam{
				Name:   "contact.name",
				Reason: "contact.name is verplicht",
			})
		}
	}
	if api.ContactEmail == "" {
		if requestBody.ContactEmail != "" {
			api.ContactEmail = requestBody.ContactEmail
		} else {
			invalids = append(invalids, InvalidParam{
				Name:   "contact.email",
				Reason: "contact.email is verplicht",
			})
		}
	}
	return invalids
}

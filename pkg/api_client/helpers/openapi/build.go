package openapi

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/developer-overheid-nl/don-api-register/pkg/api_client/helpers/problem"
	"github.com/developer-overheid-nl/don-api-register/pkg/api_client/models"
	"github.com/getkin/kin-openapi/openapi3"
	"github.com/google/uuid"
	"github.com/teris-io/shortid"
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

// DeriveAuthType determines authentication type from security schemes.
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
func BuildApi(spec *openapi3.T, requestBody models.ApiPost, label string) *models.Api {
	api := &models.Api{
		Id: shortid.MustGenerate(),
	}

	if spec.Info != nil {
		api.Title = spec.Info.Title
		api.Description = spec.Info.Description
		if spec.Info.Contact != nil {
			api.ContactName = spec.Info.Contact.Name
			api.ContactEmail = spec.Info.Contact.Email
			api.ContactUrl = spec.Info.Contact.URL
		}
		if spec.Info.Version != "" {
			api.Version = spec.Info.Version
		}
		if v, ok := spec.Info.Extensions["x-sunset"].(string); ok && v != "" {
			api.Sunset = v
		}
		if v, ok := spec.Info.Extensions["x-deprecated"].(string); ok && v != "" {
			api.Deprecated = v
		}
	}

	api.OasUri = requestBody.OasUrl

	if requestBody.OrganisationUri != "" {
		api.Organisation = &models.Organisation{
			Uri:   requestBody.OrganisationUri,
			Label: label,
		}
		api.OrganisationID = &requestBody.OrganisationUri
	}
	if spec.ExternalDocs != nil {
		api.DocsUri = spec.ExternalDocs.URL
	}

	if len(spec.Security) > 0 || (spec.Components != nil && len(spec.Components.SecuritySchemes) > 0) {
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
	if api.ContactName == "" {
		api.ContactName = requestBody.Contact.Name
	}
	if api.ContactEmail == "" {
		api.ContactEmail = requestBody.Contact.Email
	}
	if api.ContactUrl == "" {
		api.ContactUrl = requestBody.Contact.URL
	}
	return api
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

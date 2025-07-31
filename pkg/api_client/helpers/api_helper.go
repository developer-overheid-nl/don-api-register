package helpers

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

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

var httpClient = &http.Client{}

// BuildApi constructs a models.Api based on the OpenAPI spec and request body.
func BuildApi(spec *openapi3.T, requestBody models.ApiPost, label string) *models.Api {
	api := &models.Api{
		Id: uuid.New().String(),
	}

	if spec.Info != nil {
		api.Title = spec.Info.Title
		api.Description = spec.Info.Description
		if spec.Info.Contact != nil {
			api.ContactName = spec.Info.Contact.Name
			api.ContactEmail = spec.Info.Contact.Email
			api.ContactUrl = spec.Info.Contact.URL
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
func ValidateApi(api *models.Api) []InvalidParam {
	var invalids []InvalidParam

	if strings.TrimSpace(api.ContactName) == "" {
		invalids = append(invalids, InvalidParam{
			Name:   "contact.name",
			Reason: "contact.name is verplicht",
		})
	}
	if strings.TrimSpace(api.ContactEmail) == "" {
		invalids = append(invalids, InvalidParam{
			Name:   "contact.email",
			Reason: "contact.email is verplicht",
		})
	}
	if strings.TrimSpace(api.ContactUrl) == "" {
		invalids = append(invalids, InvalidParam{
			Name:   "contact.url",
			Reason: "contact.url is verplicht",
		})
	}
	if strings.TrimSpace(api.OasUri) == "" {
		invalids = append(invalids, InvalidParam{
			Name:   "oasUrl",
			Reason: "oasUrl is verplicht",
		})
	}
	if api.OrganisationID == nil || strings.TrimSpace(*api.OrganisationID) == "" {
		invalids = append(invalids, InvalidParam{
			Name:   "organisationUri",
			Reason: "organisationUri is verplicht",
		})
	}
	return invalids
}

type TooIGraph struct {
	Graph []TooIObject `json:"@graph"`
}

type TooIObject struct {
	ID    string `json:"@id"`
	Label []struct {
		Value    string `json:"@value"`
		Language string `json:"@language"`
	} `json:"http://www.w3.org/2000/01/rdf-schema#label"`
}

func FetchOrganisationLabel(ctx context.Context, uriOrType string, optionalId ...string) (string, error) {
	var uri string

	// 1. Check of uriOrType al een volledige URI is
	if strings.HasPrefix(uriOrType, "https://identifier.overheid.nl/tooi/id/") {
		uri = uriOrType
	} else if len(optionalId) > 0 {
		// 2. Combineer type en id
		uri = fmt.Sprintf("https://identifier.overheid.nl/tooi/id/%s/%s", uriOrType, optionalId[0])
	} else {
		return "", fmt.Errorf("ongeldig argument, geef een volledige URI of (type, id)")
	}

	// 3. Vraag JSON-LD op via content negotiation
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, uri, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/ld+json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("organisation not found: %s", uri)
	}

	var arr []TooIGraph
	if err := json.NewDecoder(resp.Body).Decode(&arr); err != nil {
		return "", fmt.Errorf("decode error: %w", err)
	}
	if len(arr) == 0 || len(arr[0].Graph) == 0 {
		return "", fmt.Errorf("geen organisatie gevonden in TOOI")
	}
	for _, obj := range arr[0].Graph {
		if obj.ID == uri {
			for _, lbl := range obj.Label {
				if lbl.Language == "nl" {
					return lbl.Value, nil
				}
			}
			if len(obj.Label) > 0 {
				return obj.Label[0].Value, nil
			}
			return "", fmt.Errorf("geen label gevonden voor %s", uri)
		}
	}
	return "", fmt.Errorf("organisatie %s niet gevonden in response", uri)
}

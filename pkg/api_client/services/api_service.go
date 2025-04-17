package services

import (
	"context"
	"fmt"
	"github.com/developer-overheid-nl/don-api-register/pkg/api_client/models"
	"github.com/developer-overheid-nl/don-api-register/pkg/api_client/repositories"
	"github.com/getkin/kin-openapi/openapi3"
	"github.com/google/uuid"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
)

// APIsAPIService implementeert APIsAPIServicer met de benodigde repository
type APIsAPIService struct {
	repo repositories.ApiRepository
}

// NewAPIsAPIService Constructor-functie
func NewAPIsAPIService(repo repositories.ApiRepository) *APIsAPIService {
	return &APIsAPIService{repo: repo}
}

func (s *APIsAPIService) RetrieveApi(ctx context.Context, id string) (*models.Api, error) {
	api, err := s.repo.GetApiByID(ctx, id)
	if err != nil || api == nil {
		return nil, err
	}
	return api, nil
}

func (s *APIsAPIService) ListApis(ctx context.Context, page, perPage int) (models.PaginatedResponse, error) {
	apis, pagination, err := s.repo.GetApis(ctx, page, perPage)
	if err != nil {
		return models.PaginatedResponse{}, err
	}

	return models.PaginatedResponse{
		Pagination: pagination,
		Results:    apis,
	}, nil
}

func (s *APIsAPIService) CreateApiFromOas(ctx context.Context, requestBody models.Api) (*models.Api, error) {
	parsedUrl, err := url.Parse(requestBody.OasUri)
	if err != nil {
		return nil, fmt.Errorf("ongeldige URL: %w", err)
	}

	client := &http.Client{}
	resp, err := CorsGet(client, parsedUrl.String(), "https://developer.overheid.nl")
	if err != nil {
		return nil, fmt.Errorf("fout bij ophalen OAS: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("OAS download faalt met status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("kan response body niet lezen: %w", err)
	}

	loader := openapi3.NewLoader()
	spec, err := loader.LoadFromData(body)
	if err != nil {
		log.Printf("[ERROR] Ongeldige OpenAPI: %v", err)
		return nil, fmt.Errorf("ongeldig OpenAPI-bestand: %w", err)
	}

	api, missing := s.BuildApiAndValidate(spec, requestBody)

	if len(missing) > 0 {
		return nil, fmt.Errorf("De volgende gegevens ontbreken: %s", strings.Join(missing, ", "))
	}

	if err := s.repo.Save(api); err != nil {
		return nil, fmt.Errorf("kan API niet opslaan: %w", err)
	}

	return api, nil
}

func (s *APIsAPIService) UpdateApi(ctx context.Context, api models.Api) error {
	return s.repo.UpdateApi(ctx, api)
}

func CorsGet(c *http.Client, u string, corsurl string) (*http.Response, error) {
	req, err := http.NewRequest(http.MethodGet, u, http.NoBody)
	if err != nil {
		return nil, err
	}
	req.Header.Add("Origin", corsurl)
	return c.Do(req)
}

func (s *APIsAPIService) BuildApiAndValidate(spec *openapi3.T, requestBody models.Api) (*models.Api, []string) {
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
		api.Auth = deriveAuthType(spec)
	} else if spec.Components != nil && len(spec.Components.SecuritySchemes) > 0 {
		api.Auth = deriveAuthType(spec)
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
	missing := ValidateApi(api, requestBody)
	if len(missing) == 0 {
		if len(serversToSave) == 0 {
			print(api.Servers[0].Uri)
			for _, s := range api.Servers {
				if s.Uri != "" {
					server := models.Server{
						Id:          uuid.New().String(),
						Uri:         s.Uri,
						Description: s.Description,
					}
					serversToSave = append(serversToSave, server)
				}
			}
			api.Servers = serversToSave
		}
		for _, server := range serversToSave {
			if err := s.repo.SaveServer(server); err != nil {
				missing = append(missing, fmt.Sprintf("kan server niet opslaan (%s): %v", server.Uri, err))
			}
		}
	}
	return api, missing
}

func ValidateApi(api *models.Api, requestBody models.Api) []string {
	var missing []string
	if api.Title == "" {
		if requestBody.Title != "" {
			api.Title = requestBody.Title
		} else {
			missing = append(missing, "title")
		}
	}
	if api.Description == "" {
		if requestBody.Description != "" {
			api.Description = requestBody.Description
		} else {
			missing = append(missing, "description")
		}
	}
	if api.RepositoryUri == "" {
		if requestBody.RepositoryUri != "" {
			api.RepositoryUri = requestBody.RepositoryUri
		} else {
			missing = append(missing, "RepositoryUri")
		}
	}
	if api.ContactUrl == "" {
		if requestBody.ContactUrl != "" {
			api.ContactUrl = requestBody.ContactUrl
		} else {
			missing = append(missing, "ContactUrl")
		}
	}
	if api.ContactName == "" {
		if requestBody.ContactName != "" {
			api.ContactName = requestBody.ContactName
		} else {
			missing = append(missing, "ContactName")
		}
	}
	if api.ContactEmail == "" {
		if requestBody.ContactEmail != "" {
			api.ContactEmail = requestBody.ContactEmail
		} else {
			missing = append(missing, "ContactEmail")
		}
	}
	//if api.Auth == "" {
	//	if requestBody.Auth != "" {
	//		api.Auth = requestBody.Auth
	//	} else {
	//		missing = append(missing, "Auth")
	//	}
	//}
	if api.DocsUri == "" {
		if requestBody.DocsUri != "" {
			api.DocsUri = requestBody.DocsUri
		} else {
			missing = append(missing, "DocsUri")
		}
	}
	if len(api.Servers) == 0 {
		if len(requestBody.Servers) > 0 {
			api.Servers = requestBody.Servers
		} else {
			missing = append(missing, "Servers")
		}
	}
	return missing
}

func deriveAuthType(spec *openapi3.T) string {
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

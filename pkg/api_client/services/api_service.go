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

func (s *APIsAPIService) CreateApiFromOas(ctx context.Context, oasUrl string) (*models.Api, error) {
	parsedUrl, err := url.Parse(oasUrl)
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

	api, missing := s.BuildApiAndValidate(spec, oasUrl)

	if len(missing) > 0 {
		return nil, fmt.Errorf("De volgende verplichte gegevens ontbreken: %s", strings.Join(missing, ", "))
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

func (s *APIsAPIService) BuildApiAndValidate(spec *openapi3.T, oasUrl string) (*models.Api, []string) {
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

	api.OasUri = oasUrl
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
			api.Servers = serversToSave
		}
	}
	missing := ValidateApi(api)
	if len(missing) == 0 {
		for _, server := range serversToSave {
			if err := s.repo.SaveServer(server); err != nil {
				missing = append(missing, fmt.Sprintf("kan server niet opslaan (%s): %v", server.Uri, err))
			}
		}
	}
	return api, missing
}

func ValidateApi(api *models.Api) []string {
	var missing []string
	if api.Title == "" {
		missing = append(missing, "title")
	}
	if api.Description == "" {
		missing = append(missing, "description")
	}
	if api.RepositoryUri == "" {
		missing = append(missing, "RepositoryUri")
	}
	if api.ContactUrl == "" {
		missing = append(missing, "ContactUrl")
	}
	if api.ContactName == "" {
		missing = append(missing, "ContactName")
	}
	if api.ContactEmail == "" {
		missing = append(missing, "ContactEmail")
	}
	if api.DocsUri == "" {
		missing = append(missing, "DocsUri")
	}
	if api.Servers == nil || len(api.Servers) == 0 {
		missing = append(missing, "Servers")
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

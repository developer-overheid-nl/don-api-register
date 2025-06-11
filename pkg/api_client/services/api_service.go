package services

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/developer-overheid-nl/don-api-register/pkg/api_client/helpers"
	"github.com/developer-overheid-nl/don-api-register/pkg/api_client/models"
	"github.com/developer-overheid-nl/don-api-register/pkg/api_client/repositories"
	"github.com/developer-overheid-nl/don-api-register/pkg/linter"
	"github.com/getkin/kin-openapi/openapi3"
	"github.com/google/uuid"
	"golang.org/x/sync/errgroup"
	"gorm.io/gorm"
	"io"
	"net/http"
	"net/url"
	"time"
)

var ErrNeedsPost = errors.New(
	"oasUri niet gevonden of aangepast; registreer een nieuwe API via POST en markeer de oude als deprecated",
)

// APIsAPIService implementeert APIsAPIServicer met de benodigde repository
type APIsAPIService struct {
	repo repositories.ApiRepository
}

// NewAPIsAPIService Constructor-functie
func NewAPIsAPIService(repo repositories.ApiRepository) *APIsAPIService {
	return &APIsAPIService{repo: repo}
}

func (s *APIsAPIService) UpdateOasUri(ctx context.Context, oasUri string) error {
	api, err := s.repo.FindByOasUrl(ctx, oasUri)
	if err != nil || api == nil {
		if api == nil || errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("%w: %s", ErrNeedsPost, oasUri)
		}
		return fmt.Errorf("databasefout bij FindByOasUrl: %w", err)
	}
	_ = s.lintAndPersist(ctx, api, oasUri)
	return nil
}

func (s *APIsAPIService) RetrieveApi(ctx context.Context, id string) (*models.ApiWithLintResponse, error) {
	api, err := s.repo.GetApiByID(ctx, id)
	if err != nil || api == nil {
		return nil, err
	}
	lintResults, err := s.repo.GetLintResults(ctx, id)
	if err != nil {
		return nil, err
	}

	return &models.ApiWithLintResponse{
		Api:         api,
		LintResults: lintResults,
	}, nil
}

func (s *APIsAPIService) ListApis(ctx context.Context, page, perPage int, baseURL string) (models.ApiListResponse, error) {
	apis, pagination, err := s.repo.GetApis(ctx, page, perPage)
	if err != nil {
		return models.ApiListResponse{}, err
	}

	// map domain-model → DTO
	dtos := make([]*models.ApiResponse, len(apis))
	for i, api := range apis {
		dtos[i] = helpers.ToDTO(&api)
	}

	// links bouwen
	buildURL := func(p int) *models.Link {
		return &models.Link{Href: fmt.Sprintf("%s?page=%d&perPage=%d", baseURL, p, perPage)}
	}
	links := models.Links{Self: buildURL(page)}
	if pagination.Next != nil {
		links.Next = buildURL(*pagination.Next)
	}
	if pagination.Previous != nil {
		links.Prev = buildURL(*pagination.Previous)
	}

	return models.ApiListResponse{
		Links: links,
		Apis:  dtos,
	}, nil
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

func (s *APIsAPIService) CreateApiFromOas(requestBody models.Api) (*models.ApiResponse, error) {
	// 1) Parse en haal op
	parsedUrl, err := url.Parse(requestBody.OasUri)
	if err != nil {
		return nil, helpers.NewBadRequest(
			"Ongeldige URL",
			helpers.InvalidParam{Name: "oasUri", Reason: "Moet een geldige URL zijn"},
		)
	}
	resp, err := CorsGet(&http.Client{}, parsedUrl.String(), "https://developer.overheid.nl")
	if err != nil {
		return nil, helpers.NewInternalServerError(
			fmt.Sprintf("fout bij ophalen OAS: %s", err),
		)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, helpers.NewInternalServerError(
			fmt.Sprintf("OAS download faalt met status %d", resp.StatusCode),
		)
	}

	// 2) Parse OpenAPI
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, helpers.NewInternalServerError("kan response body niet lezen")
	}
	loader := openapi3.NewLoader()
	loader.IsExternalRefsAllowed = true //voor de hash is belangrijk dat deze plat geslagen wordt.
	spec, err := loader.LoadFromData(data)
	if err != nil {
		return nil, helpers.NewBadRequest(
			"Ongeldig OpenAPI-bestand",
			helpers.InvalidParam{Name: "oasUri", Reason: err.Error()},
		)
	}

	// 3) Build & validate
	api := s.BuildApi(spec, requestBody)
	invalids := ValidateApi(api, requestBody)
	if len(invalids) > 0 {
		return nil, helpers.NewBadRequest(
			"Validatie mislukt: ontbrekende of ongeldige eigenschappen",
			invalids...,
		)
	}

	_ = s.lintAndPersist(context.Background(), api, requestBody.OasUri)
	// 4) Sla op in DB
	for _, server := range api.Servers {
		if err := s.repo.SaveServer(server); err != nil {
			return nil, helpers.NewInternalServerError("Probleem bij het opslaan van het server object: " + err.Error())
		}
	}
	if err := s.repo.Save(api); err != nil {
		return nil, helpers.NewInternalServerError("kan API niet opslaan: " + err.Error())
	}

	return helpers.ToDTO(api), nil
}

func (s *APIsAPIService) BuildApi(spec *openapi3.T, requestBody models.Api) *models.Api {
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
	api.OrganisationID = nil
	return api
}

func ValidateApi(api *models.Api, requestBody models.Api) []helpers.InvalidParam {
	var invalids []helpers.InvalidParam
	if api.ContactUrl == "" {
		if requestBody.ContactUrl != "" {
			api.ContactUrl = requestBody.ContactUrl
		} else {
			invalids = append(invalids, helpers.InvalidParam{
				Name:   "contact.url",
				Reason: "contact.url is verplicht",
			})
		}
	}
	if api.ContactName == "" {
		if requestBody.ContactName != "" {
			api.ContactName = requestBody.ContactName
		} else {
			invalids = append(invalids, helpers.InvalidParam{
				Name:   "contact.name",
				Reason: "contact.name is verplicht",
			})
		}
	}
	if api.ContactEmail == "" {
		if requestBody.ContactEmail != "" {
			api.ContactEmail = requestBody.ContactEmail
		} else {
			invalids = append(invalids, helpers.InvalidParam{
				Name:   "contact.email",
				Reason: "contact.email is verplicht",
			})
		}
	}

	return invalids
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

func computeOASHash(oasURL string) (string, error) {
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
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}

// LintAllApis runs the linter for every registered API and stores
// the output in the database
func (s *APIsAPIService) LintAllApis(ctx context.Context) error {
	apis, err := s.repo.AllApis(ctx)
	if err != nil {
		return err
	}

	sem := make(chan struct{}, 5)
	g, ctx := errgroup.WithContext(ctx)

	for _, api := range apis {
		api := api
		g.Go(func() error {
			sem <- struct{}{}
			defer func() { <-sem }()
			return s.lintAndPersist(ctx, &api, api.OasUri)
		})
	}
	return g.Wait()
}

// lintAndPersist runs the linter when the OAS has changed and stores
// both the lint result and updated hash.
func (s *APIsAPIService) lintAndPersist(ctx context.Context, api *models.Api, uri string) error {
	newHash, err := computeOASHash(uri)
	if err != nil {
		return err
	}
	if newHash == api.OasHash {
		return nil
	}

	output, lintErr := linter.LintURL(ctx, uri)
	timeNow := time.Now()
	msgs := helpers.ParseOutput(output, timeNow)
	var errorCount, warnCount int
	for _, m := range msgs {
		switch m.Severity {
		case "error":
			errorCount++
		case "warning":
			warnCount++
		}
	}
	res := &models.LintResult{
		ID:        uuid.New().String(),
		ApiID:     api.Id,
		Successes: errorCount == 0,
		Failures:  errorCount,
		Warnings:  warnCount,
		Messages:  msgs,
		CreatedAt: timeNow,
	}
	if saveErr := s.repo.SaveLintResult(ctx, res); saveErr != nil {
		return saveErr
	}
	api.OasHash = newHash
	if err := s.repo.UpdateApi(ctx, *api); err != nil {
		return err
	}
	return lintErr
}

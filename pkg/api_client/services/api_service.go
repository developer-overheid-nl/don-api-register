package services

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	httpclient "github.com/developer-overheid-nl/don-api-register/pkg/api_client/helpers/httpclient"
	openapi "github.com/developer-overheid-nl/don-api-register/pkg/api_client/helpers/openapi"
	problem "github.com/developer-overheid-nl/don-api-register/pkg/api_client/helpers/problem"
	util "github.com/developer-overheid-nl/don-api-register/pkg/api_client/helpers/util"
	"github.com/developer-overheid-nl/don-api-register/pkg/api_client/models"
	"github.com/developer-overheid-nl/don-api-register/pkg/api_client/repositories"
	"github.com/developer-overheid-nl/don-api-register/pkg/linter"
	"github.com/developer-overheid-nl/don-api-register/pkg/tools"
	"github.com/getkin/kin-openapi/openapi3"
	"github.com/google/uuid"
	"golang.org/x/sync/errgroup"
	"gorm.io/gorm"
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

func (s *APIsAPIService) UpdateOasUri(ctx context.Context, body *models.UpdateApiInput) (*models.ApiSummary, error) {
	api, err := s.repo.FindByOasUrl(ctx, body.OasUrl)
	if err != nil || api == nil {
		if api == nil || errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("%w: %s", ErrNeedsPost, body.OasUrl)
		}
		return nil, fmt.Errorf("databasefout bij GetApiByID: %w", err)
	}
	organisation := *api.Organisation
	if api.OrganisationID == nil || organisation.Uri != body.OrganisationUri {
		forbidden := problem.NewForbidden(body.OasUrl, "organisationUri komt niet overeen met eigenaar van deze API")
		return nil, forbidden
	}
	tools.Dispatch(context.Background(), "lint", func(ctx context.Context) error {
		return s.lintAndPersist(ctx, api, body.OasUrl)
	})
	updated := util.ToApiSummary(api)
	return &updated, nil
}

func (s *APIsAPIService) RetrieveApi(ctx context.Context, id string) (*models.ApiDetail, error) {
	api, err := s.repo.GetApiByID(ctx, id)
	if err != nil || api == nil {
		return nil, err
	}
	detail := util.ToApiDetail(api)
	return detail, nil
}

func (s *APIsAPIService) ListApis(ctx context.Context, page, perPage int, baseURL string) (*models.ApiListResponse, error) {
	apis, pagination, err := s.repo.GetApis(ctx, page, perPage)
	if err != nil {
		return nil, err
	}

	// map naar ApiSummary (ipv ApiResponse)
	dtos := make([]models.ApiSummary, len(apis))
	for i, api := range apis {
		dtos[i] = util.ToApiSummary(&api)
	}

	// links bouwen (zelfde als je had)
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

	// Meta data
	meta := models.Meta{Pagination: pagination}

	return &models.ApiListResponse{
		Links: links,
		Meta: meta,
		Apis:  dtos,
	}, nil
}

func (s *APIsAPIService) UpdateApi(ctx context.Context, api models.Api) error {
	return s.repo.UpdateApi(ctx, api)
}

func (s *APIsAPIService) CreateApiFromOas(requestBody models.ApiPost) (*models.ApiSummary, error) {
	// 1) Parse en haal op
	parsedUrl, err := url.Parse(requestBody.OasUrl)
	if err != nil {
		return nil, problem.NewBadRequest(
			requestBody.OasUrl,
			"Ongeldige URL",
			problem.InvalidParam{Name: "oasUri", Reason: "Moet een geldige URL zijn"},
		)
	}
	resp, err := httpclient.CorsGet(&http.Client{}, parsedUrl.String(), "https://developer.overheid.nl")
	if err != nil {
		return nil, problem.NewBadRequest(requestBody.OasUrl,
			fmt.Sprintf("fout bij ophalen OAS: %s", err),
		)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, problem.NewBadRequest(requestBody.OasUrl,
			fmt.Sprintf("OAS download faalt met status %d", resp.StatusCode),
		)
	}

	// 2) Parse OpenAPI
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, problem.NewBadRequest(requestBody.OasUrl, "kan response body niet lezen")
	}
	loader := openapi3.NewLoader()
	loader.IsExternalRefsAllowed = true
	spec, err := loader.LoadFromData(data)
	if err != nil {
		return nil, problem.NewBadRequest(
			requestBody.OasUrl,
			"Ongeldig OpenAPI-bestand",
			problem.InvalidParam{Name: "oasUri", Reason: err.Error()},
		)
	}
	if err := loader.ResolveRefsIn(spec, nil); err != nil {
		return nil, problem.NewBadRequest(requestBody.OasUrl, "could not resolve refs", problem.InvalidParam{Name: "External refs", Reason: err.Error()})
	}

	if err := spec.Validate(context.Background()); err != nil {
		if !strings.Contains(err.Error(), "invalid example") {
			apiErr := problem.NewBadRequest(requestBody.OasUrl, err.Error())
			apiErr.Title = "Invalid OAS"
			apiErr.Instance = requestBody.OasUrl
			return nil, apiErr
		}
	}

	// 3) Build & validate
	var label string
	var shouldSaveOrg bool
	if org, err := s.repo.FindOrganisationByURI(context.Background(), requestBody.OrganisationUri); err != nil {
		return nil, problem.NewInternalServerError("kan organisatie niet ophalen: " + err.Error())
	} else if org != nil {
		label = org.Label
	} else {
		if _, err := url.ParseRequestURI(requestBody.OrganisationUri); err != nil {
			return nil, problem.NewBadRequest(
				requestBody.OrganisationUri,
				"Ongeldige URL",
				problem.InvalidParam{Name: "organisationUri", Reason: "Moet een geldige URL zijn"},
			)
		}
		lbl, err := httpclient.FetchOrganisationLabel(context.Background(), requestBody.OrganisationUri)
		if err != nil {
			return nil, problem.NewBadRequest(requestBody.OrganisationUri, fmt.Sprintf("fout bij ophalen organisatie: %s", err))
		}
		label = lbl
		shouldSaveOrg = true
	}
	api := openapi.BuildApi(spec, requestBody, label)
	if shouldSaveOrg && api.OrganisationID != nil {
		if err := s.repo.SaveOrganisatie(api.Organisation); err != nil {
			return nil, problem.NewInternalServerError("kan organisatie niet opslaan: " + err.Error())
		}
	}
	invalids := openapi.ValidateApi(api)
	if len(invalids) > 0 {
		return nil, problem.NewBadRequest(
			requestBody.OasUrl,
			"Validatie mislukt: ontbrekende of ongeldige eigenschappen",
			invalids...,
		)
	}

	// 4) Sla op in DB
	for _, server := range api.Servers {
		if err := s.repo.SaveServer(server); err != nil {
			return nil, problem.NewInternalServerError("Probleem bij het opslaan van het server object: " + err.Error())
		}
	}
	if err := s.repo.Save(api); err != nil {
		if strings.Contains(err.Error(), "api bestaat al") {
			bad := problem.NewBadRequest(requestBody.OasUrl, "kan API niet opslaan: "+err.Error())
			return nil, bad
		}
		return nil, problem.NewInternalServerError("kan API niet opslaan: " + err.Error())
	}

	tools.Dispatch(context.Background(), "lint", func(ctx context.Context) error {
		return s.lintAndPersist(ctx, api, requestBody.OasUrl)
	})

	created := util.ToApiSummary(api)
	return &created, nil
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
	newHash, err := openapi.ComputeOASHash(uri)
	if err != nil {
		return err
	}
	if newHash == api.OasHash {
		return nil
	}

	output, lintErr := linter.LintURL(ctx, uri)
	timeNow := time.Now()
	msgs := openapi.ParseOutput(output, timeNow)
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

func (s *APIsAPIService) ListOrganisations(ctx context.Context) ([]models.Organisation, error) {
	return s.repo.GetOrganisations(ctx)
}

// CreateOrganisation validates and stores a new organisation
func (s *APIsAPIService) CreateOrganisation(ctx context.Context, org *models.Organisation) (*models.Organisation, error) {
	if _, err := url.ParseRequestURI(org.Uri); err != nil {
		return nil, problem.NewBadRequest(org.Uri, fmt.Sprintf("foutieve uri: %v", err),
			problem.InvalidParam{Name: "uri", Reason: "Moet een geldige URL zijn"})
	}
	if strings.TrimSpace(org.Label) == "" {
		return nil, problem.NewBadRequest(org.Uri, "label is verplicht",
			problem.InvalidParam{Name: "label", Reason: "label is verplicht"})
	}
	if err := s.repo.SaveOrganisatie(org); err != nil {
		return nil, err
	}
	return org, nil
}

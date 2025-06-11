package services

import (
	"context"
	"errors"
	"fmt"
	"github.com/developer-overheid-nl/don-api-register/pkg/api_client/helpers"
	"github.com/developer-overheid-nl/don-api-register/pkg/api_client/models"
	"github.com/developer-overheid-nl/don-api-register/pkg/api_client/repositories"
	"github.com/developer-overheid-nl/don-api-register/pkg/linter"
	"github.com/developer-overheid-nl/don-api-register/pkg/tools"
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
	tools.Dispatch(context.Background(), "lint", func(ctx context.Context) error {
		return s.lintAndPersist(ctx, api, oasUri)
	})
	println("OAS URI updated and linted:", oasUri)
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

func (s *APIsAPIService) CreateApiFromOas(requestBody models.Api) (*models.ApiResponse, error) {
	// 1) Parse en haal op
	parsedUrl, err := url.Parse(requestBody.OasUri)
	if err != nil {
		return nil, helpers.NewBadRequest(
			"Ongeldige URL",
			helpers.InvalidParam{Name: "oasUri", Reason: "Moet een geldige URL zijn"},
		)
	}
	resp, err := helpers.CorsGet(&http.Client{}, parsedUrl.String(), "https://developer.overheid.nl")
	if err != nil {
		return nil, helpers.NewBadRequest(
			fmt.Sprintf("fout bij ophalen OAS: %s", err),
		)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, helpers.NewBadRequest(
			fmt.Sprintf("OAS download faalt met status %d", resp.StatusCode),
		)
	}

	// 2) Parse OpenAPI
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, helpers.NewBadRequest("kan response body niet lezen")
	}
	loader := openapi3.NewLoader()
	loader.IsExternalRefsAllowed = true
	spec, err := loader.LoadFromData(data)
	if err != nil {
		return nil, helpers.NewBadRequest(
			"Ongeldig OpenAPI-bestand",
			helpers.InvalidParam{Name: "oasUri", Reason: err.Error()},
		)
	}
	if err := loader.ResolveRefsIn(spec, nil); err != nil {
		return nil, helpers.NewBadRequest("could not resolve refs", helpers.InvalidParam{Name: "External refs", Reason: err.Error()})
	}
	if err := spec.Validate(context.Background()); err != nil {
		return nil, helpers.NewBadRequest("invalid OAS document", helpers.InvalidParam{Name: "Invalid OAS document", Reason: err.Error()})
	}

	// 3) Build & validate
	api := helpers.BuildApi(spec, requestBody)
	invalids := helpers.ValidateApi(api, requestBody)
	if len(invalids) > 0 {
		return nil, helpers.NewBadRequest(
			"Validatie mislukt: ontbrekende of ongeldige eigenschappen",
			invalids...,
		)
	}

	tools.Dispatch(context.Background(), "lint", func(ctx context.Context) error {
		return s.lintAndPersist(ctx, api, requestBody.OasUri)
	})

	println("OAS URI updated and linted:", requestBody.OasUri)
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
	println("Linting API with OAS URI:", uri)
	newHash, err := helpers.ComputeOASHash(uri)
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

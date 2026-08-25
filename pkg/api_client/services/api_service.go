package services

import (
	"bytes"
	"context"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	httpclient "github.com/developer-overheid-nl/don-api-register/pkg/api_client/helpers/httpclient"
	openapi "github.com/developer-overheid-nl/don-api-register/pkg/api_client/helpers/openapi"
	problem "github.com/developer-overheid-nl/don-api-register/pkg/api_client/helpers/problem"
	toolslint "github.com/developer-overheid-nl/don-api-register/pkg/api_client/helpers/tools"
	util "github.com/developer-overheid-nl/don-api-register/pkg/api_client/helpers/util"
	"github.com/developer-overheid-nl/don-api-register/pkg/api_client/models"
	"github.com/developer-overheid-nl/don-api-register/pkg/api_client/repositories"
	typesense "github.com/developer-overheid-nl/don-api-register/pkg/api_client/services/typesense"
	"github.com/google/uuid"
	"golang.org/x/sync/errgroup"
	"golang.org/x/sync/semaphore"
	"golang.org/x/time/rate"
	"gorm.io/gorm"
	"sigs.k8s.io/yaml"
)

var ErrNeedsPost = errors.New(
	"oasUri niet gevonden of aangepast; registreer een nieuwe API via POST en markeer de oude als deprecated",
)

// APIsAPIService implementeert APIsAPIServicer met de benodigde repository
type APIsAPIService struct {
	repo    repositories.ApiRepository
	limiter *rate.Limiter
}

// NewAPIsAPIService Constructor-functie
func NewAPIsAPIService(repo repositories.ApiRepository) *APIsAPIService {
	return &APIsAPIService{
		repo:    repo,
		limiter: rate.NewLimiter(rate.Every(time.Second*5), 1), // 1 per 5 seconden, burst 1
	}
}

func (s *APIsAPIService) withRateLimit(ctx context.Context) error {
	return s.limiter.Wait(ctx)
}

func (s *APIsAPIService) UpdateOasUri(ctx context.Context, body *models.UpdateApiInput) (*models.ApiSummary, error) {
	api, err := s.repo.GetApiByID(ctx, body.Id)
	if err != nil || api == nil {
		if api == nil || errors.Is(err, gorm.ErrRecordNotFound) {
			if !hasOASUpdateInput(body) {
				return nil, problem.NewNotFound(body.Id, "Api not found")
			}
			return nil, fmt.Errorf("%w: %s", ErrNeedsPost, body.OasUrl)
		}
		return nil, fmt.Errorf("databasefout: %w", err)
	}
	if ownerURI := deriveOrganisationURI(api); ownerURI == "" || ownerURI != strings.TrimSpace(body.OrganisationUri) {
		return nil, problem.NewForbidden(body.OasUrl, "organisationUri komt niet overeen met eigenaar van deze API")
	}
	if err := validateLifecycleOverrides(body); err != nil {
		return nil, err
	}
	if !hasOASUpdateInput(body) {
		return s.applyLifecycleUpdate(ctx, api, body)
	}
	if body.Sunset.Set && !hasOASDocumentChange(api, body) {
		return s.applyLifecycleUpdate(ctx, api, body)
	}

	oasInput := toolslint.OASInput{
		OasUrl:  body.OasUrl,
		OasBody: body.OasBody,
	}
	var updated *models.ApiSummary
	parsed := false
	err = openapi.ProcessOAS(ctx, oasInput, openapi.FetchOpts{
		Origin: "https://developer.overheid.nl",
	}, func(res *openapi.OASResult) error {
		parsed = true
		var applyErr error
		updated, applyErr = s.applyOASUpdate(ctx, api, models.ApiPost{
			Id:              body.Id,
			OasUrl:          body.OasUrl,
			OasBody:         body.OasBody,
			ArazzoUrl:       body.ArazzoUrl,
			ArazzoBody:      body.ArazzoBody,
			OrganisationUri: body.OrganisationUri,
			Contact:         body.Contact,
		}, body, res, true)
		return applyErr
	})
	if err != nil {
		if parsed {
			return nil, err
		}
		if openapi.IsHTTPStatus(err, http.StatusNotFound) && !hasOASDocumentChange(api, body) {
			if _, retireErr := s.retireAPIForUnavailableOAS(ctx, *api); retireErr != nil {
				return nil, retireErr
			}
			api.Sunset = retiredLifecycleDate(time.Now())
			if api.Deprecated == "" {
				api.Deprecated = api.Sunset
			}
			updated := util.ToApiSummary(api)
			return &updated, nil
		}
		return nil, problem.NewBadRequest(body.OasUrl, err.Error())
	}

	return updated, nil
}

func (s *APIsAPIService) applyOASUpdate(ctx context.Context, api *models.Api, request models.ApiPost, overrides *models.UpdateApiInput, res *openapi.OASResult, asyncTools bool) (*models.ApiSummary, error) {
	if api == nil {
		return nil, fmt.Errorf("api ontbreekt")
	}
	if res == nil {
		return nil, fmt.Errorf("OAS resultaat ontbreekt")
	}
	before := *api

	orgLabel := ""
	if api.Organisation != nil {
		orgLabel = api.Organisation.Label
	} else if trimmed := strings.TrimSpace(request.OrganisationUri); trimmed != "" {
		if org, err := s.repo.FindOrganisationByURI(ctx, trimmed); err == nil && org != nil {
			orgLabel = org.Label
		}
	}

	openapi.UpdateApiFromSpec(api, res.Spec, request, orgLabel)
	applyOASSnapshot(api, res)
	applyLifecycleOverrides(api, overrides)
	if api.Organisation == nil && strings.TrimSpace(request.OrganisationUri) != "" {
		api.Organisation = &models.Organisation{
			Uri:   request.OrganisationUri,
			Label: orgLabel,
		}
	}
	if api.Organisation != nil && api.OrganisationID == nil {
		api.OrganisationID = &api.Organisation.Uri
	}

	if err := s.repo.UpdateApi(ctx, *api); err != nil {
		return nil, err
	}
	s.recordLifecycleChange(ctx, before, *api)

	oasInput := toOASInput(request)
	arazzoInput := toArazzoInput(request)
	toolResult := detachedOASResult(res)
	run := func(runCtx context.Context) error {
		return s.runToolsAndPersist(runCtx, api.Id, oasInput, arazzoInput, toolResult)
	}
	if asyncTools {
		toolslint.Dispatch(context.Background(), "tools", run)
	} else {
		if err := run(ctx); err != nil {
			slog.ErrorContext(
				ctx,
				"synchronous API tooling failed",
				"component", "tools",
				"operation", "process_api",
				"api_id", api.Id,
				"error", err,
			)
		}
	}

	updated := util.ToApiSummary(api)
	return &updated, nil
}

func (s *APIsAPIService) applyLifecycleUpdate(ctx context.Context, api *models.Api, body *models.UpdateApiInput) (*models.ApiSummary, error) {
	if api == nil {
		return nil, fmt.Errorf("api ontbreekt")
	}
	before := *api
	applyLifecycleOverrides(api, body)
	if err := s.repo.UpdateApi(ctx, *api); err != nil {
		return nil, err
	}
	s.recordLifecycleChange(ctx, before, *api)
	updated := util.ToApiSummary(api)
	return &updated, nil
}

func (s *APIsAPIService) RetrieveApi(ctx context.Context, id string) (*models.ApiDetail, error) {
	api, err := s.repo.GetApiByID(ctx, id)
	if err != nil || api == nil {
		return nil, err
	}
	detail := util.ToApiDetail(api)

	lintResults, err := s.repo.GetLintResults(ctx, api.Id)
	if err != nil {
		return nil, err
	}
	detail.LintResults = lintResults

	return detail, nil
}

func (s *APIsAPIService) GetApiFeed(ctx context.Context, id, apiURL string) ([]byte, error) {
	api, err := s.repo.GetApiByID(ctx, id)
	if err != nil || api == nil {
		return nil, err
	}
	events, err := s.repo.ListApiFeedEvents(ctx, id, 50)
	if err != nil {
		return nil, err
	}
	title := strings.TrimSpace(api.Title)
	if title == "" {
		title = api.Id
	}
	feed := models.RSSFeed{
		Version: "2.0",
		Channel: models.RSSChannel{
			Title:       fmt.Sprintf("Wijzigingen voor %s", title),
			Link:        apiURL,
			Description: fmt.Sprintf("RSS feed met inhoudelijke wijzigingen voor %s.", title),
			Language:    "nl",
		},
	}
	if len(events) > 0 {
		feed.Channel.LastBuildDate = events[0].CreatedAt.Format(time.RFC1123Z)
	}
	for _, event := range events {
		feed.Channel.Items = append(feed.Channel.Items, models.RSSItem{
			Title:       event.Title,
			Link:        apiURL,
			GUID:        models.RSSGUID{Value: event.ID, IsPermaLink: "false"},
			Description: event.Description,
			PubDate:     event.CreatedAt.Format(time.RFC1123Z),
		})
	}
	out, err := xml.MarshalIndent(feed, "", "  ")
	if err != nil {
		return nil, err
	}
	return append([]byte(xml.Header), out...), nil
}

func (s *APIsAPIService) ListLintResults(ctx context.Context) ([]models.LintResult, error) {
	return s.repo.ListLintResults(ctx)
}

func (s *APIsAPIService) ListApis(ctx context.Context, p *models.ListApisParams) ([]models.ApiSummary, models.Pagination, error) {
	if p == nil {
		p = &models.ListApisParams{}
	}
	sorting, err := models.ParseApiSort(p.SortBy, p.SortOrder)
	if err != nil {
		var invalid models.InvalidApiSortError
		if errors.As(err, &invalid) {
			return nil, models.Pagination{}, problem.New(http.StatusBadRequest, "Request validation failed", problem.ErrorDetail{
				In:       "query",
				Location: invalid.Parameter,
				Code:     invalid.Parameter,
				Detail:   invalid.Error(),
			})
		}
		return nil, models.Pagination{}, err
	}

	apis, pagination, err := s.repo.GetApis(ctx, p.Page, p.PerPage, p.ApiFilters(), sorting)
	if err != nil {
		return nil, models.Pagination{}, err
	}

	dtos := make([]models.ApiSummary, len(apis))
	for i, api := range apis {
		dtos[i] = util.ToApiSummary(&api)
	}

	return dtos, pagination, nil
}

func (s *APIsAPIService) GetApiFilters(ctx context.Context, p *models.ApiFiltersParams) ([]models.FilterGroup, error) {
	if p == nil {
		p = &models.ApiFiltersParams{}
	}
	counts, err := s.repo.GetApiFilterCounts(ctx, p)
	if err != nil {
		return nil, err
	}
	groups := []models.FilterGroup{
		buildStatusGroup(p, counts),
		buildOasVersionGroup(p, counts),
		buildAdrScoreGroup(p, counts),
		buildAuthGroup(p, counts),
		buildOrganisationGroup(p, counts),
	}
	for _, g := range groups {
		if err := g.Validate(); err != nil {
			return nil, err
		}
	}
	return groups, nil
}

func (s *APIsAPIService) SearchApis(ctx context.Context, p *models.ListApisSearchParams) ([]models.ApiSummary, models.Pagination, error) {
	if p == nil {
		p = &models.ListApisSearchParams{}
	}
	trimmed := strings.TrimSpace(p.Query)
	if trimmed == "" {
		return []models.ApiSummary{}, models.Pagination{
			CurrentPage:    p.Page,
			RecordsPerPage: p.PerPage,
		}, nil
	}
	apis, pagination, err := s.repo.SearchApis(ctx, p.Page, p.PerPage, p.Organisation, trimmed)
	if err != nil {
		return nil, models.Pagination{}, err
	}
	results := make([]models.ApiSummary, len(apis))
	for i := range apis {
		results[i] = util.ToApiSummary(&apis[i])
	}
	return results, pagination, nil
}

func (s *APIsAPIService) UpdateApi(ctx context.Context, api models.Api) error {
	return s.repo.UpdateApi(ctx, api)
}

func (s *APIsAPIService) CreateApiFromOas(requestBody models.ApiPost) (*models.ApiSummary, error) {
	ctx := context.Background()
	oasInput := toolslint.OASInput{
		OasUrl:  requestBody.OasUrl,
		OasBody: requestBody.OasBody,
	}
	arazzoInput := toArazzoInput(requestBody)

	var (
		created    *models.ApiSummary
		createdAPI *models.Api
		toolResult *openapi.OASResult
		parsed     bool
	)
	err := openapi.ProcessOAS(ctx, oasInput, openapi.FetchOpts{
		Origin: "https://developer.overheid.nl",
	}, func(resp *openapi.OASResult) error {
		parsed = true
		api, err := s.createAPIFromParsedOAS(ctx, requestBody, resp)
		if err != nil {
			return err
		}
		createdAPI = api
		toolResult = detachedOASResult(resp)
		summary := util.ToApiSummary(api)
		created = &summary
		return nil
	})
	if err != nil {
		if parsed {
			return nil, err
		}
		return nil, problem.NewBadRequest(requestBody.OasUrl, err.Error())
	}

	toolslint.Dispatch(context.Background(), "tools", func(ctx context.Context) error {
		return s.runToolsAndPersist(ctx, createdAPI.Id, oasInput, arazzoInput, toolResult)
	})

	apiCopy := *createdAPI
	go s.publishToTypesense(apiCopy)

	return created, nil
}

func (s *APIsAPIService) createAPIFromParsedOAS(
	ctx context.Context,
	requestBody models.ApiPost,
	resp *openapi.OASResult,
) (*models.Api, error) {
	var label string
	var shouldSaveOrg bool
	if org, err := s.repo.FindOrganisationByURI(ctx, requestBody.OrganisationUri); err != nil {
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
		lbl, err := httpclient.FetchOrganisationLabel(ctx, requestBody.OrganisationUri)
		if err != nil {
			return nil, problem.NewBadRequest(requestBody.OrganisationUri, fmt.Sprintf("fout bij ophalen organisatie: %s", err))
		}
		label = lbl
		shouldSaveOrg = true
	}
	api := openapi.BuildApi(resp.Spec, requestBody, label)
	applyOASSnapshot(api, resp)
	if shouldSaveOrg && api.OrganisationID != nil {
		if err := s.repo.SaveOrganisatie(api.Organisation); err != nil {
			return nil, problem.NewInternalServerError("kan organisatie niet opslaan: " + err.Error())
		}
	}
	invalids := openapi.ValidateApi(api, requestBody)
	if len(invalids) > 0 {
		return nil, problem.NewBadRequest(
			requestBody.OasUrl,
			"Validatie mislukt: ontbrekende of ongeldige eigenschappen",
			invalids...,
		)
	}

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

	api.OasHash = resp.Hash
	if err := s.repo.UpdateApi(ctx, *api); err != nil {
		return nil, problem.NewInternalServerError("kan API hash niet opslaan: " + err.Error())
	}
	return api, nil
}

// RefreshChangedApis haalt alle geregistreerde APIs op, vergelijkt de OAS-hash en
// voert dezelfde stappen uit als een POST wanneer de remote OAS gewijzigd is.
func (s *APIsAPIService) RefreshChangedApis(ctx context.Context) (int, error) {
	apis, err := s.repo.AllApis(ctx)
	if err != nil {
		return 0, err
	}

	var updated int
	for _, candidate := range apis {
		if err := ctx.Err(); err != nil {
			return updated, err
		}

		oasInput := toolslint.OASInput{OasUrl: candidate.OasUri}
		processErr := openapi.ProcessOAS(ctx, oasInput, openapi.FetchOpts{
			Origin: "https://developer.overheid.nl",
		}, func(res *openapi.OASResult) error {
			if s.applyRefreshedOAS(ctx, candidate, res) {
				updated++
			}
			return nil
		})
		if processErr != nil {
			if s.handleOASRefreshFetchError(ctx, candidate, processErr) {
				updated++
			}
		}
	}

	return updated, nil
}

func (s *APIsAPIService) handleOASRefreshFetchError(ctx context.Context, candidate models.Api, fetchErr error) bool {
	if openapi.IsHTTPStatus(fetchErr, http.StatusNotFound) {
		retired, retireErr := s.retireAPIForUnavailableOAS(ctx, candidate)
		if retireErr != nil {
			slog.ErrorContext(
				ctx,
				"failed to retire API after missing OAS",
				"component", "oas_refresh",
				"operation", "retire_api",
				"api_id", candidate.Id,
				"error", retireErr,
			)
		}
		return retired && retireErr == nil
	}

	nextSnapshot := models.OASMetadata{
		Version: candidate.OAS.Version,
		Status:  classifyOASStatus(fetchErr),
		Auth:    currentOASAuth(candidate),
	}
	if updateErr := s.updateOASMetadataSnapshot(ctx, candidate.Id, candidate.OAS, nextSnapshot); updateErr != nil {
		slog.ErrorContext(
			ctx,
			"failed to store unavailable OAS status",
			"component", "oas_refresh",
			"operation", "update_status",
			"api_id", candidate.Id,
			"error", updateErr,
		)
	} else {
		s.recordOASUnavailable(ctx, candidate.Id, candidate.OAS, nextSnapshot)
	}
	slog.WarnContext(
		ctx,
		"skipping API with unavailable OAS",
		"component", "oas_refresh",
		"operation", "fetch_oas",
		"api_id", candidate.Id,
		"error", fetchErr,
	)
	return false
}

func (s *APIsAPIService) applyRefreshedOAS(ctx context.Context, candidate models.Api, res *openapi.OASResult) bool {
	if err := s.updateOASMetadataSnapshot(ctx, candidate.Id, candidate.OAS, deriveOASSnapshot(candidate.OAS, res)); err != nil {
		slog.ErrorContext(
			ctx,
			"failed to update OAS metadata",
			"component", "oas_refresh",
			"operation", "update_metadata",
			"api_id", candidate.Id,
			"error", err,
		)
	}

	if res.Hash == candidate.OasHash {
		return false
	}

	full, err := s.repo.GetApiByID(ctx, candidate.Id)
	if err != nil || full == nil {
		slog.ErrorContext(
			ctx,
			"API detail unavailable during OAS refresh",
			"component", "oas_refresh",
			"operation", "load_api",
			"api_id", candidate.Id,
			"error", err,
		)
		return false
	}

	orgURI := deriveOrganisationURI(full)
	if orgURI == "" {
		slog.WarnContext(
			ctx,
			"skipping API without organisation URI",
			"component", "oas_refresh",
			"operation", "validate_api",
			"api_id", candidate.Id,
		)
		return false
	}

	req := models.ApiPost{
		OasUrl:          full.OasUri,
		OrganisationUri: orgURI,
		Contact: models.Contact{
			Name:  full.ContactName,
			Email: full.ContactEmail,
			URL:   full.ContactUrl,
		},
	}

	if _, err := s.applyOASUpdate(ctx, full, req, nil, res, false); err != nil {
		slog.ErrorContext(
			ctx,
			"failed to update API from refreshed OAS",
			"component", "oas_refresh",
			"operation", "update_api",
			"api_id", full.Id,
			"error", err,
		)
		return false
	}
	return true
}

// LintAllApis runs the linter for every registered API and stores
// the output in the database
func (s *APIsAPIService) LintAllApis(ctx context.Context) error {
	apis, err := s.repo.AllApis(ctx)
	if err != nil {
		return err
	}

	const maxConcurrent = 2
	sem := semaphore.NewWeighted(int64(maxConcurrent))
	g, ctx := errgroup.WithContext(ctx)

	for _, api := range apis {
		// Bereken hash alvast (en respecteer job-context)
		oasInput := toolslint.OASInput{OasUrl: api.OasUri}
		var expectedHash string
		err := openapi.ProcessOAS(ctx, oasInput, openapi.FetchOpts{Origin: "https://developer.overheid.nl"}, func(resp *openapi.OASResult) error {
			expectedHash = resp.Hash
			return nil
		})
		if err != nil {
			if openapi.IsHTTPStatus(err, http.StatusNotFound) {
				if _, retireErr := s.retireAPIForUnavailableOAS(ctx, api); retireErr != nil {
					slog.ErrorContext(
						ctx,
						"failed to retire API after missing OAS",
						"component", "lint",
						"operation", "retire_api",
						"api_id", api.Id,
						"error", retireErr,
					)
				}
			}
			slog.WarnContext(
				ctx,
				"skipping lint for unavailable OAS",
				"component", "lint",
				"operation", "fetch_oas",
				"api_id", api.Id,
				"error", err,
			)
		} else {
			if err := sem.Acquire(ctx, 1); err != nil {
				return err
			}
			g.Go(func() error {
				defer sem.Release(1)

				// Per-lint timeout (overschrijft niet je job-timeout)
				lintCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
				defer cancel()

				return s.lintAndPersist(lintCtx, api.Id, toolslint.OASInput{OasUrl: api.OasUri}, expectedHash)
			})
		}
	}

	return g.Wait()
}

// lintAndPersist runs the linter when the OAS has changed and stores
// both the lint result and updated hash.
func (s *APIsAPIService) lintAndPersist(ctx context.Context, apiID string, input toolslint.OASInput, expectedHash string) error {
	current, err := s.repo.GetApiByID(ctx, apiID)
	if err != nil || current == nil {
		return err
	}

	slog.DebugContext(
		ctx,
		"evaluating OAS lint state",
		"component", "lint",
		"operation", "compare_hash",
		"api_id", apiID,
		"expected_hash", expectedHash,
		"current_hash", current.OasHash,
	)
	// Call external tools API for linting of the OAS URL
	if err := s.withRateLimit(ctx); err != nil {
		return err
	}
	if current.OasHash != expectedHash || current.AdrScore == nil {
		previousHash := current.OasHash
		previousScore := current.AdrScore
		slog.DebugContext(
			ctx,
			"requesting OAS lint",
			"component", "lint",
			"operation", "lint",
			"api_id", apiID,
		)
		dto, lintErr := toolslint.LintGet(ctx, input)
		if dto == nil {
			return lintErr
		}

		// Map DTO to our persistence model
		msgs := make([]models.LintMessage, 0, len(dto.Messages))
		var errCount, warnCount int
		for _, m := range dto.Messages {
			if strings.ToLower(m.Severity) == "error" {
				errCount++
			} else if strings.ToLower(m.Severity) == "warning" {
				warnCount++
			}
			infos := make([]models.LintMessageInfo, 0, len(m.Infos))
			for _, i := range m.Infos {
				infos = append(infos, models.LintMessageInfo(i))
			}
			id := m.ID
			if strings.TrimSpace(id) == "" {
				id = uuid.New().String()
			}
			msgs = append(msgs, models.LintMessage{
				ID:             id,
				Severity:       m.Severity,
				Code:           m.Code,
				RulesetVersion: dto.RulesetVersion,
				Infos:          infos,
				CreatedAt:      m.CreatedAt,
			})
		}
		score := dto.Score
		slog.DebugContext(
			ctx,
			"OAS lint result prepared",
			"component", "lint",
			"operation", "persist_result",
			"api_id", apiID,
			"message_count", len(msgs),
			"error_count", errCount,
			"warning_count", warnCount,
			"score", score,
		)

		rid := dto.ID
		if strings.TrimSpace(rid) == "" {
			rid = uuid.New().String()
		}
		res := &models.LintResult{
			ID:        rid,
			ApiID:     apiID,
			Successes: dto.Successes,
			Failures:  dto.Failures,
			Warnings:  dto.Warnings,
			Messages:  msgs,
			CreatedAt: dto.CreatedAt,
		}
		if err := s.repo.SaveLintResult(ctx, res); err != nil {
			return err
		}
		slog.DebugContext(
			ctx,
			"OAS lint result stored",
			"component", "lint",
			"operation", "persist_result",
			"api_id", apiID,
			"result_id", res.ID,
		)

		// ✅ Hash en score wegschrijven
		current.AdrScore = &score
		current.OasHash = expectedHash
		if err := s.repo.UpdateApi(ctx, *current); err != nil {
			return err
		}
		s.recordADRScoreChange(ctx, previousScore, current.AdrScore, apiID)
		s.recordOASHashChange(ctx, apiID, previousHash, expectedHash)
		slog.DebugContext(
			ctx,
			"API lint state updated",
			"component", "lint",
			"operation", "update_api",
			"api_id", apiID,
			"score", score,
			"oas_hash", expectedHash,
		)
	}
	return nil
}

// runToolsAndPersist runs lint and postman generation
// and persists their outputs. Lint result + ADR score are stored as before;
// Postman artifacts are stored as blobs linked to the API.
func (s *APIsAPIService) runToolsAndPersist(ctx context.Context, apiID string, oasInput toolslint.OASInput, arazzoInput toolslint.ArazzoInput, result *openapi.OASResult) error {
	if err := s.lintAndPersist(ctx, apiID, oasInput, result.Hash); err != nil {
		slog.ErrorContext(
			ctx,
			"OAS lint processing failed",
			"component", "tools",
			"operation", "lint",
			"api_id", apiID,
			"error", err,
		)
	}

	if err := s.persistOASArtifacts(ctx, apiID, result); err != nil {
		slog.ErrorContext(
			ctx,
			"failed to persist OAS artifacts",
			"component", "tools",
			"operation", "persist_oas_artifacts",
			"api_id", apiID,
			"error", err,
		)
	}

	g, ctx := errgroup.WithContext(ctx)

	g.Go(func() error {
		if err := s.withRateLimit(ctx); err != nil {
			return nil
		}
		data, name, ct, err := toolslint.PostmanPost(ctx, oasInput)
		if err != nil {
			slog.ErrorContext(
				ctx,
				"Postman generation failed",
				"component", "tools",
				"operation", "generate_postman",
				"api_id", apiID,
				"error", err,
			)
			return nil
		}
		art := &models.ApiArtifact{
			ID:          uuid.New().String(),
			ApiID:       apiID,
			Kind:        "postman",
			Filename:    name,
			ContentType: ct,
			Data:        data,
			CreatedAt:   time.Now(),
		}
		if err := s.repo.SaveArtifact(ctx, art); err != nil {
			slog.ErrorContext(
				ctx,
				"failed to store Postman artifact",
				"component", "tools",
				"operation", "persist_postman",
				"api_id", apiID,
				"error", err,
			)
		} else {
			slog.DebugContext(
				ctx,
				"Postman artifact stored",
				"component", "tools",
				"operation", "persist_postman",
				"api_id", apiID,
				"artifact_id", art.ID,
			)
			if err := s.repo.DeleteArtifactsByKind(ctx, apiID, "postman", []string{art.ID}); err != nil {
				slog.WarnContext(
					ctx,
					"failed to clean up old Postman artifacts",
					"component", "tools",
					"operation", "cleanup_postman",
					"api_id", apiID,
					"error", err,
				)
			}
		}
		return nil
	})

	if !arazzoInput.IsEmpty() {
		arazzoInput.Normalize()
		g.Go(func() error {
			if err := s.withRateLimit(ctx); err != nil {
				return nil
			}
			data, ct, err := toolslint.ArazzoMarkdown(ctx, arazzoInput)
			if err != nil {
				slog.ErrorContext(
					ctx,
					"Arazzo Markdown generation failed",
					"component", "tools",
					"operation", "generate_arazzo_markdown",
					"api_id", apiID,
					"error", err,
				)
				return nil
			}
			art := &models.ApiArtifact{
				ID:          uuid.New().String(),
				ApiID:       apiID,
				Kind:        "arazzo_markdown",
				Format:      "markdown",
				Source:      "tools",
				Filename:    "arazzo.md",
				ContentType: ct,
				Data:        data,
				CreatedAt:   time.Now(),
			}
			if err := s.repo.SaveArtifact(ctx, art); err != nil {
				slog.ErrorContext(
					ctx,
					"failed to store Arazzo Markdown artifact",
					"component", "tools",
					"operation", "persist_arazzo_markdown",
					"api_id", apiID,
					"error", err,
				)
			} else {
				slog.DebugContext(
					ctx,
					"Arazzo Markdown artifact stored",
					"component", "tools",
					"operation", "persist_arazzo_markdown",
					"api_id", apiID,
					"artifact_id", art.ID,
				)
			}
			return nil
		})

		g.Go(func() error {
			if err := s.withRateLimit(ctx); err != nil {
				return nil
			}
			data, ct, err := toolslint.ArazzoMermaid(ctx, arazzoInput)
			if err != nil {
				slog.ErrorContext(
					ctx,
					"Arazzo Mermaid generation failed",
					"component", "tools",
					"operation", "generate_arazzo_mermaid",
					"api_id", apiID,
					"error", err,
				)
				return nil
			}
			art := &models.ApiArtifact{
				ID:          uuid.New().String(),
				ApiID:       apiID,
				Kind:        "arazzo_mermaid",
				Format:      "mermaid",
				Source:      "tools",
				Filename:    "arazzo.mmd",
				ContentType: ct,
				Data:        data,
				CreatedAt:   time.Now(),
			}
			if err := s.repo.SaveArtifact(ctx, art); err != nil {
				slog.ErrorContext(
					ctx,
					"failed to store Arazzo Mermaid artifact",
					"component", "tools",
					"operation", "persist_arazzo_mermaid",
					"api_id", apiID,
					"error", err,
				)
			} else {
				slog.DebugContext(
					ctx,
					"Arazzo Mermaid artifact stored",
					"component", "tools",
					"operation", "persist_arazzo_mermaid",
					"api_id", apiID,
					"artifact_id", art.ID,
				)
			}
			return nil
		})
	}

	_ = g.Wait()
	return nil
}

func (s *APIsAPIService) publishToTypesense(api models.Api) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := typesense.PublishApi(ctx, &api); err != nil {
		if errors.Is(err, typesense.ErrDisabled) {
			return
		}
		slog.ErrorContext(
			ctx,
			"Typesense indexing failed",
			"component", "typesense",
			"operation", "index_api",
			"api_id", api.Id,
			"error", err,
		)
	}
}

func (s *APIsAPIService) ListOrganisations(ctx context.Context) ([]models.Organisation, int, error) {
	return s.repo.GetOrganisations(ctx)
}

// PublishAllApisToTypesense pushes every stored API to Typesense. Intended as a one-off helper.
func (s *APIsAPIService) PublishAllApisToTypesense(ctx context.Context) error {
	if !typesense.Enabled() {
		slog.InfoContext(
			ctx,
			"Typesense indexing disabled; skipping bulk publish",
			"component", "typesense",
			"operation", "bulk_index",
		)
		return nil
	}

	apis, err := s.repo.AllApis(ctx)
	if err != nil {
		return err
	}

	for _, api := range apis {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		apiCopy := api
		itemCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		err := typesense.PublishApi(itemCtx, &apiCopy)
		cancel()
		if err != nil {
			if errors.Is(err, typesense.ErrDisabled) {
				slog.InfoContext(
					ctx,
					"Typesense indexing disabled during bulk publish",
					"component", "typesense",
					"operation", "bulk_index",
				)
				return nil
			}
			slog.ErrorContext(
				ctx,
				"Typesense bulk indexing failed for API",
				"component", "typesense",
				"operation", "bulk_index",
				"api_id", apiCopy.Id,
				"error", err,
			)
		}
	}
	return nil
}

// CreateOrganisation validates and stores a new organisation
func (s *APIsAPIService) CreateOrganisation(ctx context.Context, org *models.Organisation) (*models.Organisation, error) {
	if _, err := url.ParseRequestURI(org.Uri); err != nil {
		return nil, problem.NewBadRequest(org.Uri, fmt.Sprintf("foutieve uri: %v", err),
			problem.InvalidParam{Name: "uri", Reason: "Moet een geldige URL zijn"})
	}
	if lbl, err := httpclient.FetchOrganisationLabel(ctx, org.Uri); err == nil && strings.TrimSpace(lbl) != "" {
		org.Label = lbl
	}
	if strings.TrimSpace(org.Label) == "" {
		return nil, problem.NewBadRequest(org.Uri, "label is verplicht wanneer TOOI geen label levert",
			problem.InvalidParam{Name: "label", Reason: "Vul label in wanneer TOOI geen label oplevert"})
	}
	existing, err := s.repo.FindOrganisationByURI(ctx, org.Uri)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return nil, problem.New(http.StatusConflict, "Organisation already exists",
			bodyError("uri", "conflict", "organisation already exists"),
		)
	}
	if err := s.repo.SaveOrganisatie(org); err != nil {
		return nil, err
	}
	return org, nil
}

// GetArtifact retrieves the latest artifact for an API and kind.
func (s *APIsAPIService) GetArtifact(ctx context.Context, apiID, kind string) (*models.ApiArtifact, error) {
	if strings.TrimSpace(apiID) == "" || strings.TrimSpace(kind) == "" {
		return nil, fmt.Errorf("apiID en kind zijn verplicht")
	}
	return s.repo.GetArtifact(ctx, apiID, kind)
}

func (s *APIsAPIService) GetOasDocument(ctx context.Context, apiID, version, format string) (*models.ApiArtifact, error) {
	if strings.TrimSpace(apiID) == "" {
		return nil, fmt.Errorf("apiID is verplicht")
	}
	version = strings.TrimSpace(version)
	if version != "3.0" && version != "3.1" {
		return nil, problem.NewBadRequest(version, "ondersteunde versies zijn 3.0 en 3.1")
	}
	format = strings.ToLower(strings.TrimSpace(format))
	if format == "yml" {
		format = "yaml"
	}
	if format != "json" && format != "yaml" {
		return nil, problem.NewBadRequest(format, "ondersteunde extensies zijn json en yaml")
	}
	art, err := s.repo.GetOasArtifact(ctx, apiID, version, format)
	if err != nil {
		return nil, err
	}
	return art, nil
}

func (s *APIsAPIService) persistOASArtifacts(ctx context.Context, apiID string, res *openapi.OASResult) error {
	if res == nil {
		return errors.New("leeg OAS resultaat")
	}
	if len(res.Raw) == 0 {
		return errors.New("OAS bytes ontbreken")
	}

	originalVersion := fmt.Sprintf("%d.%d", res.Major, res.Minor)
	originalFormat, err := detectOASFormat(res.Raw, res.ContentType)
	if err != nil {
		return fmt.Errorf("kan formaat niet bepalen: %w", err)
	}

	canonicalJSON, err := renderCanonicalJSON(res, originalFormat)
	if err != nil {
		return fmt.Errorf("kan canonical JSON renderen: %w", err)
	}

	var (
		errs []error
		keep []string
	)
	// Bewaar ongewijzigde originele spec
	if id, err := s.saveOASArtifact(ctx, apiID, originalVersion, originalFormat, "original", res.Raw); err != nil {
		errs = append(errs, err)
	} else {
		keep = append(keep, id)
	}

	// Zorg dat dezelfde versie ook in de andere representatie beschikbaar is
	if originalFormat != "json" {
		if id, err := s.saveOASArtifact(ctx, apiID, originalVersion, "json", "converted", canonicalJSON); err != nil {
			errs = append(errs, err)
		} else {
			keep = append(keep, id)
		}
	}
	if originalFormat != "yaml" {
		yamlData, yErr := yaml.JSONToYAML(canonicalJSON)
		if yErr != nil {
			errs = append(errs, fmt.Errorf("kan YAML renderen voor versie %s: %w", originalVersion, yErr))
		} else if id, err := s.saveOASArtifact(ctx, apiID, originalVersion, "yaml", "converted", yamlData); err != nil {
			errs = append(errs, err)
		} else {
			keep = append(keep, id)
		}
	}

	// Converteer naar de andere OpenAPI versie indien ondersteund (3.0 <-> 3.1)
	if targetShort, targetFull, ok := targetVersion(res); ok {
		convertedJSON, convErr := updateOpenAPIVersion(canonicalJSON, targetFull)
		if convErr != nil {
			errs = append(errs, fmt.Errorf("kan OAS versie converteren naar %s: %w", targetFull, convErr))
		} else {
			if id, err := s.saveOASArtifact(ctx, apiID, targetShort, "json", "converted", convertedJSON); err != nil {
				errs = append(errs, err)
			} else {
				keep = append(keep, id)
			}
			yamlData, yErr := yaml.JSONToYAML(convertedJSON)
			if yErr != nil {
				errs = append(errs, fmt.Errorf("kan YAML renderen voor versie %s: %w", targetShort, yErr))
			} else if id, err := s.saveOASArtifact(ctx, apiID, targetShort, "yaml", "converted", yamlData); err != nil {
				errs = append(errs, err)
			} else {
				keep = append(keep, id)
			}
		}
	} else {
		slog.DebugContext(
			ctx,
			"automatic OAS conversion not supported for version",
			"component", "openapi",
			"operation", "convert",
			"api_id", apiID,
			"oas_version", res.Version,
		)
	}

	if len(errs) == 0 && len(keep) > 0 {
		if err := s.repo.DeleteArtifactsByKind(ctx, apiID, "oas", keep); err != nil {
			errs = append(errs, fmt.Errorf("kan oude OAS artifacts niet verwijderen: %w", err))
		}
	}

	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	return nil
}

func (s *APIsAPIService) saveOASArtifact(ctx context.Context, apiID, version, format, source string, data []byte) (string, error) {
	if len(data) == 0 {
		return "", fmt.Errorf("artifact data is leeg voor versie %s (%s)", version, format)
	}
	format = strings.ToLower(format)
	art := &models.ApiArtifact{
		ID:          uuid.New().String(),
		ApiID:       apiID,
		Kind:        "oas",
		Version:     version,
		Format:      format,
		Source:      source,
		Filename:    oasFilename(version, source, format),
		ContentType: formatContentType(format),
		Data:        data,
		CreatedAt:   time.Now(),
	}
	if err := s.repo.SaveArtifact(ctx, art); err != nil {
		return "", fmt.Errorf("kan artifact %s opslaan: %w", art.Filename, err)
	}
	slog.DebugContext(
		ctx,
		"OAS artifact stored",
		"component", "openapi",
		"operation", "persist_artifact",
		"api_id", apiID,
		"artifact_id", art.ID,
		"oas_version", version,
		"format", format,
		"source", source,
	)
	return art.ID, nil
}

// BackfillOASArtifacts (éénmalig) genereert OAS-artifacts voor bestaande APIs
// die zijn aangemaakt vóórdat de nieuwe artifact-structuur bestond.
func (s *APIsAPIService) BackfillOASArtifacts(ctx context.Context) error {
	apis, err := s.repo.AllApis(ctx)
	if err != nil {
		return err
	}

	for _, api := range apis {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		has, err := s.repo.HasArtifactOfKind(ctx, api.Id, "oas")
		if err != nil {
			return err
		}
		if has {
			continue
		}

		if err := s.withRateLimit(ctx); err != nil {
			return err
		}
		oasInput := toolslint.OASInput{OasUrl: api.OasUri}
		var artifactResult *openapi.OASResult
		err = openapi.ProcessOAS(ctx, oasInput, openapi.FetchOpts{
			Origin: "https://developer.overheid.nl",
		}, func(res *openapi.OASResult) error {
			applyOASSnapshot(&api, res)
			if api.OasHash != res.Hash {
				api.OasHash = res.Hash
			}
			artifactResult = detachedOASResult(res)
			return nil
		})
		if err != nil {
			slog.WarnContext(
				ctx,
				"skipping OAS artifact backfill for unavailable OAS",
				"component", "oas_backfill",
				"operation", "fetch_oas",
				"api_id", api.Id,
				"error", err,
			)
			continue
		}
		if err := s.persistOASArtifacts(ctx, api.Id, artifactResult); err != nil {
			slog.ErrorContext(
				ctx,
				"failed to persist backfilled OAS artifacts",
				"component", "oas_backfill",
				"operation", "persist_artifacts",
				"api_id", api.Id,
				"error", err,
			)
		}
		if err := s.repo.UpdateApi(ctx, api); err != nil {
			slog.ErrorContext(
				ctx,
				"failed to update API after OAS artifact backfill",
				"component", "oas_backfill",
				"operation", "update_api",
				"api_id", api.Id,
				"error", err,
			)
		}
	}

	return nil
}

func applyOASSnapshot(api *models.Api, res *openapi.OASResult) {
	if api == nil || res == nil {
		return
	}
	api.OAS = deriveOASSnapshot(api.OAS, res)
}

func detachedOASResult(res *openapi.OASResult) *openapi.OASResult {
	if res == nil {
		return nil
	}
	detached := *res
	detached.Spec = nil
	return &detached
}

func deriveOASSnapshot(current models.OASMetadata, res *openapi.OASResult) models.OASMetadata {
	if res == nil {
		return current
	}
	current.Version = strings.TrimSpace(res.Version)
	current.Status = models.OASStatusValid
	current.Auth = strings.TrimSpace(openapi.AuthTypeFromSpec(res.Spec))
	return current
}

func currentOASAuth(api models.Api) string {
	if auth := strings.TrimSpace(api.OAS.Auth); auth != "" {
		return auth
	}
	return strings.TrimSpace(api.Auth)
}

func (s *APIsAPIService) retireAPIForUnavailableOAS(ctx context.Context, api models.Api) (bool, error) {
	if strings.TrimSpace(api.Id) == "" {
		return false, nil
	}
	full, err := s.repo.GetApiByID(ctx, api.Id)
	if err != nil {
		return false, err
	}
	if full == nil {
		return false, nil
	}

	nextOAS := models.OASMetadata{
		Version: full.OAS.Version,
		Status:  models.OASStatusUnreachable,
		Auth:    currentOASAuth(*full),
	}
	if err := s.updateOASMetadataSnapshot(ctx, full.Id, full.OAS, nextOAS); err != nil {
		return false, err
	}

	now := time.Now()
	if full.LifecycleStatus(now) == "retired" {
		return false, nil
	}
	full.Sunset = retiredLifecycleDate(now)
	if full.Deprecated == "" {
		full.Deprecated = full.Sunset
	}
	if err := s.repo.UpdateApi(ctx, *full); err != nil {
		return false, err
	}
	return true, nil
}

func retiredLifecycleDate(now time.Time) string {
	return now.AddDate(0, 0, -1).Format(time.DateOnly)
}

func classifyOASStatus(err error) string {
	if err == nil {
		return models.OASStatusValid
	}
	msg := strings.ToLower(err.Error())
	switch {
	case strings.Contains(msg, "kan oas niet ophalen"),
		strings.Contains(msg, "kan oas niet lezen"),
		strings.Contains(msg, "geen oasurl opgegeven"):
		return models.OASStatusUnreachable
	case strings.Contains(msg, "invalid oas"):
		return models.OASStatusInvalid
	default:
		return models.OASStatusUnknown
	}
}

func (s *APIsAPIService) updateOASMetadataSnapshot(ctx context.Context, apiID string, current, next models.OASMetadata) error {
	current.Version = strings.TrimSpace(current.Version)
	current.Status = strings.TrimSpace(current.Status)
	current.Auth = strings.TrimSpace(current.Auth)
	next.Version = strings.TrimSpace(next.Version)
	next.Status = strings.TrimSpace(next.Status)
	next.Auth = strings.TrimSpace(next.Auth)
	if current == next {
		return nil
	}
	return s.repo.UpdateOASMetadata(ctx, apiID, next)
}

func bodyError(field, code, detail string) problem.ErrorDetail {
	return problem.ErrorDetail{
		In:       "body",
		Location: fmt.Sprintf("#/%s", field),
		Code:     code,
		Detail:   detail,
	}
}

var apiFeedEventTexts = map[string]models.ApiFeedEventText{
	models.ApiFeedEventLifecycleChanged: {
		Title:       "Lifecycle gewijzigd",
		Description: "Lifecycle status is gewijzigd van `{old}` naar `{new}`.",
	},
	models.ApiFeedEventADRScoreChanged: {
		Title:       "ADR-score gewijzigd",
		Description: "ADR-score wijzigde van {old} naar {new}.",
	},
	models.ApiFeedEventOASHashChanged: {
		Title:       "OpenAPI-specificatie gewijzigd",
		Description: "De inhoud van de OpenAPI-specificatie is gewijzigd.",
	},
	models.ApiFeedEventOASUnavailable: {
		Title:       "OpenAPI-specificatie niet beschikbaar",
		Description: "De OpenAPI-specificatie kon tijdens de dagelijkse controle niet worden opgehaald.",
	},
}

func (s *APIsAPIService) recordLifecycleChange(ctx context.Context, before, after models.Api) {
	if before.Sunset == after.Sunset && before.Deprecated == after.Deprecated {
		return
	}
	now := time.Now()
	oldStatus := before.LifecycleStatus(now)
	newStatus := after.LifecycleStatus(now)
	oldValue := lifecycleFeedValue(before, after, oldStatus)
	newValue := lifecycleFeedValue(after, before, newStatus)
	s.saveFeedEvent(ctx, after.Id, models.ApiFeedEventLifecycleChanged, oldValue, newValue)
}

func (s *APIsAPIService) recordADRScoreChange(ctx context.Context, before, after *int, apiID string) {
	if scorePtrValue(before) == scorePtrValue(after) {
		return
	}
	oldValue := scorePtrValue(before)
	newValue := scorePtrValue(after)
	s.saveFeedEvent(ctx, apiID, models.ApiFeedEventADRScoreChanged, oldValue, newValue)
}

func (s *APIsAPIService) recordOASHashChange(ctx context.Context, apiID, before, after string) {
	before = strings.TrimSpace(before)
	after = strings.TrimSpace(after)
	if before == after {
		return
	}
	s.saveFeedEvent(ctx, apiID, models.ApiFeedEventOASHashChanged, before, after)
}

func (s *APIsAPIService) recordOASUnavailable(ctx context.Context, apiID string, before, after models.OASMetadata) {
	beforeStatus := strings.TrimSpace(before.Status)
	afterStatus := strings.TrimSpace(after.Status)
	if afterStatus != models.OASStatusUnreachable || beforeStatus == afterStatus {
		return
	}
	oldValue := beforeStatus
	if oldValue == "" {
		oldValue = models.OASStatusUnknown
	}
	s.saveFeedEvent(ctx, apiID, models.ApiFeedEventOASUnavailable, oldValue, afterStatus)
}

func (s *APIsAPIService) saveFeedEvent(ctx context.Context, apiID, eventType, oldValue, newValue string) {
	if strings.TrimSpace(apiID) == "" {
		return
	}
	title, description := feedEventText(eventType, oldValue, newValue)
	event := &models.ApiFeedEvent{
		ID:          uuid.New().String(),
		ApiID:       apiID,
		Type:        eventType,
		Title:       title,
		Description: description,
		OldValue:    oldValue,
		NewValue:    newValue,
		CreatedAt:   time.Now(),
	}
	if err := s.repo.SaveApiFeedEvent(ctx, event); err != nil {
		slog.ErrorContext(
			ctx,
			"failed to store API feed event",
			"component", "api_feed",
			"operation", "persist_event",
			"api_id", apiID,
			"event_type", eventType,
			"error", err,
		)
	}
}

func feedEventText(eventType, oldValue, newValue string) (string, string) {
	text, ok := apiFeedEventTexts[eventType]
	if !ok {
		return eventType, ""
	}
	description := strings.NewReplacer(
		"{old}", oldValue,
		"{new}", newValue,
	).Replace(text.Description)
	return text.Title, description
}

func lifecycleFeedValue(api, compare models.Api, status string) string {
	parts := []string{status}
	if api.Deprecated != compare.Deprecated && strings.TrimSpace(api.Deprecated) != "" {
		parts = append(parts, "deprecated="+strings.TrimSpace(api.Deprecated))
	}
	if api.Sunset != compare.Sunset && strings.TrimSpace(api.Sunset) != "" {
		parts = append(parts, "sunset="+strings.TrimSpace(api.Sunset))
	}
	return strings.Join(parts, ", ")
}

func scorePtrValue(score *int) string {
	if score == nil {
		return "unknown"
	}
	return strconv.Itoa(*score)
}

func deriveOrganisationURI(api *models.Api) string {
	if api == nil {
		return ""
	}
	if api.OrganisationID != nil && strings.TrimSpace(*api.OrganisationID) != "" {
		return strings.TrimSpace(*api.OrganisationID)
	}
	if api.Organisation != nil && strings.TrimSpace(api.Organisation.Uri) != "" {
		return strings.TrimSpace(api.Organisation.Uri)
	}
	return ""
}

func hasOASUpdateInput(body *models.UpdateApiInput) bool {
	if body == nil {
		return false
	}
	return strings.TrimSpace(body.OasUrl) != "" || strings.TrimSpace(body.OasBody) != ""
}

func hasOASDocumentChange(api *models.Api, body *models.UpdateApiInput) bool {
	if body == nil {
		return false
	}
	if strings.TrimSpace(body.OasBody) != "" {
		return true
	}
	requestedURL := strings.TrimSpace(body.OasUrl)
	if requestedURL == "" {
		return false
	}
	if api == nil {
		return true
	}
	return requestedURL != strings.TrimSpace(api.OasUri)
}

func validateLifecycleOverrides(body *models.UpdateApiInput) error {
	if body == nil {
		return nil
	}
	if err := validateLifecycleDate("sunset", body.Sunset); err != nil {
		return err
	}
	if err := validateLifecycleDate("deprecated", body.Deprecated); err != nil {
		return err
	}
	return nil
}

func validateLifecycleDate(name string, value models.OptionalString) error {
	if !value.Set || value.Value == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*value.Value)
	if _, err := time.Parse(time.DateOnly, trimmed); err != nil {
		return problem.NewBadRequest(trimmed, fmt.Sprintf("%s moet in formaat YYYY-MM-DD zijn", name),
			problem.InvalidParam{Name: name, Reason: "Moet een geldige datum zijn (YYYY-MM-DD)"},
		)
	}
	return nil
}

func applyLifecycleOverrides(api *models.Api, body *models.UpdateApiInput) {
	if api == nil || body == nil {
		return
	}
	if body.Sunset.Set {
		if body.Sunset.Value == nil {
			api.Sunset = ""
		} else {
			api.Sunset = strings.TrimSpace(*body.Sunset.Value)
		}
	}
	if body.Deprecated.Set {
		if body.Deprecated.Value == nil {
			api.Deprecated = ""
		} else {
			api.Deprecated = strings.TrimSpace(*body.Deprecated.Value)
		}
	}
}

func toOASInput(post models.ApiPost) toolslint.OASInput {
	return toolslint.OASInput{
		OasUrl:  strings.TrimSpace(post.OasUrl),
		OasBody: post.OasBody,
	}
}

func toArazzoInput(post models.ApiPost) toolslint.ArazzoInput {
	return toolslint.ArazzoInput{
		ArazzoUrl:  strings.TrimSpace(post.ArazzoUrl),
		ArazzoBody: post.ArazzoBody,
	}
}

func detectOASFormat(raw []byte, contentType string) (string, error) {
	ct := strings.ToLower(contentType)
	switch {
	case strings.Contains(ct, "json"):
		return "json", nil
	case strings.Contains(ct, "yaml"), strings.Contains(ct, "yml"):
		return "yaml", nil
	}
	sample := raw
	if len(sample) > 256 {
		sample = sample[:256]
	}
	trimmed := strings.TrimSpace(string(sample))
	if trimmed == "" {
		return "", errors.New("leeg document")
	}
	switch trimmed[0] {
	case '{', '[':
		return "json", nil
	}
	if strings.HasPrefix(strings.ToLower(trimmed), "openapi:") || strings.HasPrefix(trimmed, "---") {
		return "yaml", nil
	}
	return "", fmt.Errorf("onbekend formaat")
}

func formatContentType(format string) string {
	switch strings.ToLower(format) {
	case "json":
		return "application/json"
	case "yaml", "yml":
		return "application/yaml"
	default:
		return "application/octet-stream"
	}
}

func oasFilename(version, source, format string) string {
	return fmt.Sprintf("oas-%s-%s.%s", version, source, format)
}

func renderCanonicalJSON(res *openapi.OASResult, originalFormat string) ([]byte, error) {
	if len(res.CanonicalJSON) > 0 {
		return res.CanonicalJSON, nil
	}
	if res.Spec != nil {
		if rendered, err := res.Spec.RenderJSON("  "); err == nil && len(rendered) > 0 {
			return rendered, nil
		}
	}
	return toPrettyJSON(res.Raw, originalFormat)
}

func prettyJSON(data []byte) ([]byte, error) {
	var buf bytes.Buffer
	if err := json.Indent(&buf, data, "", "  "); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func toPrettyJSON(raw []byte, format string) ([]byte, error) {
	switch strings.ToLower(format) {
	case "json":
		return prettyJSON(raw)
	case "yaml", "yml":
		jsonData, err := yaml.YAMLToJSON(raw)
		if err != nil {
			return nil, err
		}
		return prettyJSON(jsonData)
	default:
		return nil, fmt.Errorf("onbekend formaat %s", format)
	}
}

func targetVersion(res *openapi.OASResult) (short string, full string, ok bool) {
	switch res.Minor {
	case 0:
		return "3.1", "3.1.0", true
	case 1:
		return "3.0", "3.0.3", true
	default:
		return "", "", false
	}
}

func updateOpenAPIVersion(jsonData []byte, targetVersion string) ([]byte, error) {
	var doc map[string]json.RawMessage
	if err := json.Unmarshal(jsonData, &doc); err != nil {
		return nil, err
	}
	versionJSON, err := json.Marshal(targetVersion)
	if err != nil {
		return nil, err
	}
	doc["openapi"] = versionJSON
	if strings.HasPrefix(targetVersion, "3.0") {
		delete(doc, "webhooks")
		delete(doc, "jsonSchemaDialect")
		delete(doc, "$self")
	}
	out, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return nil, err
	}
	return out, nil
}

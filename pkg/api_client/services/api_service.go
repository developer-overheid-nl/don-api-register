package services

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/url"
	"strings"
	"time"

	httpclient "github.com/developer-overheid-nl/don-api-register/pkg/api_client/helpers/httpclient"
	openapi "github.com/developer-overheid-nl/don-api-register/pkg/api_client/helpers/openapi"
	problem "github.com/developer-overheid-nl/don-api-register/pkg/api_client/helpers/problem"
	toolslint "github.com/developer-overheid-nl/don-api-register/pkg/api_client/helpers/tools"
	util "github.com/developer-overheid-nl/don-api-register/pkg/api_client/helpers/util"
	"github.com/developer-overheid-nl/don-api-register/pkg/api_client/models"
	"github.com/developer-overheid-nl/don-api-register/pkg/api_client/repositories"
	"github.com/developer-overheid-nl/don-api-register/pkg/tools"
	"github.com/google/uuid"
	"golang.org/x/sync/errgroup"
	"golang.org/x/sync/semaphore"
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
	api, err := s.repo.GetApiByID(ctx, body.Id)
	if err != nil || api == nil {
		if api == nil || errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("%w: %s", ErrNeedsPost, body.OasUrl)
		}
		return nil, fmt.Errorf("databasefout: %w", err)
	}
	if api.OrganisationID == nil || (*api.Organisation).Uri != body.OrganisationUri {
		return nil, problem.NewForbidden(body.OasUrl, "organisationUri komt niet overeen met eigenaar van deze API")
	}

	// → nieuwe OAS strict valideren + hash
	res, err := openapi.FetchParseValidateAndHash(ctx, body.OasUrl, openapi.FetchOpts{
		Origin: "https://developer.overheid.nl",
	})
	if err != nil {
		return nil, problem.NewBadRequest(body.OasUrl, err.Error())
	}

	// Niks veranderd? Klaar.
	if res.Hash == api.OasHash {
		updated := util.ToApiSummary(api)
		return &updated, nil
	}

	// Hash gewijzigd → opslaan en linten
	api.OasHash = res.Hash
	if err := s.repo.UpdateApi(ctx, *api); err != nil {
		return nil, err
	}

	tools.Dispatch(context.Background(), "tools", func(ctx context.Context) error {
		return s.runToolsAndPersist(ctx, api.Id, body.OasUrl, res.Hash)
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

	lintResults, err := s.repo.GetLintResults(ctx, api.Id)
	if err != nil {
		return nil, err
	}
	detail.LintResults = lintResults

	return detail, nil
}

func (s *APIsAPIService) ListApis(ctx context.Context, p *models.ListApisParams) ([]models.ApiSummary, models.Pagination, error) {
	apis, pagination, err := s.repo.GetApis(ctx, p.Page, p.PerPage, p.Organisation, p.Ids)
	if err != nil {
		return nil, models.Pagination{}, err
	}

	dtos := make([]models.ApiSummary, len(apis))
	for i, api := range apis {
		dtos[i] = util.ToApiSummary(&api)
	}

	return dtos, pagination, nil
}

func (s *APIsAPIService) UpdateApi(ctx context.Context, api models.Api) error {
	return s.repo.UpdateApi(ctx, api)
}

func (s *APIsAPIService) CreateApiFromOas(requestBody models.ApiPost) (*models.ApiSummary, error) {
	ctx := context.Background()

	// 1) Strict validate + hash
	resp, err := openapi.FetchParseValidateAndHash(ctx, requestBody.OasUrl, openapi.FetchOpts{
		Origin: "https://developer.overheid.nl",
	})
	if err != nil {
		return nil, problem.NewBadRequest(requestBody.OasUrl, err.Error())
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
	api := openapi.BuildApi(resp.Spec, requestBody, label)
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

	api.OasHash = resp.Hash
	if err := s.repo.UpdateApi(ctx, *api); err != nil {
		return nil, problem.NewInternalServerError("kan API hash niet opslaan: " + err.Error())
	}

	tools.Dispatch(context.Background(), "tools", func(ctx context.Context) error {
		return s.runToolsAndPersist(ctx, api.Id, requestBody.OasUrl, resp.Hash)
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

	const maxConcurrent = 2
	sem := semaphore.NewWeighted(int64(maxConcurrent))
	g, ctx := errgroup.WithContext(ctx)

	for _, api := range apis {
		api := api // capture

		// Bereken hash alvast (en respecteer job-context)
		resp, err := openapi.FetchParseValidateAndHash(ctx, api.OasUri, openapi.FetchOpts{Origin: "https://developer.overheid.nl"})
		if err != nil {
			// Loggen en doorgaan – één kapotte API mag de rest niet blokkeren
			log.Printf("[lint] skip api=%s: hash fetch failed: %v", api.Id, err)
			continue
		}

		if err := sem.Acquire(ctx, 1); err != nil {
			return err
		}
		g.Go(func() error {
			defer sem.Release(1)

			// Per-lint timeout (overschrijft niet je job-timeout)
			lintCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
			defer cancel()

			return s.lintAndPersist(lintCtx, api.Id, api.OasUri, resp.Hash)
		})
	}

	return g.Wait()
}

// lintAndPersist runs the linter when the OAS has changed and stores
// both the lint result and updated hash.
func (s *APIsAPIService) lintAndPersist(ctx context.Context, apiID, oasURL, expectedHash string) error {
	current, err := s.repo.GetApiByID(ctx, apiID)
	if err != nil || current == nil {
		return err
	}

	log.Printf("[lint] api=%s expectedHash=%s currentHash=%s", apiID, expectedHash, current.OasHash)
	if current.OasHash == expectedHash || current.AdrScore == nil {
		log.Printf("[lint] hash unchanged; proceeding to fetch tools lint for score update")
	}

	// Call external tools API for linting of the OAS URL
	log.Printf("[lint] calling tools lint for url=%s", oasURL)
	dto, lintErr := toolslint.LintGet(ctx, oasURL)
	if dto == nil {
		// Avoid panic on nil; log and return error to caller
		if lintErr != nil {
			log.Printf("[lint] tools lint error: %v", lintErr)
		}
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
			infos = append(infos, models.LintMessageInfo{
				ID:            i.ID,
				LintMessageID: i.LintMessageID,
				Message:       i.Message,
				Path:          i.Path,
			})
		}
		id := m.ID
		if strings.TrimSpace(id) == "" {
			id = uuid.New().String()
		}
		msgs = append(msgs, models.LintMessage{
			ID:        id,
			Severity:  m.Severity,
			Code:      m.Code,
			Infos:     infos,
			CreatedAt: m.CreatedAt,
		})
	}
	score := dto.Score
	log.Printf("[lint] messages=%d errors=%d warnings=%d score=%d", len(msgs), errCount, warnCount, score)

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
		log.Printf("[lint] save result failed: %v", err)
		return err
	}
	log.Printf("[lint] saved lint result id=%s", res.ID)

	// ✅ Hash en score wegschrijven
	current.AdrScore = &score
	current.OasHash = expectedHash
	if err := s.repo.UpdateApi(ctx, *current); err != nil {
		log.Printf("[lint] update api failed: %v", err)
		return err
	}
	log.Printf("[lint] updated AdrScore=%d & OasHash=%s api=%s", score, expectedHash, apiID)
	return nil
}

// runToolsAndPersist runs lint, bruno and postman generation
// and persists their outputs. Lint result + ADR score are stored as before;
// Bruno and Postman artifacts are stored as blobs linked to the API.
func (s *APIsAPIService) runToolsAndPersist(ctx context.Context, apiID, oasURL, expectedHash string) error {
	// Lint first; do not abort the rest if lint fails
	if err := s.lintAndPersist(ctx, apiID, oasURL, expectedHash); err != nil {
		log.Printf("[tools] lint failed: %v", err)
	}

	// Generate artifacts in parallel, but do not fail the whole job if one fails
	g, ctx := errgroup.WithContext(ctx)

	g.Go(func() error {
		data, name, ct, err := toolslint.BrunoPost(ctx, oasURL)
		if err != nil {
			log.Printf("[tools] bruno generation failed: %v", err)
			return nil
		}
		art := &models.ApiArtifact{
			ID:          uuid.New().String(),
			ApiID:       apiID,
			Kind:        "bruno",
			Filename:    name,
			ContentType: ct,
			Data:        data,
			CreatedAt:   time.Now(),
		}
		if err := s.repo.SaveArtifact(ctx, art); err != nil {
			log.Printf("[tools] save bruno artifact failed: %v", err)
		} else {
			log.Printf("[tools] saved bruno artifact id=%s api=%s", art.ID, apiID)
		}
		return nil
	})

	g.Go(func() error {
		data, name, ct, err := toolslint.PostmanPost(ctx, oasURL)
		if err != nil {
			log.Printf("[tools] postman generation failed: %v", err)
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
			log.Printf("[tools] save postman artifact failed: %v", err)
		} else {
			log.Printf("[tools] saved postman artifact id=%s api=%s", art.ID, apiID)
		}
		return nil
	})

	g.Go(func() error {
		data, name, ct, err := toolslint.OasConverterPost(ctx, oasURL)
		if err != nil {
			log.Printf("[tools] oas converter failed: %v", err)
			return nil
		}
		art := &models.ApiArtifact{
			ID:          uuid.New().String(),
			ApiID:       apiID,
			Kind:        "oas-converted",
			Filename:    name,
			ContentType: ct,
			Data:        data,
			CreatedAt:   time.Now(),
		}
		if err := s.repo.SaveArtifact(ctx, art); err != nil {
			log.Printf("[tools] save oas-converted artifact failed: %v", err)
		} else {
			log.Printf("[tools] saved oas-converted artifact id=%s api=%s", art.ID, apiID)
		}
		return nil
	})

	_ = g.Wait()
	return nil
}

func (s *APIsAPIService) ListOrganisations(ctx context.Context) ([]models.Organisation, int, error) {
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

// GetArtifact retrieves the latest artifact for an API and kind.
func (s *APIsAPIService) GetArtifact(ctx context.Context, apiID, kind string) (*models.ApiArtifact, error) {
	if strings.TrimSpace(apiID) == "" || strings.TrimSpace(kind) == "" {
		return nil, fmt.Errorf("apiID en kind zijn verplicht")
	}
	return s.repo.GetArtifact(ctx, apiID, kind)
}

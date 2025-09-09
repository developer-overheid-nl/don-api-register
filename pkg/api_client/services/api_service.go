package services

import (
	"context"
	"errors"
	"fmt"
	"log"
	"math"
	"net/url"
	"sort"
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

	tools.Dispatch(context.Background(), "lint", func(ctx context.Context) error {
		return s.lintAndPersist(ctx, api.Id, body.OasUrl, res.Hash)
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

	tools.Dispatch(context.Background(), "lint", func(ctx context.Context) error {
		return s.lintAndPersist(ctx, api.Id, requestBody.OasUrl, resp.Hash)
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

	return g.Wait() // ⬅️ WACHTEN tot alles klaar is
}

var measuredRules = map[string]struct{}{
	"openapi3":                     {},
	"openapi-root-exists":          {},
	"missing-version-header":       {},
	"missing-header":               {},
	"include-major-version-in-uri": {},
	"paths-no-trailing-slash":      {},
	"info-contact-fields-exist":    {},
	"http-methods":                 {},
	"semver":                       {},
}

func computeAdrScore(msgs []models.LintMessage) (score int, failed []string) {
	failedSet := map[string]struct{}{}
	for _, m := range msgs {
		if strings.ToLower(m.Severity) != "error" {
			continue
		}
		if _, ok := measuredRules[m.Code]; ok {
			failedSet[m.Code] = struct{}{}
		}
	}
	for k := range failedSet {
		failed = append(failed, k)
	}
	sort.Strings(failed)

	total := len(measuredRules)
	if total == 0 {
		return 100, failed
	}
	score = int(math.Round((1 - float64(len(failed))/float64(total)) * 100))
	return score, failed
}

// lintAndPersist runs the linter when the OAS has changed and stores
// both the lint result and updated hash.
func (s *APIsAPIService) lintAndPersist(ctx context.Context, apiID, oasURL, expectedHash string) error {
	current, err := s.repo.GetApiByID(ctx, apiID)
	if err != nil || current == nil {
		return err
	}

	log.Printf("[lint] api=%s expectedHash=%s currentHash=%s", apiID, expectedHash, current.OasHash)
	if current.OasHash == expectedHash {
		log.Printf("[lint] skip lint: hash unchanged")
		return nil
	}

	log.Printf("[lint] running spectral for url=%s", oasURL)
	output, lintRunErr := linter.LintURL(ctx, oasURL) // zie stap 3 voor verbeterde LintURL
	now := time.Now()

	// Als Spectral niks heeft teruggegeven, log een synthetische fout zodat je toch historie hebt.
	if strings.TrimSpace(output) == "" && lintRunErr != nil {
		msg := models.LintMessage{
			ID:        uuid.New().String(),
			Code:      "lint-exec",
			Severity:  "error",
			CreatedAt: now,
			Infos: []models.LintMessageInfo{{
				ID:            uuid.New().String(),
				LintMessageID: "", // ingevuld bij save
				Message:       lintRunErr.Error(),
				Path:          oasURL,
			}},
		}
		res := &models.LintResult{
			ID:        uuid.New().String(),
			ApiID:     apiID,
			Successes: false,
			Failures:  1,
			Warnings:  0,
			Messages:  []models.LintMessage{msg},
			CreatedAt: now,
		}
		if err := s.repo.SaveLintResult(ctx, res); err != nil {
			log.Printf("[lint] save result failed: %v", err)
			return err
		}
		// Hash niet bijwerken, want lint is niet gelukt – zodat we later opnieuw proberen.
		return nil
	}

	msgs := openapi.ParseOutput(output, now)

	var errCount, warnCount int
	for _, m := range msgs {
		switch strings.ToLower(m.Severity) {
		case "error":
			errCount++
		case "warning":
			warnCount++
		}
	}
	score, _ := computeAdrScore(msgs)
	log.Printf("[lint] messages=%d errors=%d warnings=%d score=%d", len(msgs), errCount, warnCount, score)

	res := &models.LintResult{
		ID:        uuid.New().String(),
		ApiID:     apiID,
		Successes: score == 100,
		Failures:  errCount,
		Warnings:  warnCount,
		Messages:  msgs,
		CreatedAt: now,
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

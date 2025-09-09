package services

import (
    "context"
    "errors"
    "fmt"
    "math"
    "net/url"
    "log"
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

    for _, api := range apis {
        // Use the provided ctx so cron timeout/cancellation is respected.
        tools.Dispatch(ctx, "lint", func(ctx context.Context) error {
            return s.lintAndPersist(ctx, api.Id, api.OasUri, api.OasHash)
        })
    }
    return nil
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
    // Re-read uit DB om races met andere updates te voorkomen
    current, err := s.repo.GetApiByID(ctx, apiID)
    if err != nil || current == nil {
        return err
    }
    // Hash veranderd sinds we deze lint planden? Skip.
    log.Printf("[lint] api=%s expectedHash=%s currentHash=%s", apiID, expectedHash, current.OasHash)
    if current.OasHash == expectedHash {
        log.Printf("[lint] skip lint: hash unchanged")
        return nil
    }

    // Run linter
    log.Printf("[lint] running spectral for url=%s", oasURL)
    output, _ := linter.LintURL(ctx, oasURL)
    now := time.Now()

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
        Failures:  errCount, // occurrences
        Warnings:  warnCount,
        Messages:  msgs,
        CreatedAt: now,
    }
    if saveErr := s.repo.SaveLintResult(ctx, res); saveErr != nil {
        log.Printf("[lint] save result failed: %v", saveErr)
        return saveErr
    }
    log.Printf("[lint] saved lint result id=%s", res.ID)
    current.AdrScore = &score
    if err := s.repo.UpdateApi(ctx, *current); err != nil {
        log.Printf("[lint] update AdrScore failed: %v", err)
        return err
    }
    log.Printf("[lint] updated AdrScore=%d api=%s", score, apiID)
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

package handler

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	problem "github.com/developer-overheid-nl/don-api-register/pkg/api_client/helpers/problem"
	"github.com/developer-overheid-nl/don-api-register/pkg/api_client/helpers/util"
	"github.com/developer-overheid-nl/don-api-register/pkg/api_client/models"
	"github.com/developer-overheid-nl/don-api-register/pkg/api_client/services"
	"github.com/gin-gonic/gin"
)

// APIsAPIController binds HTTP requests to the APIsAPIService
type APIsAPIController struct {
	Service *services.APIsAPIService
}

// NewAPIsAPIController creates a new controller
func NewAPIsAPIController(s *services.APIsAPIService) *APIsAPIController {
	return &APIsAPIController{Service: s}
}

// ListApis handles GET /apis
func (c *APIsAPIController) ListApis(ctx *gin.Context, p *models.ListApisParams) ([]models.ApiSummary, error) {
	if p.Page < 1 {
		p.Page = 1
	}
	if p.PerPage < 1 {
		p.PerPage = 10
	}
	p.BaseURL = ctx.FullPath()
	apis, pagination, err := c.Service.ListApis(ctx.Request.Context(), p)
	if err != nil {
		return nil, err
	}
	util.SetPaginationHeaders(ctx.Request, ctx.Header, pagination)

	return apis, nil
}

// SearchApis handles GET /apis/search
func (c *APIsAPIController) SearchApis(ctx *gin.Context, p *models.ListApisSearchParams) ([]models.ApiSummary, error) {
	if p.Page < 1 {
		p.Page = 1
	}
	if p.PerPage < 1 {
		p.PerPage = 10
	}
	p.BaseURL = ctx.FullPath()
	results, pagination, err := c.Service.SearchApis(ctx.Request.Context(), p)
	if err != nil {
		return nil, err
	}
	util.SetPaginationHeaders(ctx.Request, ctx.Header, pagination)
	return results, nil
}

// RetrieveApi handles GET /apis/:id
func (c *APIsAPIController) RetrieveApi(ctx *gin.Context, params *models.ApiParams) (*models.ApiDetail, error) {
	api, err := c.Service.RetrieveApi(ctx.Request.Context(), params.Id)
	if err != nil {
		return nil, err
	}
	if api == nil {
		return nil, problem.NewNotFound(params.Id, "Api not found")
	}
	return api, nil
}

// ListLintResults handles GET /lint-results
func (c *APIsAPIController) ListLintResults(ctx *gin.Context) ([]models.LintResult, error) {
	return c.Service.ListLintResults(ctx.Request.Context())
}

// CreateApiFromOas handles POST /apis
func (c *APIsAPIController) CreateApiFromOas(ctx *gin.Context, body *models.ApiPost) (*models.ApiSummary, error) {
	created, err := c.Service.CreateApiFromOas(*body)
	if err != nil {
		return nil, err
	}
	return created, nil
}

// UpdateApi handles PUT /apis/:id
func (c *APIsAPIController) UpdateApi(ctx *gin.Context, body *models.UpdateApiInput) (*models.ApiSummary, error) {
	updated, err := c.Service.UpdateOasUri(ctx.Request.Context(), body)
	if errors.Is(err, services.ErrNeedsPost) {
		return nil, problem.NewNotFound(body.OasUrl, fmt.Sprintf("'%s' moet als nieuwe API geregistreerd worden via POST en de oude API als deprecated worden gemarkeerd", body.OasUrl),
			problem.InvalidParam{Name: "oasUrl", Reason: "Deze URI is nieuw of significant gewijzigd"},
		)
	}
	if err != nil {
		return nil, err
	}
	return updated, nil
}

// ListOrganisations handles GET /organisations
func (c *APIsAPIController) ListOrganisations(ctx *gin.Context) ([]models.OrganisationSummary, error) {
	orgs, total, err := c.Service.ListOrganisations(ctx.Request.Context())
	if err != nil {
		return nil, err
	}
	ctx.Header("Total-Count", fmt.Sprintf("%d", total))
	// Convert []models.Organisation to []models.OrganisationSummary
	orgSummaries := make([]models.OrganisationSummary, len(orgs))
	for i, org := range orgs {
		orgSummaries[i] = models.OrganisationSummary{
			Uri:   org.Uri,
			Label: org.Label,
			Links: &models.Links{
				Apis: &models.Link{
					Href: fmt.Sprintf("/v1/apis?organisation=%s", org.Uri),
				},
			},
		}
	}
	return orgSummaries, nil
}

// CreateOrganisation handles POST /organisations
func (c *APIsAPIController) CreateOrganisation(ctx *gin.Context, body *models.Organisation) (*models.Organisation, error) {
	created, err := c.Service.CreateOrganisation(ctx.Request.Context(), body)
	if err != nil {
		return nil, err
	}
	return created, nil
}

// GetPostman handles GET /apis/:id/postman
func (c *APIsAPIController) GetPostman(ctx *gin.Context, params *models.ApiParams) error {
	art, err := c.Service.GetArtifact(ctx.Request.Context(), params.Id, "postman")
	if err != nil {
		return err
	}
	if art == nil {
		return problem.NewNotFound(params.Id, "Postman artifact not found")
	}
	if art.ContentType != "" {
		ctx.Header("Content-Type", art.ContentType)
	}
	if art.Filename != "" {
		ctx.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", art.Filename))
	}
	ctx.Data(200, art.ContentType, art.Data)
	return nil
}

func (c *APIsAPIController) GetOas(ctx *gin.Context, params *models.ApiOasParams) error {
	version, format, err := parseOASVersionAndFormat(strings.TrimSpace(params.Version))
	if err != nil {
		return problem.NewBadRequest(params.Version, err.Error())
	}
	art, err := c.Service.GetOasDocument(ctx.Request.Context(), params.Id, version, format)
	if err != nil {
		return err
	}
	if art == nil {
		return problem.NewNotFound(params.Id, "OAS artifact not found")
	}
	if art.ContentType != "" {
		ctx.Header("Content-Type", art.ContentType)
	}
	if art.Filename != "" {
		ctx.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", art.Filename))
	}
	if art.Version != "" {
		ctx.Header("OAS-Version", art.Version)
	}
	if art.Source != "" {
		ctx.Header("OAS-Source", art.Source)
	}
	ctx.Data(200, art.ContentType, art.Data)
	return nil
}

func parseOASVersionAndFormat(raw string) (string, string, error) {
	const (
		jsonSuffix = ".json"
		yamlSuffix = ".yaml"
		ymlSuffix  = ".yml"
	)
	if raw == "" {
		return "", "", fmt.Errorf("verwacht pad-formaat {version}.{ext} met json of yaml")
	}
	lower := strings.ToLower(raw)

	switch {
	case strings.HasSuffix(lower, jsonSuffix):
		version := strings.TrimSpace(raw[:len(raw)-len(jsonSuffix)])
		if version == "" {
			return "", "", fmt.Errorf("versie mag niet leeg zijn")
		}
		norm, err := normalizeOASVersion(version)
		if err != nil {
			return "", "", err
		}
		return norm, "json", nil
	case strings.HasSuffix(lower, yamlSuffix):
		version := strings.TrimSpace(raw[:len(raw)-len(yamlSuffix)])
		if version == "" {
			return "", "", fmt.Errorf("versie mag niet leeg zijn")
		}
		norm, err := normalizeOASVersion(version)
		if err != nil {
			return "", "", err
		}
		return norm, "yaml", nil
	case strings.HasSuffix(lower, ymlSuffix):
		version := strings.TrimSpace(raw[:len(raw)-len(ymlSuffix)])
		if version == "" {
			return "", "", fmt.Errorf("versie mag niet leeg zijn")
		}
		norm, err := normalizeOASVersion(version)
		if err != nil {
			return "", "", err
		}
		return norm, "yaml", nil
	default:
		return "", "", fmt.Errorf("verwacht pad-formaat {version}.{ext} met json of yaml")
	}
}

func normalizeOASVersion(version string) (string, error) {
	segments := strings.Split(version, ".")
	if len(segments) < 2 {
		return "", fmt.Errorf("ondersteunde versies zijn 3.0 en 3.1")
	}
	major := strings.TrimSpace(segments[0])
	minor := strings.TrimSpace(segments[1])

	majorInt, err := strconv.Atoi(major)
	if err != nil || majorInt != 3 {
		return "", fmt.Errorf("ondersteunde versies zijn 3.0 en 3.1")
	}
	minorInt, err := strconv.Atoi(minor)
	if err != nil {
		return "", fmt.Errorf("ondersteunde versies zijn 3.0 en 3.1")
	}
	if minorInt != 0 && minorInt != 1 {
		return "", fmt.Errorf("ondersteunde versies zijn 3.0 en 3.1")
	}
	return fmt.Sprintf("3.%d", minorInt), nil
}

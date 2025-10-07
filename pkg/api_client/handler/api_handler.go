package handler

import (
	"errors"
	"fmt"

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
func (c *APIsAPIController) SearchApis(ctx *gin.Context, params *models.SearchApisParams) ([]models.ApiSummary, error) {
	limit := params.EffectiveLimit()
	query := params.NormalizedQuery()
	results, err := c.Service.SearchApis(ctx.Request.Context(), query, limit)
	if err != nil {
		return nil, err
	}
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
	ctx.Header("X-Total-Count", fmt.Sprintf("%d", total))
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

// GetBruno handles GET /apis/:id/bruno
func (c *APIsAPIController) GetBruno(ctx *gin.Context, params *models.ApiParams) error {
	art, err := c.Service.GetArtifact(ctx.Request.Context(), params.Id, "bruno")
	if err != nil {
		return err
	}
	if art == nil {
		return problem.NewNotFound(params.Id, "Bruno artifact not found")
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

func (c *APIsAPIController) GetOas31(ctx *gin.Context, params *models.ApiParams) error {
	art, err := c.Service.GetArtifact(ctx.Request.Context(), params.Id, "oas-converted")
	if err != nil {
		return err
	}
	if art == nil {
		return problem.NewNotFound(params.Id, "OAS 3.1 artifact not found")
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

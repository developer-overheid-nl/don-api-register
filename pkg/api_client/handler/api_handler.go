package handler

import (
	"errors"
	"fmt"
	"strings"

	problem "github.com/developer-overheid-nl/don-api-register/pkg/api_client/helpers/problem"
	"github.com/developer-overheid-nl/don-api-register/pkg/api_client/models"
	"github.com/developer-overheid-nl/don-api-register/pkg/api_client/params"
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
func (c *APIsAPIController) ListApis(ctx *gin.Context, p *params.ListApisParams) (*models.ApiListResponse, error) {
	if p.Page < 1 {
		p.Page = 1
	}
	if p.PerPage < 1 {
		p.PerPage = 10
	}
	p.BaseURL = ctx.FullPath()
	response, pagination, err := c.Service.ListApis(ctx.Request.Context(), p)
	if err != nil {
		return nil, err
	}
	ctx.Header("X-Total-Count", fmt.Sprintf("%d", pagination.TotalRecords))
	ctx.Header("X-Total-Pages", fmt.Sprintf("%d", pagination.TotalPages))
	ctx.Header("X-Per-Page", fmt.Sprintf("%d", pagination.RecordsPerPage))
	ctx.Header("X-Current-Page", fmt.Sprintf("%d", pagination.CurrentPage))
	// Add RFC 5988 Link header
	if response != nil {
		var links []string
		if response.Links.Self != nil {
			links = append(links, fmt.Sprintf("<%s>; rel=\"self\"", response.Links.Self.Href))
		}
		if response.Links.Next != nil {
			links = append(links, fmt.Sprintf("<%s>; rel=\"next\"", response.Links.Next.Href))
		}
		if response.Links.Prev != nil {
			links = append(links, fmt.Sprintf("<%s>; rel=\"prev\"", response.Links.Prev.Href))
		}
		if response.Links.First != nil {
			links = append(links, fmt.Sprintf("<%s>; rel=\"first\"", response.Links.First.Href))
		}
		if response.Links.Last != nil {
			links = append(links, fmt.Sprintf("<%s>; rel=\"last\"", response.Links.Last.Href))
		}
		if len(links) > 0 {
			ctx.Header("Link", strings.Join(links, ", "))
		}
	}
	return response, nil
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
func (c *APIsAPIController) ListOrganisations(ctx *gin.Context) (*models.OrganisationListResponse, error) {
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
	return &models.OrganisationListResponse{
		Organisations: orgSummaries,
	}, nil
}

// CreateOrganisation handles POST /organisations
func (c *APIsAPIController) CreateOrganisation(ctx *gin.Context, body *models.Organisation) (*models.Organisation, error) {
	created, err := c.Service.CreateOrganisation(ctx.Request.Context(), body)
	if err != nil {
		return nil, err
	}
	return created, nil
}

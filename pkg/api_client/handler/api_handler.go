package handler

import (
	"errors"
	"fmt"
	"github.com/developer-overheid-nl/don-api-register/pkg/api_client/helpers"
	"github.com/developer-overheid-nl/don-api-register/pkg/api_client/models"
	"github.com/developer-overheid-nl/don-api-register/pkg/api_client/services"

	"github.com/gin-gonic/gin"
)

// APIsAPIController binds HTTP requests to the APIsAPIService
type APIsAPIController struct {
	service      *services.APIsAPIService
	errorHandler helpers.ErrorHandler
}

// NewAPIsAPIController creates a new controller
func NewAPIsAPIController(s *services.APIsAPIService) *APIsAPIController {
	return &APIsAPIController{service: s, errorHandler: helpers.DefaultErrorHandler}
}

// listApisParams defines query parameters for ListApis
type listApisParams struct {
	Page    int `query:"page"`
	PerPage int `query:"perPage"`
}

// ListApis handles GET /apis
func (c *APIsAPIController) ListApis(ctx *gin.Context, params *listApisParams) (interface{}, error) {
	if params.Page < 1 {
		params.Page = 1
	}
	if params.PerPage < 1 {
		params.PerPage = 10
	}
	baseURL := fmt.Sprintf("https://%s%s", ctx.Request.Host, ctx.FullPath())

	response, err := c.service.ListApis(ctx.Request.Context(), params.Page, params.PerPage, baseURL)
	if err != nil {
		return nil, err
	}
	return response, nil
}

// RetrieveApi handles GET /api/:id
func (c *APIsAPIController) RetrieveApi(ctx *gin.Context, params *models.OasParams) (*models.Api, error) {
	api, err := c.service.RetrieveApi(ctx.Request.Context(), params.OasUrl)
	if err != nil {
		return nil, err
	}
	if api == nil {
		return nil, fmt.Errorf("API not found")
	}
	return api, nil
}

// CreateApiFromOas handles POST /apis
func (c *APIsAPIController) CreateApiFromOas(ctx *gin.Context, body *models.Api) (*models.ApiResponse, error) {
	created, err := c.service.CreateApiFromOas(*body)
	if err != nil {
		return nil, err
	}
	return created, nil
}

// UpdateApi handles PUT /api
func (c *APIsAPIController) UpdateApi(ctx *gin.Context, params *models.OasParams) (interface{}, error) {
	if err := c.service.UpdateOasUri(ctx.Request.Context(), params.OasUrl); err != nil {
		if errors.Is(err, services.ErrNeedsPost) {
			return nil, helpers.NewNotFound(fmt.Sprintf("'%s' moet als nieuwe API geregistreerd worden via POST en de oude API als deprecated worden gemarkeerd", params.OasUrl),
				helpers.InvalidParam{Name: "oasUri", Reason: "Deze URI is nieuw of significant gewijzigd"},
			)
		}
		return nil, err
	}
	return nil, nil
}

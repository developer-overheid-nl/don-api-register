package handler

import (
	"fmt"
	"strings"

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
func (c *APIsAPIController) ListApis(ctx *gin.Context, params *listApisParams) (*models.PaginatedResponse, error) {
	// standaardwaarden
	if params.Page < 1 {
		params.Page = 1
	}
	if params.PerPage < 1 {
		params.PerPage = 10
	}

	paginated, err := c.service.ListApis(ctx.Request.Context(), params.Page, params.PerPage)
	if err != nil {
		return nil, err
	}
	return &paginated, nil
}

// retrieveApiParams defines path parameter for RetrieveApi
type retrieveApiParams struct {
	ID string `path:"id"`
}

// RetrieveApi handles GET /api/:id
func (c *APIsAPIController) RetrieveApi(ctx *gin.Context, params *retrieveApiParams) (*models.Api, error) {
	if params.ID == "" {
		return nil, fmt.Errorf("field 'id' is required")
	}

	api, err := c.service.RetrieveApi(ctx.Request.Context(), params.ID)
	if err != nil {
		return nil, err
	}
	if api == nil {
		return nil, fmt.Errorf("API not found")
	}
	return api, nil
}

// CreateApiFromOas handles POST /apis
func (c *APIsAPIController) CreateApiFromOas(body *models.Api) (*models.Api, error) {
	created, missing, err := c.service.CreateApiFromOas(*body)
	if err != nil {
		if len(missing) > 0 {
			return nil, fmt.Errorf("missing properties: %s", strings.Join(missing, ", "))
		}
		return nil, err
	}
	return created, nil
}

// UpdateApi handles PUT /api/:id
// updateApiParams defines the path parameter and request body for UpdateApi
type updateApiParams struct {
	ID         string `path:"id" json:"-"`
	models.Api        // embedded: body fields are all JSON properties
}

// UpdateApi handles PUT /api/:id
func (c *APIsAPIController) UpdateApi(ctx *gin.Context, params *updateApiParams) (*models.Api, error) {
	// ensure that the API object's ID matches the path parameter
	params.Api.Id = params.ID

	if err := c.service.UpdateApi(ctx.Request.Context(), params.Api); err != nil {
		return nil, err
	}
	return &params.Api, nil
}

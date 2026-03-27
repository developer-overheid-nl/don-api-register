package handler

import (
	"github.com/developer-overheid-nl/don-api-register/pkg/api_client/helpers/util"
	"github.com/developer-overheid-nl/don-api-register/pkg/api_client/models"
	"github.com/developer-overheid-nl/don-api-register/pkg/api_client/services"
	"github.com/gin-gonic/gin"
)

type StatisticsController struct {
	Service *services.AdoptionService
}

func NewStatisticsController(s *services.AdoptionService) *StatisticsController {
	return &StatisticsController{Service: s}
}

func (c *StatisticsController) GetSummary(ctx *gin.Context, p *models.AdoptionBaseParams) (*models.AdoptionSummary, error) {
	return c.Service.GetSummary(ctx.Request.Context(), p)
}

func (c *StatisticsController) GetRules(ctx *gin.Context, p *models.AdoptionRulesParams) (*models.AdoptionRules, error) {
	return c.Service.GetRules(ctx.Request.Context(), p)
}

func (c *StatisticsController) GetTimeline(ctx *gin.Context, p *models.AdoptionTimelineParams) (*models.AdoptionTimeline, error) {
	return c.Service.GetTimeline(ctx.Request.Context(), p)
}

func (c *StatisticsController) GetApis(ctx *gin.Context, p *models.AdoptionApisParams) (*models.AdoptionApis, error) {
	result, pagination, err := c.Service.GetApis(ctx.Request.Context(), p)
	if err != nil {
		return nil, err
	}
	util.SetPaginationHeaders(ctx.Request, ctx.Header, *pagination)
	return result, nil
}

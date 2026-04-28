package handler

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/developer-overheid-nl/don-api-register/pkg/api_client/helpers/util"
	"github.com/developer-overheid-nl/don-api-register/pkg/api_client/models"
	"github.com/gin-gonic/gin"
)

const apisJsonLdMediaType = "application/ld+json"

var apisJsonLdContext = json.RawMessage(`{"dcat":"http://www.w3.org/ns/dcat#","dct":"http://purl.org/dc/terms/","vcard":"http://www.w3.org/2006/vcard/ns#","id":{"@id":"dct:identifier"},"title":{"@id":"dct:title"},"description":{"@id":"dct:description"},"contact":{"@id":"dcat:contactPoint"},"name":{"@id":"vcard:fn"},"email":{"@id":"vcard:hasEmail","@type":"@id"},"url":{"@id":"vcard:hasURL","@type":"@id"},"uri":{"@id":"dct:publisher"},"oasUrl":{"@id":"dcat:endpointDescription","@type":"@id"}}`)

// AcceptsJsonLd reports whether the Accept header explicitly requests application/ld+json.
func AcceptsJsonLd(accept string) bool {
	for _, part := range strings.Split(accept, ",") {
		media := strings.TrimSpace(strings.SplitN(part, ";", 2)[0])
		if strings.EqualFold(media, apisJsonLdMediaType) {
			return true
		}
	}
	return false
}

// ListApisJsonLd handles GET /apis with Accept: application/ld+json.
func (c *APIsAPIController) ListApisJsonLd(ctx *gin.Context) error {
	p := &models.ListApisParams{}
	if v := ctx.Query("page"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			p.Page = n
		}
	}
	if v := ctx.Query("perPage"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			p.PerPage = n
		}
	}
	if v := ctx.Query("organisation"); v != "" {
		val := v
		p.Organisation = &val
	}
	if v := ctx.Query("ids"); v != "" {
		val := v
		p.Ids = &val
	}
	if p.Page < 1 {
		p.Page = 1
	}
	if p.PerPage < 1 {
		p.PerPage = 10
	}
	p.BaseURL = ctx.FullPath()

	apis, pagination, err := c.Service.ListApis(ctx.Request.Context(), p)
	if err != nil {
		return err
	}
	util.SetPaginationHeaders(ctx.Request, ctx.Header, pagination)

	items := make([]models.ApiSummaryJsonLd, len(apis))
	for i, a := range apis {
		items[i] = models.ApiSummaryJsonLd{
			Type:       "dcat:DataService",
			ConformsTo: []string{"https://spec.openapis.org/oas"},
			ApiSummary: a,
		}
	}
	body := models.ApisCollectionJsonLd{
		Context: apisJsonLdContext,
		Graph:   items,
	}
	data, err := json.Marshal(body)
	if err != nil {
		return err
	}
	ctx.Data(http.StatusOK, apisJsonLdMediaType, data)
	return nil
}

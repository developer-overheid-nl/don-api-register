package handler

import (
	"encoding/json"
	"net/http"
	"strings"

	problem "github.com/developer-overheid-nl/don-api-register/pkg/api_client/helpers/problem"
	"github.com/developer-overheid-nl/don-api-register/pkg/api_client/models"
	"github.com/gin-gonic/gin"
)

const ApisJsonLdMediaType = "application/ld+json"

var apiDetailJsonLdContext = json.RawMessage(`{"dcat":"http://www.w3.org/ns/dcat#","dct":"http://purl.org/dc/terms/","vcard":"http://www.w3.org/2006/vcard/ns#","dcat:endpointDescription":{"@type":"@id"},"vcard:hasEmail":{"@type":"@id"},"vcard:hasURL":{"@type":"@id"},"dct:publisher":{"@type":"@id"}}`)

func oasConformsToURL(version string) string {
	v := strings.TrimSpace(version)
	if v == "" {
		return "https://spec.openapis.org/oas"
	}
	return "https://spec.openapis.org/oas/v" + v + ".html"
}

// AcceptsJsonLd reports whether the Accept header explicitly requests application/ld+json.
func AcceptsJsonLd(accept string) bool {
	for _, part := range strings.Split(accept, ",") {
		media := strings.TrimSpace(strings.SplitN(part, ";", 2)[0])
		if strings.EqualFold(media, ApisJsonLdMediaType) {
			return true
		}
	}
	return false
}

// RetrieveApiJsonLd handles GET /apis/:id with Accept: application/ld+json.
func (c *APIsAPIController) RetrieveApiJsonLd(ctx *gin.Context, params *models.ApiParams) error {
	api, err := c.Service.RetrieveApi(ctx.Request.Context(), params.Id)
	if err != nil {
		return err
	}
	if api == nil {
		return problem.NewNotFound(params.Id, "Api not found")
	}

	contact := models.ContactJsonLd{FN: api.Contact.Name}
	if api.Contact.Email != "" {
		contact.HasEmail = "mailto:" + api.Contact.Email
	}
	if api.Contact.URL != "" {
		contact.HasURL = api.Contact.URL
	}

	body := models.ApiDetailJsonLd{
		Context:             apiDetailJsonLdContext,
		Type:                "dcat:DataService",
		ConformsTo:          []string{oasConformsToURL(api.OasVersion)},
		Identifier:          api.Id,
		Title:               api.Title,
		Description:         api.Description,
		EndpointDescription: api.OasUrl,
		ContactPoint:        contact,
		Publisher:           api.Organisation.Uri,
	}
	data, err := json.Marshal(body)
	if err != nil {
		return err
	}
	ctx.Data(http.StatusOK, ApisJsonLdMediaType, data)
	return nil
}

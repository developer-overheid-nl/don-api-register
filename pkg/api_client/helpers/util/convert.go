package util

import (
	"fmt"

	"github.com/developer-overheid-nl/don-api-register/pkg/api_client/models"
)

func ToApiSummary(api *models.Api) models.ApiSummary {
	var org models.Organisation
	if api.Organisation != nil {
		org = *api.Organisation
	}

	return models.ApiSummary{
		Id:          api.Id,
		OasUrl:      api.OasUri,
		Title:       api.Title,
		Description: api.Description,
		Contact: models.Contact{
			Name:  api.ContactName,
			URL:   api.ContactUrl,
			Email: api.ContactEmail,
		},
		Organisation: org,
		AdrScore:     api.AdrScore,
		Links: &models.Links{
			Self: &models.Link{Href: fmt.Sprintf("/apis/%s", api.Id)},
		},
	}
}

func ToApiDetail(api *models.Api) *models.ApiDetail {
	return &models.ApiDetail{
		ApiSummary: ToApiSummary(api),
		DocsUri:    api.DocsUri,
		Servers:    api.Servers,
	}
}

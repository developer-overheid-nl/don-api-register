package util

import (
	"fmt"
	"time"

	"github.com/developer-overheid-nl/don-api-register/pkg/api_client/models"
)

func ToApiSummary(api *models.Api) models.ApiSummary {
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
		Lifecycle: models.Lifecycle{
			Version:    api.Version,
			Sunset:     api.Sunset,
			Deprecated: api.Deprecated,
			Status:     api.LifecycleStatus(time.Now()),
		},
		AdrScore: api.AdrScore,
		Organisation: models.OrganisationSummary{
			Uri:   api.Organisation.Uri,
			Label: api.Organisation.Label,
			Links: &models.Links{
				Apis: &models.Link{Href: fmt.Sprintf("/v1/apis?organisation=%s", api.Organisation.Uri)},
			},
		},

		Links: &models.Links{
			Self: &models.Link{Href: fmt.Sprintf("/v1/apis/%s", api.Id)},
		},
	}
}

func ToApiDetail(api *models.Api) *models.ApiDetail {
	// Map servers to only url and description
	servers := make([]models.ServerInfo, 0, len(api.Servers))
	for _, srv := range api.Servers {
		servers = append(servers, models.ServerInfo{
			Url:         srv.Uri,
			Description: srv.Description,
		})
	}

	detail := &models.ApiDetail{
		ApiSummary: ToApiSummary(api),
		DocsUrl:    api.DocsUrl,
		Servers:    servers,
	}
	// Remove Links from detail
	detail.Links = nil
	return detail
}

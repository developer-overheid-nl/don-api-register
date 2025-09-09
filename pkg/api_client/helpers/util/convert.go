package util

import (
	"fmt"
	"time"

	"github.com/developer-overheid-nl/don-api-register/pkg/api_client/models"
)

func parseTime(value string) time.Time {
	t, err := time.Parse(time.DateOnly, value)
	if err != nil {
		return time.Time{}
	}
	return t
}

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
			Status: func() string {
				switch {
				case api.Sunset != "" && parseTime(api.Sunset).After(time.Now()):
					return "sunset"
				case api.Sunset != "" && parseTime(api.Sunset).Before(time.Now()):
					return "retired"
				case api.Deprecated != "" && parseTime(api.Deprecated).Before(time.Now()):
					return "deprecated"
				default:
					return "active"
				}
			}(),
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

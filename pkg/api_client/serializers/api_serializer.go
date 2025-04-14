package serializers

import (
	"github.com/developer-overheid-nl/don-api-register/pkg/api_client/models"
)

func SerializeApi(data models.ApiRawData) models.Api {
	return models.Api{
		Id:            data.Id,
		Type:          data.Type,
		Title:         data.Title,
		Description:   data.Description,
		Auth:          data.Auth,
		OasUri:        deref(data.OasUri),
		DocsUri:       deref(data.DocsUri),
		AdrScore:      data.AdrScore,
		RepositoryUri: data.RepositoryUri,
		Organisation:  serializeOrganisation(data.Organisation),
	}
}

func serializeOrganisation(org *models.OrganisationInfo) *models.ApiOrganisation {
	if org == nil {
		return nil
	}
	return &models.ApiOrganisation{
		Label: org.Name,
		Uri:   deref(org.Uri),
	}
}
func deref(ptr *string) string {
	if ptr != nil {
		return *ptr
	}
	return ""
}

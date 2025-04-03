// Code generated by OpenAPI Generator (https://openapi-generator.tech); DO NOT EDIT.

/*
 * API register API v1
 *
 * API van het API register (apis.developer.overheid.nl)
 *
 * API version: 1.0.0
 * Contact: developer.overheid@geonovum.nl
 */

package api_client




type Api struct {

	Id string `json:"id,omitempty"`

	Type string `json:"type,omitempty"`

	OasUri string `json:"oasUri,omitempty"`

	DocsUri string `json:"docsUri,omitempty"`

	Title string `json:"title,omitempty"`

	Description string `json:"description,omitempty"`

	Auth string `json:"auth,omitempty"`

	AdrScore string `json:"adrScore,omitempty"`

	RepositoryUri string `json:"repositoryUri,omitempty"`

	Organisation ApiOrganisation `json:"organisation,omitempty"`
}

// AssertApiRequired checks if the required fields are not zero-ed
func AssertApiRequired(obj Api) error {
	if err := AssertApiOrganisationRequired(obj.Organisation); err != nil {
		return err
	}
	return nil
}

// AssertApiConstraints checks if the values respects the defined constraints
func AssertApiConstraints(obj Api) error {
	if err := AssertApiOrganisationConstraints(obj.Organisation); err != nil {
		return err
	}
	return nil
}

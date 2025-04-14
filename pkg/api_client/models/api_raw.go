package models

type ApiRawData struct {
	Id            string
	Type          string
	Title         string
	Description   string
	Auth          string
	OasUri        *string
	DocsUri       *string
	AdrScore      *string
	RepositoryUri *string
	Organisation  *OrganisationInfo
}

type OrganisationInfo struct {
	Name string
	Uri  *string
}

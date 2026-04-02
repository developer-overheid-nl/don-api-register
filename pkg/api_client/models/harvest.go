package models

type HarvestSource struct {
	Name            string  `json:"name,omitempty"`
	IndexURL        string  `json:"indexUrl"`
	OrganisationUri string  `json:"organisationUri"`
	Contact         Contact `json:"contact"`
	UISuffix        string  `json:"uiSuffix,omitempty"`
	OASPath         string  `json:"oasPath,omitempty"`
}

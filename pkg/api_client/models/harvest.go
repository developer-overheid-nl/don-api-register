package models

import "encoding/json"

type HarvestSource struct {
	Name            string  `json:"name,omitempty"`
	IndexURL        string  `json:"indexUrl"`
	OrganisationUri string  `json:"organisationUri"`
	Contact         Contact `json:"contact"`
	UISuffix        string  `json:"uiSuffix,omitempty"`
	OASPath         string  `json:"oasPath,omitempty"`
}

type HarvestResult struct {
	CandidateCount int
	CreatedCount   int
	SkippedCount   int
	FailedCount    int
}

type HarvestIndexLink struct {
	Href string `json:"href"`
}

type HarvestIndexAPIEntry struct {
	Links json.RawMessage `json:"links"`
}

type HarvestIndexRoot struct {
	Apis []HarvestIndexAPIEntry `json:"apis"`
}

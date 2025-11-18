package tools

import "strings"

// OASInput represents an OpenAPI source provided either as URL or raw body.
type OASInput struct {
	OasUrl  string `json:"oasUrl,omitempty"`
	OasBody string `json:"oasBody,omitempty"`
}

// Normalize trims whitespace in-place for easier comparisons.
func (i *OASInput) Normalize() {
	i.OasUrl = strings.TrimSpace(i.OasUrl)
	i.OasBody = strings.TrimSpace(i.OasBody)
}

// IsEmpty returns true when neither URL nor body is provided.
func (i OASInput) IsEmpty() bool {
	return strings.TrimSpace(i.OasUrl) == "" && strings.TrimSpace(i.OasBody) == ""
}

// ArazzoInput mirrors the tools API input contract for arazzo endpoints.
type ArazzoInput struct {
	ArazzoUrl  string `json:"arazzoUrl,omitempty"`
	ArazzoBody string `json:"arazzoBody,omitempty"`
}

// Normalize trims whitespace in-place for easier comparisons.
func (i *ArazzoInput) Normalize() {
	i.ArazzoUrl = strings.TrimSpace(i.ArazzoUrl)
	i.ArazzoBody = strings.TrimSpace(i.ArazzoBody)
}

// IsEmpty returns true when neither URL nor body is provided.
func (i ArazzoInput) IsEmpty() bool {
	return strings.TrimSpace(i.ArazzoUrl) == "" && strings.TrimSpace(i.ArazzoBody) == ""
}

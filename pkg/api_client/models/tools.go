package models

import (
	"strings"
	"time"
)

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

// LintMessageInfoDTO matches a tools lint message info response.
type LintMessageInfoDTO struct {
	ID            string `json:"id"`
	LintMessageID string `json:"lintMessageId,omitempty"`
	Message       string `json:"message"`
	Path          string `json:"path,omitempty"`
}

// LintMessageDTO matches a tools lint message response.
type LintMessageDTO struct {
	ID        string               `json:"id"`
	Code      string               `json:"code"`
	Severity  string               `json:"severity"`
	CreatedAt time.Time            `json:"createdAt"`
	Infos     []LintMessageInfoDTO `json:"infos,omitempty"`
}

// LintResultDTO matches the tools lint response.
type LintResultDTO struct {
	ID             string           `json:"id"`
	ApiID          string           `json:"apiId,omitempty"`
	Successes      bool             `json:"successes"`
	Failures       int              `json:"failures"`
	Warnings       int              `json:"warnings"`
	Score          int              `json:"score"`
	Messages       []LintMessageDTO `json:"messages"`
	CreatedAt      time.Time        `json:"createdAt"`
	RulesetVersion string           `json:"rulesetVersion"`
}

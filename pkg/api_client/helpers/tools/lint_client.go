package tools

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"time"
)

// DTOs that match the tools lint response
type LintMessageInfoDTO struct {
	ID            string `json:"id"`
	LintMessageID string `json:"lintMessageId,omitempty"`
	Message       string `json:"message"`
	Path          string `json:"path,omitempty"`
}

type LintMessageDTO struct {
	ID        string               `json:"id"`
	Code      string               `json:"code"`
	Severity  string               `json:"severity"`
	CreatedAt time.Time            `json:"createdAt"`
	Infos     []LintMessageInfoDTO `json:"infos,omitempty"`
}

type LintResultDTO struct {
	ID        string           `json:"id"`
	ApiID     string           `json:"apiId,omitempty"`
	Successes bool             `json:"successes"`
	Failures  int              `json:"failures"`
	Warnings  int              `json:"warnings"`
	Score     int              `json:"score"`
	Messages  []LintMessageDTO `json:"messages"`
	CreatedAt time.Time        `json:"createdAt"`
	RulesetVersion string `json:"rulesetVersion"`
}

// LintGet calls the tools API to lint the given OAS input and returns the result DTO.
func LintGet(ctx context.Context, input OASInput) (*LintResultDTO, error) {
	input.Normalize()
	if input.IsEmpty() {
		return nil, errors.New("missing OAS input")
	}
	data, _, err := doToolsJSONRequest(ctx, "oas/validate", input, "application/json")
	if err != nil {
		log.Printf("[LintGet] request failed: %v", err)
		return nil, err
	}
	var out LintResultDTO
	if err := json.Unmarshal(data, &out); err != nil {
		log.Printf("[LintGet] decode response failed: %v", err)
		return nil, err
	}
	log.Printf("[LintGet] Lint-resultaat succesvol ontvangen: %+v", out)
	return &out, nil
}

package tools

import (
	"context"
	"encoding/json"
	"errors"
	"log"

	"github.com/developer-overheid-nl/don-api-register/pkg/api_client/models"
)

type LintMessageInfoDTO = models.LintMessageInfoDTO

type LintMessageDTO = models.LintMessageDTO

type LintResultDTO = models.LintResultDTO

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
	log.Printf("[LintGet] lint result id=%s messages=%d failures=%d warnings=%d score=%d", out.ID, len(out.Messages), out.Failures, out.Warnings, out.Score)
	return &out, nil
}

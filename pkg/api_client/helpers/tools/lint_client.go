package tools

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"

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
		return nil, err
	}
	var out LintResultDTO
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, err
	}
	slog.DebugContext(
		ctx,
		"OAS lint completed",
		"component", "tools",
		"operation", "lint",
		"message_count", len(out.Messages),
		"failure_count", out.Failures,
		"warning_count", out.Warnings,
		"score", out.Score,
	)
	return &out, nil
}

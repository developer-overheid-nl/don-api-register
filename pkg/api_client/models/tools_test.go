package models

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOASInputNormalizeAndIsEmpty(t *testing.T) {
	input := OASInput{
		OasUrl:  " https://example.org/openapi.json ",
		OasBody: "  ",
	}

	input.Normalize()

	assert.Equal(t, "https://example.org/openapi.json", input.OasUrl)
	assert.Empty(t, input.OasBody)
	assert.False(t, input.IsEmpty())

	input.OasUrl = " "
	assert.True(t, input.IsEmpty())
}

func TestArazzoInputNormalizeAndIsEmpty(t *testing.T) {
	input := ArazzoInput{
		ArazzoUrl:  " ",
		ArazzoBody: " workflows: [] ",
	}

	input.Normalize()

	assert.Empty(t, input.ArazzoUrl)
	assert.Equal(t, "workflows: []", input.ArazzoBody)
	assert.False(t, input.IsEmpty())

	input.ArazzoBody = "\t"
	assert.True(t, input.IsEmpty())
}

func TestLintResultDTOJSONRoundTrip(t *testing.T) {
	result := LintResultDTO{
		ID:             "lint-1",
		ApiID:          "api-1",
		Successes:      true,
		Score:          95,
		RulesetVersion: "2026.07",
		Messages: []LintMessageDTO{{
			ID:       "msg-1",
			Code:     "rule-1",
			Severity: "warning",
			Infos: []LintMessageInfoDTO{{
				ID:            "info-1",
				LintMessageID: "msg-1",
				Message:       "detail",
				Path:          "$.info",
			}},
		}},
	}

	data, err := json.Marshal(result)
	require.NoError(t, err)

	var decoded LintResultDTO
	require.NoError(t, json.Unmarshal(data, &decoded))

	assert.Equal(t, "lint-1", decoded.ID)
	assert.Equal(t, "api-1", decoded.ApiID)
	assert.Equal(t, "2026.07", decoded.RulesetVersion)
	require.Len(t, decoded.Messages, 1)
	require.Len(t, decoded.Messages[0].Infos, 1)
	assert.Equal(t, "$.info", decoded.Messages[0].Infos[0].Path)
}

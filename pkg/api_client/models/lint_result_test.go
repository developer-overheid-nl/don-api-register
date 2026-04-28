package models_test

import (
	"encoding/json"
	"testing"

	"github.com/developer-overheid-nl/don-api-register/pkg/api_client/models"
	"github.com/stretchr/testify/require"
)

func TestLintResultJSON_RulesetVersionLivesOnMessage(t *testing.T) {
	result := models.LintResult{
		ID:    "result-1",
		ApiID: "api-1",
		Messages: []models.LintMessage{
			{
				ID:             "message-1",
				LintResultID:   "result-1",
				RulesetVersion: "2026.04",
			},
		},
	}

	data, err := json.Marshal(result)
	require.NoError(t, err)

	var decoded map[string]any
	require.NoError(t, json.Unmarshal(data, &decoded))

	_, exists := decoded["rulesetVersion"]
	require.False(t, exists)

	messages, ok := decoded["messages"].([]any)
	require.True(t, ok)
	require.Len(t, messages, 1)

	message, ok := messages[0].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "2026.04", message["rulesetVersion"])
}

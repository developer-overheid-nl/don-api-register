package openapi

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseOutputGroupsMessagesByCode(t *testing.T) {
	now := time.Date(2026, 7, 8, 12, 0, 0, 0, time.UTC)
	out := `
1:2 warning adr-001 Eerste melding  $.info.title
3:4 error adr-001 Tweede melding  $.paths
not a spectral line
5:6 warning adr-002 Andere melding  $.info.description
`

	got := ParseOutput(out, now)

	require.Len(t, got, 2)
	byCode := map[string]int{}
	for _, message := range got {
		byCode[message.Code] = len(message.Infos)
		assert.NotEmpty(t, message.ID)
		assert.Equal(t, now, message.CreatedAt)
		assert.NotZero(t, message.Line)
		assert.NotZero(t, message.Column)
		for _, info := range message.Infos {
			assert.NotEmpty(t, info.ID)
			assert.Equal(t, message.ID, info.LintMessageID)
			assert.NotEmpty(t, info.Message)
			assert.NotEmpty(t, info.Path)
		}
	}
	assert.Equal(t, 2, byCode["adr-001"])
	assert.Equal(t, 1, byCode["adr-002"])
}

func TestParseOutputIgnoresEmptyOrMalformedLines(t *testing.T) {
	got := ParseOutput("\ninvalid\n1:2 warning missing-columns\n", time.Now())

	assert.Empty(t, got)
}

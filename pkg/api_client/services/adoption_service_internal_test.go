package services

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseDateRange_RejectsStartAfterEnd(t *testing.T) {
	start, end, err := parseDateRange("2026-02-01", "2026-01-31")

	require.Error(t, err)
	assert.True(t, start.IsZero())
	assert.True(t, end.IsZero())
	assert.Contains(t, err.Error(), "startDate must be on or before endDate")
}

func TestParseDateRange_AllowsSameDay(t *testing.T) {
	start, end, err := parseDateRange("2026-01-31", "2026-01-31")

	require.NoError(t, err)
	assert.Equal(t, time.Date(2026, 1, 31, 0, 0, 0, 0, time.UTC), start)
	assert.Equal(t, time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC), end)
}

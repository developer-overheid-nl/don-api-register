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

func TestParseDateRange_AllowsDutchDateFormat(t *testing.T) {
	start, end, err := parseDateRange("01-01-2024", "31-01-2024")

	require.NoError(t, err)
	assert.Equal(t, time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC), start)
	assert.Equal(t, time.Date(2024, 2, 1, 0, 0, 0, 0, time.UTC), end)
}

func TestParseDateRange_InvalidFormat_ShowsSupportedFormats(t *testing.T) {
	_, _, err := parseDateRange("2024/01/01", "2024/01/31")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid startDate")
	assert.Contains(t, err.Error(), "YYYY-MM-DD or DD-MM-YYYY")
}

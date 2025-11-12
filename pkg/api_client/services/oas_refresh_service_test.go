package services

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNextRunAt_BeforeTarget(t *testing.T) {
	loc := time.FixedZone("CET", 3600)
	now := time.Date(2025, 4, 10, 6, 15, 0, 0, loc)
	next := nextRunAt(now, refreshHour, refreshMinute)
	expected := time.Date(2025, 4, 10, 11, 15, 0, 0, loc)
	assert.Equal(t, expected, next)
}

func TestNextRunAt_AfterTarget(t *testing.T) {
	loc := time.FixedZone("CET", 3600)
	now := time.Date(2025, 4, 10, 8, 0, 0, 0, loc)
	next := nextRunAt(now, refreshHour, refreshMinute)
	expected := time.Date(2025, 4, 10, 11, 15, 0, 0, loc)
	assert.Equal(t, expected, next)
}

package models

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNormalizeSummaryAndDescription_UsesKnownSummary(t *testing.T) {
	summary, description := NormalizeSummaryAndDescription("Korte samenvatting", "Uitgebreide beschrijving")

	require.NotNil(t, summary)
	assert.Equal(t, "Korte samenvatting", *summary)
	assert.Equal(t, "Uitgebreide beschrijving", description)
}

func TestNormalizeSummaryAndDescription_DerivesSummaryFromMarkdownDescription(t *testing.T) {
	summary, description := NormalizeSummaryAndDescription("", "# Titel\n\nDit is **belangrijk** met [link](https://example.com).")

	require.NotNil(t, summary)
	assert.Equal(t, "Titel Dit is belangrijk met link.", *summary)
	assert.Equal(t, "# Titel\n\nDit is **belangrijk** met [link](https://example.com).", description)
}

func TestNormalizeSummaryAndDescription_FillsDescriptionFromSummary(t *testing.T) {
	summary, description := NormalizeSummaryAndDescription("Alleen summary", "")

	require.NotNil(t, summary)
	assert.Equal(t, "Alleen summary", *summary)
	assert.Equal(t, "Alleen summary", description)
}

func TestNormalizeSummaryAndDescription_ReturnsNilSummaryWhenNoTextExists(t *testing.T) {
	summary, description := NormalizeSummaryAndDescription("", " ")

	assert.Nil(t, summary)
	assert.Empty(t, description)
}

func TestDeriveSummary_TruncatesLongDescriptions(t *testing.T) {
	text := strings.Repeat("a", SummaryMaxLength) + " extra tekst"

	summary := DeriveSummary(text)

	assert.Equal(t, strings.Repeat("a", SummaryMaxLength)+"...", summary)
}

func TestDeriveSummary_CompletesWordBeforeEllipsis(t *testing.T) {
	prefix := strings.Repeat("a", SummaryMaxLength-5) + " "
	text := prefix + "langwoord extra tekst"

	summary := DeriveSummary(text)

	assert.Equal(t, prefix+"langwoord...", summary)
}

func TestDeriveSummary_DoesNotAddEllipsisWhenCompletingLastWord(t *testing.T) {
	prefix := strings.Repeat("a", SummaryMaxLength-5) + " "
	text := prefix + "woord"

	summary := DeriveSummary(text)

	assert.Equal(t, text, summary)
}

func TestOptionalStringJSON(t *testing.T) {
	value := NewOptionalString("active")
	require.True(t, value.Set)
	require.NotNil(t, value.Value)
	assert.Equal(t, "active", *value.Value)

	nullValue := NewNullString()
	assert.True(t, nullValue.Set)
	assert.Nil(t, nullValue.Value)

	var decoded OptionalString
	require.NoError(t, decoded.UnmarshalJSON([]byte(`"deprecated"`)))
	require.True(t, decoded.Set)
	require.NotNil(t, decoded.Value)
	assert.Equal(t, "deprecated", *decoded.Value)

	require.NoError(t, decoded.UnmarshalJSON([]byte(`null`)))
	assert.True(t, decoded.Set)
	assert.Nil(t, decoded.Value)

	require.Error(t, decoded.UnmarshalJSON([]byte(`123`)))
}

func TestLifecycleStatus(t *testing.T) {
	now := time.Date(2026, 7, 8, 12, 0, 0, 0, time.UTC)

	assert.Equal(t, "sunset", Api{Sunset: "2026-07-09"}.LifecycleStatus(now))
	assert.Equal(t, "retired", Api{Sunset: "2026-07-07"}.LifecycleStatus(now))
	assert.Equal(t, "deprecated", Api{Deprecated: "2026-07-07"}.LifecycleStatus(now))
	assert.Equal(t, "retired", Api{Sunset: "not-a-date", Deprecated: "not-a-date"}.LifecycleStatus(now))
}

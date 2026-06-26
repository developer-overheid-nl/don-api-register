package models

import (
	"strings"
	"testing"

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

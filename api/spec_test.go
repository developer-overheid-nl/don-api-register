package apispec

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOpenAPIJSONReturnsCopy(t *testing.T) {
	first := OpenAPIJSON()
	require.NotEmpty(t, first)

	var spec map[string]any
	require.NoError(t, json.Unmarshal(first, &spec))
	assert.Contains(t, spec, "openapi")

	first[0] = 'x'
	second := OpenAPIJSON()
	assert.NotEqual(t, first[0], second[0])
}

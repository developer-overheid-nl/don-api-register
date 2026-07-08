package apispec

import (
	"encoding/json"
	"fmt"
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

func TestOpenAPIExamplesDoNotContainNull(t *testing.T) {
	var spec any
	require.NoError(t, json.Unmarshal(OpenAPIJSON(), &spec))

	var nullExamplePaths []string
	collectNullExamples(spec, "$", &nullExamplePaths)

	assert.Empty(t, nullExamplePaths, "null examples crash Super-Linter's Spectral OpenAPI summary")
}

func collectNullExamples(value any, path string, found *[]string) {
	switch typed := value.(type) {
	case map[string]any:
		for key, child := range typed {
			collectNullExamples(child, path+"."+key, found)
		}
	case []any:
		for index, child := range typed {
			childPath := fmt.Sprintf("%s[%d]", path, index)
			if child == nil && endsWithExamplesPath(path) {
				*found = append(*found, childPath)
				continue
			}
			collectNullExamples(child, childPath, found)
		}
	}
}

func endsWithExamplesPath(path string) bool {
	return len(path) >= len(".examples") && path[len(path)-len(".examples"):] == ".examples"
}

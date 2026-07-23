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

func TestCreateOrganisationRequestOnlyRequiresURI(t *testing.T) {
	var spec map[string]any
	require.NoError(t, json.Unmarshal(OpenAPIJSON(), &spec))

	paths := spec["paths"].(map[string]any)
	organisations := paths["/organisations"].(map[string]any)
	post := organisations["post"].(map[string]any)
	requestBody := post["requestBody"].(map[string]any)
	content := requestBody["content"].(map[string]any)
	jsonContent := content["application/json"].(map[string]any)
	schema := jsonContent["schema"].(map[string]any)
	assert.Equal(t, "#/components/schemas/OrganisationInput", schema["$ref"])

	components := spec["components"].(map[string]any)
	schemas := components["schemas"].(map[string]any)
	require.Contains(t, schemas, "OrganisationInput")
	input := schemas["OrganisationInput"].(map[string]any)
	assert.Equal(t, []any{"uri"}, input["required"])
	properties := input["properties"].(map[string]any)
	assert.Contains(t, properties, "label")
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

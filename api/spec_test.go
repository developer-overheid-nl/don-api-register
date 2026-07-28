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

func TestListApisDocumentsSorting(t *testing.T) {
	var spec map[string]any
	require.NoError(t, json.Unmarshal(OpenAPIJSON(), &spec))

	paths := spec["paths"].(map[string]any)
	listApis := paths["/apis"].(map[string]any)["get"].(map[string]any)
	parameters := listApis["parameters"].([]any)
	refs := make([]string, 0, len(parameters))
	for _, parameter := range parameters {
		refs = append(refs, parameter.(map[string]any)["$ref"].(string))
	}
	require.Contains(t, refs, "#/components/parameters/SortBy")
	require.Contains(t, refs, "#/components/parameters/SortOrder")

	components := spec["components"].(map[string]any)
	parameterDefinitions := components["parameters"].(map[string]any)

	sortBy := parameterDefinitions["SortBy"].(map[string]any)
	assert.Equal(t, "sortBy", sortBy["name"])
	assert.Equal(t, "query", sortBy["in"])
	sortBySchema := sortBy["schema"].(map[string]any)
	assert.Equal(t, []any{"title", "adrScore", "version"}, sortBySchema["enum"])
	assert.Equal(t, "title", sortBySchema["default"])

	sortOrder := parameterDefinitions["SortOrder"].(map[string]any)
	assert.Equal(t, "sortOrder", sortOrder["name"])
	assert.Equal(t, "query", sortOrder["in"])
	sortOrderSchema := sortOrder["schema"].(map[string]any)
	assert.Equal(t, []any{"asc", "desc"}, sortOrderSchema["enum"])
	assert.Equal(t, "asc", sortOrderSchema["default"])
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

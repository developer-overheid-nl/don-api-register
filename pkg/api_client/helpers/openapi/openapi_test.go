package openapi_test

import (
	"testing"
	"time"

	"github.com/developer-overheid-nl/don-api-register/pkg/api_client/helpers/openapi"
	"github.com/developer-overheid-nl/don-api-register/pkg/api_client/models"
	"github.com/getkin/kin-openapi/openapi3"
	"github.com/stretchr/testify/assert"
)

func TestValidateApi(t *testing.T) {
	api := &models.Api{}
	invalids := openapi.ValidateApi(api)
	assert.Len(t, invalids, 5)

	var names []string
	for _, inv := range invalids {
		names = append(names, inv.Name)
	}
	expected := []string{"contact.name", "contact.email", "contact.url", "oasUrl", "organisationUri"}
	for _, n := range expected {
		assert.Contains(t, names, n)
	}
}

func TestParseOutput(t *testing.T) {
	sample := "1:2  error  CODE1  Message one  path1\n3:4  warning  CODE1  Second msg  path2"
	msgs := openapi.ParseOutput(sample, time.Now())
	assert.Len(t, msgs, 1)
	assert.Equal(t, "CODE1", msgs[0].Code)
	assert.Len(t, msgs[0].Infos, 2)
}

func TestBuildApi(t *testing.T) {
	spec := &openapi3.T{
		Info: &openapi3.Info{Title: "T", Version: "1"},
	}
	req := models.ApiPost{OasUrl: "u", OrganisationUri: "org", Contact: models.Contact{Name: "n", Email: "e", URL: "u"}}
	api := openapi.BuildApi(spec, req, "label")
	assert.Equal(t, "T", api.Title)
	assert.Equal(t, "org", *api.OrganisationID)
}

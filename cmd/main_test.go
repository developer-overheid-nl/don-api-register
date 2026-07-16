package main

import (
	"errors"
	"reflect"
	"testing"

	"github.com/developer-overheid-nl/don-api-register/pkg/api_client/models"
	"github.com/go-playground/validator/v10"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInvalidParamsFromBindingMapsJSONTags(t *testing.T) {
	type commandInput struct {
		OasURL          string `json:"oasUrl" validate:"required_without=OasBody,url"`
		OasBody         string `json:"oasBody"`
		OrganisationURI string `json:"organisationUri" validate:"required,url"`
	}
	validate := validator.New()
	err := validate.Struct(commandInput{
		OrganisationURI: "not-a-url",
	})
	require.Error(t, err)

	got := invalidParamsFromBinding(err, commandInput{})

	byName := map[string]string{}
	for _, param := range got {
		byName[param.Name] = param.Reason
	}
	assert.Equal(t, "Moet een geldige URL zijn (bijv. https://…)", byName["organisationUri"])
	assert.Equal(t, "is verplicht wanneer geen OAS of lifecycle-datum is opgegeven", byName["oasUrl"])
}

func TestInvalidParamsFromBindingFallsBackForNonValidationError(t *testing.T) {
	got := invalidParamsFromBinding(errors.New("decode failed"), models.UpdateApiInput{})

	require.Len(t, got, 1)
	assert.Equal(t, "body", got[0].Name)
	assert.Equal(t, "decode failed", got[0].Reason)
}

func TestIsValidationErr(t *testing.T) {
	type commandInput struct {
		OrganisationURI string `validate:"required,url"`
	}
	validate := validator.New()
	err := validate.Struct(commandInput{OrganisationURI: "not-a-url"})

	assert.True(t, isValidationErr(err))
	assert.False(t, isValidationErr(errors.New("plain error")))
}

func TestHumanReason(t *testing.T) {
	type commandInput struct {
		Required string `validate:"required"`
		Date     string `validate:"datetime=2006-01-02"`
	}
	validate := validator.New()
	err := validate.Struct(commandInput{})
	require.Error(t, err)

	verrs := err.(validator.ValidationErrors)
	reasons := map[string]string{}
	for _, fe := range verrs {
		reasons[fe.Field()] = humanReason(fe)
	}
	assert.Equal(t, "is verplicht", reasons["Required"])
	assert.Equal(t, "Moet een geldige datum zijn (YYYY-MM-DD)", reasons["Date"])

	defaultReason := fakeFieldError{tag: "custom"}
	assert.Equal(t, defaultReason.Error(), humanReason(defaultReason))
}

type fakeFieldError struct {
	validator.FieldError
	tag string
}

func (f fakeFieldError) Tag() string         { return f.tag }
func (f fakeFieldError) Error() string       { return "custom validation failed" }
func (f fakeFieldError) Field() string       { return "field" }
func (f fakeFieldError) StructField() string { return "Field" }
func (f fakeFieldError) Value() interface{}  { return nil }
func (f fakeFieldError) Type() reflect.Type  { return reflect.TypeOf("") }

package helpers

import (
	"context"
	"github.com/getkin/kin-openapi/openapi3"
)

// ParseAndValidateOAS parses OpenAPI data and validates it.
func ParseAndValidateOAS(data []byte) (*openapi3.T, error) {
	loader := openapi3.NewLoader()
	loader.IsExternalRefsAllowed = true
	spec, err := loader.LoadFromData(data)
	if err != nil {
		return nil, err
	}
	if err := loader.ResolveRefsIn(spec, nil); err != nil {
		return nil, err
	}
	if err := spec.Validate(context.Background()); err != nil {
		return nil, err
	}
	return spec, nil
}

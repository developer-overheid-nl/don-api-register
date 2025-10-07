package openapi

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"
)

type FetchOpts struct {
	Origin     string       // bv. "https://developer.overheid.nl"
	HTTPClient *http.Client // optioneel
}

type OASResult struct {
	Spec *openapi3.T
	Hash string
}

func FetchParseValidateAndHash(ctx context.Context, oasURL string, opts FetchOpts) (*OASResult, error) {
	loader := openapi3.NewLoader()
	loader.IsExternalRefsAllowed = true

	u, _ := url.Parse(oasURL)
	spec, err := loader.LoadFromURI(u)
	if err != nil {
		return nil, err
	}
	if err := loader.ResolveRefsIn(spec, nil); err != nil {
		return nil, err
	}

	version := strings.TrimSpace(spec.OpenAPI)
	if version == "" {
		return nil, fmt.Errorf("invalid OAS: ontbrekende openapi versie")
	}

	if !strings.HasPrefix(version, "3.0") && !strings.HasPrefix(version, "3.1") {
		return nil, fmt.Errorf("invalid OAS: OpenAPI versie %q niet ondersteund (alleen 3.0 en 3.1)", version)
	}

	vopts := []openapi3.ValidationOption{openapi3.DisableExamplesValidation()}
	if strings.HasPrefix(version, "3.1") {
		if err := spec.Validate(ctx, vopts...); err != nil {
			if basicErr := ensureBasicOpenAPIStructure(spec); basicErr != nil {
				return nil, fmt.Errorf("invalid OAS: %s", basicErr.Error())
			}
			log.Printf("[openapi] skipping strict validation for OpenAPI %s spec: %v", version, err)
		}
	} else {
		if err := spec.Validate(ctx, vopts...); err != nil {
			return nil, fmt.Errorf("invalid OAS: %s", strings.TrimSpace(err.Error()))
		}
	}

	h, err := hashSpec(spec)
	if err != nil {
		return nil, err
	}

	return &OASResult{Spec: spec, Hash: h}, nil
}

func ensureBasicOpenAPIStructure(spec *openapi3.T) error {
	if spec.Info == nil {
		return errors.New("info ontbreekt")
	}
	if spec.Paths == nil {
		return errors.New("paths ontbreekt")
	}
	return nil
}

func hashSpec(spec *openapi3.T) (string, error) {
	b, err := json.Marshal(spec)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:]), nil
}

package openapi

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
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

	var vopts []openapi3.ValidationOption
	vopts = append(vopts, openapi3.DisableExamplesValidation())
	if err := spec.Validate(ctx, vopts...); err != nil {
		return nil, fmt.Errorf("invalid OAS: %s", strings.TrimSpace(err.Error()))
	}

	h, err := hashSpec(spec)
	if err != nil {
		return nil, err
	}

	return &OASResult{Spec: spec, Hash: h}, nil
}

func hashSpec(spec *openapi3.T) (string, error) {
	b, err := json.Marshal(spec)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:]), nil
}

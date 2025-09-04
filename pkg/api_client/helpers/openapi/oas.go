package openapi

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"
)

type FetchOpts struct {
	Origin     string       // bv. "https://developer.overheid.nl"
	HTTPClient *http.Client // optioneel
}

type OASResult struct {
	Spec *openapi3.T
	Raw  []byte
	Hash string
}

func FetchParseValidateAndHash(ctx context.Context, oasURL string, opts FetchOpts) (*OASResult, error) {
	cli := opts.HTTPClient
	if cli == nil {
		cli = http.DefaultClient
	}

	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, oasURL, nil)
	if opts.Origin != "" {
		req.Header.Set("Origin", opts.Origin)
	}
	resp, err := cli.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("OAS download failed with status %d", resp.StatusCode)
	}
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	loader := openapi3.NewLoader()
	loader.IsExternalRefsAllowed = true

	spec, err := loader.LoadFromData(raw)
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

	return &OASResult{Spec: spec, Raw: raw, Hash: h}, nil
}

func hashSpec(spec *openapi3.T) (string, error) {
	b, err := json.Marshal(spec)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:]), nil
}

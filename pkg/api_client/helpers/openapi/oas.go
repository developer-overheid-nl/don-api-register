package openapi

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"

	"github.com/pb33f/libopenapi"
	validator "github.com/pb33f/libopenapi-validator"
	"github.com/pb33f/libopenapi/datamodel"
	v3 "github.com/pb33f/libopenapi/datamodel/high/v3"
)

type FetchOpts struct {
	Origin     string       // bv. "https://developer.overheid.nl"
	HTTPClient *http.Client // optioneel
}

type OASResult struct {
	Spec *v3.Document // high-level v3 model
	Hash string       // sha256 van de genormaliseerde spec
}

func FetchParseValidateAndHash(ctx context.Context, oasURL string, opts FetchOpts) (*OASResult, error) {
	cli := opts.HTTPClient
	if cli == nil {
		cli = http.DefaultClient
	}

	// 1) OAS bytes ophalen
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, oasURL, nil)
	if err != nil {
		return nil, err
	}
	if opts.Origin != "" {
		req.Header.Set("Origin", opts.Origin)
	}
	resp, err := cli.Do(req)
	if err != nil {
		return nil, fmt.Errorf("kan OAS niet ophalen: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("kan OAS niet ophalen: status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("kan OAS niet lezen: %w", err)
	}

	// 2) libopenapi config voor (remote) refs
	cfg := datamodel.DocumentConfiguration{
		AllowRemoteReferences: true,
		AllowFileReferences:   true,
	}

	// 3) Parse document met config
	doc, docErr := libopenapi.NewDocumentWithConfiguration(raw, &cfg)

	if docErr != nil {
		return nil, fmt.Errorf("invalid OAS (parse): %s", strings.TrimSpace(docErr.Error()))
	}

	// 4) Build high-level v3 model (lost refs op)
	model, buildErrs := doc.BuildV3Model()
	if buildErrs != nil {
		// libopenapi geeft een error; bundel kort samen
		var parts []string
		parts = append(parts, buildErrs.Error())
		return nil, fmt.Errorf("invalid OAS (model): %s", strings.Join(parts, "; "))
	}

	// 5) Valideer OAS 3.0/3.1 met libopenapi-validator
	docValidator, vErrs := validator.NewValidator(doc)
	if vErrs != nil && len(vErrs) > 0 {
		var parts []string
		for _, e := range vErrs {
			parts = append(parts, e.Error())
		}
		return nil, fmt.Errorf("validator init error: %s", strings.Join(parts, "; "))
	}
	ok, validationErrs := docValidator.ValidateDocument()
	if !ok {
		// maak nette, compacte foutmelding
		var parts []string
		for _, e := range validationErrs {
			// e.Message bevat de essentie; evt. kun je e.Locator ook tonen
			parts = append(parts, e.Message)
		}
		return nil, fmt.Errorf("invalid OAS: %s", strings.Join(parts, "; "))
	}

	// 6) Hash over de genormaliseerde weergave
	//    RenderJSON levert een deterministische representatie.
	rendered, err := model.Model.RenderJSON("  ")
	if err != nil || len(rendered) == 0 {
		log.Printf("[oas] RenderJSON failed (err=%v), fallback to raw bytes for hashing", err)
		rendered = raw
	}
	sum := sha256.Sum256(rendered)

	spec := model.Model
	return &OASResult{
		Spec: &spec,
		Hash: hex.EncodeToString(sum[:]),
	}, nil
}

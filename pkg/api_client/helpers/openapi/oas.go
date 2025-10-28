package openapi

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"net/http"
	"regexp"
	"strconv"
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
	Spec        *v3.Document // high-level v3 model
	Hash        string       // sha256 van de genormaliseerde spec
	Raw         []byte       // oorspronkelijke bytes zoals opgehaald
	ContentType string       // content-type header van de response (kan leeg zijn)
	Version     string       // volledige openapi versiestring, bv. 3.0.3
	Major       int
	Minor       int
	Patch       int
}

var versionPrefixPattern = regexp.MustCompile(`^(\d+)\.(\d+)`)

func FetchParseValidateAndHash(ctx context.Context, oasURL string, opts FetchOpts) (*OASResult, error) {
	cli := opts.HTTPClient
	if cli == nil {
		cli = http.DefaultClient
	}

	// 1) OAS bytes ophalen (met optionele Origin, en fallback zonder)
	type attempt struct {
		origin string
	}
	attempts := []attempt{{origin: opts.Origin}}
	if opts.Origin != "" {
		attempts = append(attempts, attempt{origin: ""})
	}
	var raw []byte
	var contentType string
	for i, att := range attempts {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, oasURL, nil)
		if err != nil {
			return nil, err
		}
		if att.origin != "" {
			req.Header.Set("Origin", att.origin)
		}
		resp, err := cli.Do(req)
		if err != nil {
			return nil, fmt.Errorf("kan OAS niet ophalen: %w", err)
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			return nil, fmt.Errorf("kan OAS niet ophalen: status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
		}
		contentType = resp.Header.Get("Content-Type")
		raw, err = io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return nil, fmt.Errorf("kan OAS niet lezen: %w", err)
		}
		originLabel := "zonder Origin"
		if att.origin != "" {
			originLabel = "met Origin"
		}
		if n := len(raw); n == 0 {
			log.Printf("[oas] fetched empty body %s from %s (status %d)", originLabel, oasURL, resp.StatusCode)
			if att.origin != "" && i == 0 {
				log.Printf("[oas] retrying fetch without Origin header for %s", oasURL)
				continue
			}
		} else {
			preview := raw
			if n > 128 {
				preview = raw[:128]
			}
			log.Printf("[oas] fetched %d bytes %s from %s (status %d): %.128q", n, originLabel, oasURL, resp.StatusCode, preview)
		}
		break
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
	if len(vErrs) > 0 {
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
	version := strings.TrimSpace(spec.Version)
	if version == "" {
		return nil, fmt.Errorf("invalid OAS: ontbrekende openapi versie")
	}
	match := versionPrefixPattern.FindStringSubmatch(version)
	if len(match) != 3 {
		return nil, fmt.Errorf("invalid OAS: ongeldige openapi versie %s", version)
	}
	major, _ := strconv.Atoi(match[1])
	minor, _ := strconv.Atoi(match[2])
	var patch int
	if parts := strings.SplitN(version, ".", 3); len(parts) == 3 {
		if v, err := strconv.Atoi(parts[2]); err == nil {
			patch = v
		}
	}
	if major != 3 || minor > 1 {
		return nil, fmt.Errorf("invalid OAS: unsupported OpenAPI version %s (alleen 3.0 en 3.1 worden ondersteund)", version)
	}
	return &OASResult{
		Spec:        &spec,
		Hash:        hex.EncodeToString(sum[:]),
		Raw:         raw,
		ContentType: contentType,
		Version:     version,
		Major:       major,
		Minor:       minor,
		Patch:       patch,
	}, nil
}

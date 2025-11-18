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

	"github.com/developer-overheid-nl/don-api-register/pkg/api_client/helpers/tools"
	"github.com/pb33f/libopenapi"
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

func FetchParseValidateAndHash(ctx context.Context, input tools.OASInput, opts FetchOpts) (*OASResult, error) {
	input.Normalize()
	if input.IsEmpty() {
		return nil, fmt.Errorf("OAS input ontbreekt")
	}

	raw, contentType, err := bundleOAS(ctx, input)
	if err != nil {
		log.Printf("[oas] bundle failed (%v), fallback naar directe fetch", err)
		raw, contentType, err = fetchRawOAS(ctx, input, opts)
		if err != nil {
			return nil, err
		}
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

func bundleOAS(ctx context.Context, input tools.OASInput) ([]byte, string, error) {
	data, contentType, err := tools.BundleOAS(ctx, input)
	if err != nil {
		return nil, "", err
	}
	log.Printf("[oas] bundle succeeded (len=%d, ct=%s)", len(data), contentType)
	return data, contentType, nil
}

func fetchRawOAS(ctx context.Context, input tools.OASInput, opts FetchOpts) ([]byte, string, error) {
	if body := strings.TrimSpace(input.OasBody); body != "" {
		raw := []byte(body)
		log.Printf("[oas] using inline body (%d bytes) for hashing", len(raw))
		return raw, "", nil
	}
	oasURL := strings.TrimSpace(input.OasUrl)
	if oasURL == "" {
		return nil, "", fmt.Errorf("geen oasUrl opgegeven")
	}
	cli := opts.HTTPClient
	if cli == nil {
		cli = http.DefaultClient
	}
	type attempt struct {
		origin string
	}
	attempts := []attempt{{origin: opts.Origin}}
	if opts.Origin != "" {
		attempts = append(attempts, attempt{origin: ""})
	}
	for i, att := range attempts {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, oasURL, nil)
		if err != nil {
			return nil, "", err
		}
		if att.origin != "" {
			req.Header.Set("Origin", att.origin)
		}
		resp, err := cli.Do(req)
		if err != nil {
			return nil, "", fmt.Errorf("kan OAS niet ophalen: %w", err)
		}
		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return nil, "", fmt.Errorf("kan OAS niet lezen: %w", err)
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return nil, "", fmt.Errorf("kan OAS niet ophalen: status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
		}
		contentType := resp.Header.Get("Content-Type")
		originLabel := "zonder Origin"
		if att.origin != "" {
			originLabel = "met Origin"
		}
		if n := len(body); n == 0 {
			log.Printf("[oas] fetched empty body %s from %s (status %d)", originLabel, oasURL, resp.StatusCode)
			if att.origin != "" && i == 0 {
				log.Printf("[oas] retrying fetch without Origin header for %s", oasURL)
				continue
			}
		} else {
			preview := body
			if n > 128 {
				preview = body[:128]
			}
			log.Printf("[oas] fetched %d bytes %s from %s (status %d): %.128q", n, originLabel, oasURL, resp.StatusCode, preview)
		}
		return body, contentType, nil
	}
	return nil, "", fmt.Errorf("kan OAS niet ophalen: geen geldige response")
}

package openapi

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"github.com/developer-overheid-nl/don-api-register/pkg/api_client/helpers/tools"
	"github.com/developer-overheid-nl/don-api-register/pkg/api_client/models"
	"github.com/pb33f/libopenapi"
	"github.com/pb33f/libopenapi/datamodel"
	"go.yaml.in/yaml/v4"
)

type FetchOpts = models.FetchOpts

type OASResult = models.OASResult

type HTTPStatusError = models.HTTPStatusError

func IsHTTPStatus(err error, statusCode int) bool {
	var statusErr *HTTPStatusError
	return errors.As(err, &statusErr) && statusErr.StatusCode == statusCode
}

var versionPrefixPattern = regexp.MustCompile(`^(\d+)\.(\d+)`)

const maxOASResponseBytes int64 = 20 << 20

var oasLifecycleMu sync.Mutex

// ProcessOAS owns one complete libopenapi document lifecycle. libopenapi uses
// process-global caches and node pools, so model consumers are serialized and
// the caches are cleared before another document can be processed.
func ProcessOAS(ctx context.Context, input tools.OASInput, opts FetchOpts, consume func(*OASResult) error) error {
	return processOASWithCleanup(ctx, input, opts, consume, libopenapi.ClearAllCaches)
}

func processOASWithCleanup(
	ctx context.Context,
	input tools.OASInput,
	opts FetchOpts,
	consume func(*OASResult) error,
	cleanup func(),
) error {
	oasLifecycleMu.Lock()
	defer oasLifecycleMu.Unlock()
	if cleanup != nil {
		defer cleanup()
	}

	res, err := FetchParseValidateAndHash(ctx, input, opts)
	if err != nil {
		return err
	}
	if consume == nil {
		return nil
	}
	return consume(res)
}

func FetchParseValidateAndHash(ctx context.Context, input tools.OASInput, opts FetchOpts) (*OASResult, error) {
	input.Normalize()
	if input.IsEmpty() {
		return nil, fmt.Errorf("OAS input ontbreekt")
	}

	var (
		raw         []byte
		contentType string
		fromBundle  bool
		err         error
	)

	raw, contentType, err = bundleOAS(ctx, input)
	if err != nil {
		bundleErr := err
		raw, contentType, err = fetchRawOAS(ctx, input, opts)
		if err != nil {
			return nil, fmt.Errorf("OAS bundling failed: %w; source document fetch failed: %w", bundleErr, err)
		}
		slog.DebugContext(
			ctx,
			"OAS bundling failed; falling back to source document",
			"component", "openapi",
			"operation", "bundle",
			"error", bundleErr,
		)
	} else {
		fromBundle = true
	}

	if fromBundle && hasRecursiveYAMLAlias(raw, contentType) {
		slog.DebugContext(
			ctx,
			"bundled OAS contains recursive YAML aliases; retrying source document",
			"component", "openapi",
			"operation", "parse",
		)
		raw, contentType, err = fetchRawOAS(ctx, input, opts)
		if err != nil {
			return nil, err
		}
		fromBundle = false
	}

	res, err := parseValidateAndHash(raw, contentType)
	if err == nil {
		return res, nil
	}
	if fromBundle && shouldRetryRawFetchAfterBundleParseError(err) {
		slog.DebugContext(
			ctx,
			"bundled OAS parse failed on recursive YAML anchors; retrying source document",
			"component", "openapi",
			"operation", "parse",
		)
		raw, contentType, retryErr := fetchRawOAS(ctx, input, opts)
		if retryErr != nil {
			return nil, retryErr
		}
		return parseValidateAndHash(raw, contentType)
	}
	return nil, err
}

func shouldRetryRawFetchAfterBundleParseError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "failed to decode yaml to json") &&
		strings.Contains(msg, "anchor") &&
		strings.Contains(msg, "contains itself")
}

func hasRecursiveYAMLAlias(raw []byte, contentType string) bool {
	if !strings.Contains(strings.ToLower(contentType), "yaml") {
		return false
	}
	var root yaml.Node
	if err := yaml.Unmarshal(raw, &root); err != nil {
		return false
	}
	visiting := make(map[*yaml.Node]bool)
	visited := make(map[*yaml.Node]bool)
	return yamlNodeHasCycle(&root, visiting, visited)
}

func yamlNodeHasCycle(node *yaml.Node, visiting, visited map[*yaml.Node]bool) bool {
	if node == nil {
		return false
	}
	if node.Kind == yaml.AliasNode {
		return yamlNodeHasCycle(node.Alias, visiting, visited)
	}
	if visiting[node] {
		return true
	}
	if visited[node] {
		return false
	}
	visiting[node] = true
	for _, child := range node.Content {
		if yamlNodeHasCycle(child, visiting, visited) {
			return true
		}
	}
	delete(visiting, node)
	visited[node] = true
	return false
}

func parseValidateAndHash(raw []byte, contentType string) (*OASResult, error) {
	// 2) libopenapi config voor (remote) refs
	cfg := datamodel.DocumentConfiguration{
		AllowRemoteReferences:         true,
		AllowFileReferences:           true,
		IgnoreArrayCircularReferences: true,
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
	canonicalJSON := rendered
	if err != nil || len(rendered) == 0 {
		slog.Warn(
			"canonical OAS rendering failed; hashing source bytes",
			"component", "openapi",
			"operation", "hash",
			"error", err,
		)
		rendered = raw
		canonicalJSON = nil
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
		Spec:          &spec,
		Hash:          hex.EncodeToString(sum[:]),
		Raw:           raw,
		CanonicalJSON: canonicalJSON,
		ContentType:   contentType,
		Version:       version,
		Major:         major,
		Minor:         minor,
		Patch:         patch,
	}, nil
}

func bundleOAS(ctx context.Context, input tools.OASInput) ([]byte, string, error) {
	data, contentType, err := tools.BundleOAS(ctx, input)
	if err != nil {
		return nil, "", err
	}
	slog.DebugContext(
		ctx,
		"OAS bundled",
		"component", "openapi",
		"operation", "bundle",
		"inline_body", strings.TrimSpace(input.OasBody) != "",
		"byte_count", len(data),
		"content_type", contentType,
	)
	return data, contentType, nil
}

func fetchRawOAS(ctx context.Context, input tools.OASInput, opts FetchOpts) ([]byte, string, error) {
	if body := strings.TrimSpace(input.OasBody); body != "" {
		raw := []byte(body)
		slog.DebugContext(
			ctx,
			"using inline OAS document",
			"component", "openapi",
			"operation", "fetch",
			"byte_count", len(raw),
		)
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
	origins := []string{opts.Origin}
	if opts.Origin != "" {
		origins = append(origins, "")
	}
	for i, origin := range origins {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, oasURL, nil)
		if err != nil {
			return nil, "", err
		}
		if origin != "" {
			req.Header.Set("Origin", origin)
		}
		resp, err := cli.Do(req)
		if err != nil {
			return nil, "", fmt.Errorf("kan OAS niet ophalen: %w", err)
		}
		body, readErr := io.ReadAll(io.LimitReader(resp.Body, maxOASResponseBytes+1))
		closeErr := resp.Body.Close()
		if readErr != nil {
			return nil, "", fmt.Errorf("kan OAS niet lezen: %w", readErr)
		}
		if int64(len(body)) > maxOASResponseBytes {
			return nil, "", fmt.Errorf("OAS response exceeds 20 MiB: %s", oasURL)
		}
		if closeErr != nil {
			return nil, "", fmt.Errorf("kan OAS response body niet sluiten: %w", closeErr)
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return nil, "", &HTTPStatusError{
				StatusCode: resp.StatusCode,
				Body:       string(body),
			}
		}
		contentType := resp.Header.Get("Content-Type")
		if n := len(body); n == 0 {
			slog.DebugContext(
				ctx,
				"received empty OAS response",
				"component", "openapi",
				"operation", "fetch",
				"origin_header", origin != "",
				"status_code", resp.StatusCode,
			)
			if origin != "" && i == 0 {
				slog.DebugContext(
					ctx,
					"retrying OAS fetch without Origin header",
					"component", "openapi",
					"operation", "fetch",
				)
				continue
			}
		} else {
			slog.DebugContext(
				ctx,
				"OAS fetched",
				"component", "openapi",
				"operation", "fetch",
				"origin_header", origin != "",
				"status_code", resp.StatusCode,
				"byte_count", n,
				"content_type", contentType,
			)
		}
		return body, contentType, nil
	}
	return nil, "", fmt.Errorf("kan OAS niet ophalen: geen geldige response")
}

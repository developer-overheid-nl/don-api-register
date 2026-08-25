package models

import (
	"fmt"
	"net/http"
	"strings"

	v3 "github.com/pb33f/libopenapi/datamodel/high/v3"
)

type FetchOpts struct {
	Origin     string       // bv. "https://developer.overheid.nl"
	HTTPClient *http.Client // optioneel
}

type OASResult struct {
	Spec          *v3.Document // high-level v3 model
	Hash          string       // sha256 van de genormaliseerde spec
	Raw           []byte       // oorspronkelijke bytes zoals opgehaald
	CanonicalJSON []byte       // canonieke JSON-weergave van het geparseerde model
	ContentType   string       // content-type header van de response (kan leeg zijn)
	Version       string       // volledige openapi versiestring, bv. 3.0.3
	Major         int
	Minor         int
	Patch         int
}

type OASRefreshResult struct {
	CandidateCount   int
	ProcessedCount   int
	UpdatedCount     int
	UnavailableCount int
	FailedCount      int
}

type HTTPStatusError struct {
	StatusCode int
	Body       string
}

func (e *HTTPStatusError) Error() string {
	return fmt.Sprintf("kan OAS niet ophalen: status %d: %s", e.StatusCode, strings.TrimSpace(e.Body))
}

type OpenAPIInfo struct {
	Info struct {
		Version string `json:"version"`
	} `json:"info"`
}

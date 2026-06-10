package httpclient

import (
	"context"
	"net/http"

	commonhttp "github.com/developer-overheid-nl/don-register-common/httpclient"
)

// CorsGet performs a GET request including an Origin header.
func CorsGet(c *http.Client, u string, corsurl string) (*http.Response, error) {
	return commonhttp.CorsGet(c, u, corsurl)
}

type TooIGraph = commonhttp.TooIGraph

type TooIObject = commonhttp.TooIObject

var HTTPClient = commonhttp.HTTPClient

// FetchOrganisationLabel retrieves the organisation label from the TOOI service.
func FetchOrganisationLabel(ctx context.Context, uriOrType string, optionalId ...string) (label string, err error) {
	commonhttp.HTTPClient = HTTPClient
	return commonhttp.FetchOrganisationLabel(ctx, uriOrType, optionalId...)
}

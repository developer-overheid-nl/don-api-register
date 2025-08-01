package httpclient

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// CorsGet performs a GET request including an Origin header.
func CorsGet(c *http.Client, u string, corsurl string) (*http.Response, error) {
	req, err := http.NewRequest(http.MethodGet, u, http.NoBody)
	if err != nil {
		return nil, err
	}
	req.Header.Add("Origin", corsurl)
	return c.Do(req)
}

type TooIGraph struct {
	Graph []TooIObject `json:"@graph"`
}

type TooIObject struct {
	ID    string `json:"@id"`
	Label []struct {
		Value    string `json:"@value"`
		Language string `json:"@language"`
	} `json:"http://www.w3.org/2000/01/rdf-schema#label"`
}

var HTTPClient = http.DefaultClient

// FetchOrganisationLabel retrieves the organisation label from the TOOI service.
func FetchOrganisationLabel(ctx context.Context, uriOrType string, optionalId ...string) (string, error) {
	var uri string
	if strings.HasPrefix(uriOrType, "https://identifier.overheid.nl/tooi/id/") {
		uri = uriOrType
	} else if len(optionalId) > 0 {
		uri = fmt.Sprintf("https://identifier.overheid.nl/tooi/id/%s/%s", uriOrType, optionalId[0])
	} else {
		return "", fmt.Errorf("ongeldig argument, geef een volledige URI of (type, id)")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, uri, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/ld+json")

	resp, err := HTTPClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("organisation not found: %s", uri)
	}

	var arr []TooIGraph
	if err := json.NewDecoder(resp.Body).Decode(&arr); err != nil {
		return "", fmt.Errorf("decode error: %w", err)
	}
	if len(arr) == 0 || len(arr[0].Graph) == 0 {
		return "", fmt.Errorf("geen organisatie gevonden in TOOI")
	}
	for _, obj := range arr[0].Graph {
		if obj.ID == uri {
			for _, lbl := range obj.Label {
				if lbl.Language == "nl" {
					return lbl.Value, nil
				}
			}
			if len(obj.Label) > 0 {
				return obj.Label[0].Value, nil
			}
			return "", fmt.Errorf("geen label gevonden voor %s", uri)
		}
	}
	return "", fmt.Errorf("organisatie %s niet gevonden in response", uri)
}

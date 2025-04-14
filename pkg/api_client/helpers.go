/*
 * API register API v1
 *
 * API van het API register (apis.developer.overheid.nl)
 *
 * API version: 1.0.0
 * Contact: developer.overheid@geonovum.nl
 */

package api_client

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
)

var (
	ErrInvalidContentType = errors.New("Content-Type header is not application/json")
	ErrEmptyBody          = errors.New("request body is empty")
)

func DecodeJSONBody(w http.ResponseWriter, r *http.Request, dst interface{}) error {
	if r.Header.Get("Content-Type") != "" && r.Header.Get("Content-Type") != "application/json" {
		return ErrInvalidContentType
	}

	if r.Body == nil {
		return ErrEmptyBody
	}
	defer r.Body.Close()

	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields() // extra veiligheid

	if err := decoder.Decode(&dst); err != nil {
		return fmt.Errorf("could not decode JSON: %w", err)
	}

	return nil
}

type OpenAPIInfo struct {
	Info struct {
		Version string `json:"version"`
	} `json:"info"`
}

func LoadOASVersion(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("could not open OAS file: %w", err)
	}
	defer f.Close()

	var oas OpenAPIInfo
	if err := json.NewDecoder(f).Decode(&oas); err != nil {
		return "", fmt.Errorf("could not parse OAS: %w", err)
	}

	if oas.Info.Version == "" {
		return "", fmt.Errorf("version missing from OAS")
	}

	return oas.Info.Version, nil
}

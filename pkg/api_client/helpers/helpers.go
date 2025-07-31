/*
 * API register API v1
 *
 * API van het API register (apis.developer.overheid.nl)
 *
 * API version: 1.0.0
 * Contact: developer.overheid@geonovum.nl
 */

package helpers

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/developer-overheid-nl/don-api-register/pkg/api_client/models"
	"io"
	"net/http"
	"os"
)

var (
	ErrInvalidContentType = errors.New("Content-Type header is not application/json")
	ErrEmptyBody          = errors.New("request body is empty")
)

func EncodeJSONResponse(i interface{}, status *int, w http.ResponseWriter) error {
	wHeader := w.Header()

	f, ok := i.(*os.File)
	if ok {
		data, err := io.ReadAll(f)
		if err != nil {
			return err
		}
		wHeader.Set("Content-Type", http.DetectContentType(data))
		wHeader.Set("Content-Disposition", "attachment; filename="+f.Name())
		if status != nil {
			w.WriteHeader(*status)
		} else {
			w.WriteHeader(http.StatusOK)
		}
		_, err = w.Write(data)
		return err
	}
	wHeader.Set("Content-Type", "application/json; charset=UTF-8")

	if status != nil {
		w.WriteHeader(*status)
	} else {
		w.WriteHeader(http.StatusOK)
	}

	if i != nil {
		return json.NewEncoder(w).Encode(i)
	}

	return nil
}

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

func ToApiSummary(api *models.Api) models.ApiSummary {
	return models.ApiSummary{
		Id:          api.Id,
		OasUrl:      api.OasUri,
		Title:       api.Title,
		Description: api.Description,
		Contact: models.Contact{
			Name:  api.ContactName,
			URL:   api.ContactUrl,
			Email: api.ContactEmail,
		},
		Organisation: models.Organisation{
			Label: api.Organisation.Label,
			Uri:   api.Organisation.Uri,
		},
		AdrScore: api.AdrScore,
		Links: &models.Links{
			Self: &models.Link{Href: fmt.Sprintf("/apis/%s", api.Id)},
		},
	}
}

func ToApiDetail(api *models.Api) *models.ApiDetail {
	return &models.ApiDetail{
		ApiSummary: ToApiSummary(api),
		DocsUri:    api.DocsUri,
		Servers:    api.Servers,
	}
}

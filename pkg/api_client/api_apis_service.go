/*
 * API register API v1
 *
 * API van het API register (apis.developer.overheid.nl)
 *
 * API version: 1.0.0
 * Contact: developer.overheid@geonovum.nl
 */

package openapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
)

// APIsAPIService is a service that implements the logic for the APIsAPIServicer
// This service should implement the business logic for every endpoint for the APIsAPI API.
// Include any external packages or services that will be required by this service.
type APIsAPIService struct {
}

// NewAPIsAPIService creates a default api service
func NewAPIsAPIService() *APIsAPIService {
	return &APIsAPIService{}
}

func (s *APIsAPIService) ListApis(ctx context.Context) (ImplResponse, error) {
	// De backend URL waar je API draait
	url := "http://localhost:8000/api/v1/apis"

	// HTTP GET request
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return Response(http.StatusInternalServerError, nil), fmt.Errorf("failed to create request: %w", err)
	}

	// Client gebruiken
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return Response(http.StatusBadGateway, nil), fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	// Check of de statuscode correct is
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return Response(resp.StatusCode, nil), fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(body))
	}

	// Decode JSON
	var apis []Api
	if err := json.NewDecoder(resp.Body).Decode(&apis); err != nil {
		return Response(http.StatusInternalServerError, nil), fmt.Errorf("failed to decode response: %w", err)
	}

	// Geef de succesvolle response terug
	return Response(http.StatusOK, apis), nil
}

// RetrieveApi - API ophalen
func (s *APIsAPIService) RetrieveApi(ctx context.Context, id string) (ImplResponse, error) {
	// TODO - update RetrieveApi with the required logic for this service method.
	// Add api_apis_service.go to the .openapi-generator-ignore to avoid overwriting this service implementation when updating open api generation.

	// TODO: Uncomment the next line to return response Response(200, Api{}) or use other options such as http.Ok ...
	// return Response(200, Api{}), nil

	// TODO: Uncomment the next line to return response Response(404, {}) or use other options such as http.Ok ...
	// return Response(404, nil),nil

	return Response(http.StatusNotImplemented, nil), errors.New("RetrieveApi method not implemented")
}

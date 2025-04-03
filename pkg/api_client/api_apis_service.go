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
	"fmt"
	"net/http"
	"os"
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
	file, err := os.Open("./api/mock_apis.json")
	if err != nil {
		return Response(http.StatusInternalServerError, nil), fmt.Errorf("failed to open mock_apis.json: %w", err)
	}
	defer file.Close()

	var apis []Api
	if err := json.NewDecoder(file).Decode(&apis); err != nil {
		return Response(http.StatusInternalServerError, nil), fmt.Errorf("failed to decode mock_apis.json: %w", err)
	}

	return Response(http.StatusOK, apis), nil
}

func (s *APIsAPIService) RetrieveApi(ctx context.Context, id string) (ImplResponse, error) {
	file, err := os.Open("./api/mock_api.json")
	if err != nil {
		return Response(http.StatusInternalServerError, nil), fmt.Errorf("failed to open mock_api.json: %w", err)
	}
	defer file.Close()

	var api Api
	if err := json.NewDecoder(file).Decode(&api); err != nil {
		return Response(http.StatusInternalServerError, nil), fmt.Errorf("failed to decode mock_api.json: %w", err)
	}

	return Response(http.StatusOK, api), nil
}

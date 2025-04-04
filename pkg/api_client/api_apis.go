// Code generated by OpenAPI Generator (https://openapi-generator.tech); DO NOT EDIT.

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
	"net/http"
	"strings"

	"github.com/gorilla/mux"
)

// APIsAPIController binds http requests to an api service and writes the service results to the http response
type APIsAPIController struct {
	service APIsAPIServicer
	errorHandler ErrorHandler
}

// APIsAPIOption for how the controller is set up.
type APIsAPIOption func(*APIsAPIController)

// WithAPIsAPIErrorHandler inject ErrorHandler into controller
func WithAPIsAPIErrorHandler(h ErrorHandler) APIsAPIOption {
	return func(c *APIsAPIController) {
		c.errorHandler = h
	}
}

// NewAPIsAPIController creates a default api controller
func NewAPIsAPIController(s APIsAPIServicer, opts ...APIsAPIOption) *APIsAPIController {
	controller := &APIsAPIController{
		service:      s,
		errorHandler: DefaultErrorHandler,
	}

	for _, opt := range opts {
		opt(controller)
	}

	return controller
}

// Routes returns all the api routes for the APIsAPIController
func (c *APIsAPIController) Routes() Routes {
	return Routes{
		"ListApis": Route{
			strings.ToUpper("Get"),
			"/apis/v1/apis",
			c.ListApis,
		},
		"RetrieveApi": Route{
			strings.ToUpper("Get"),
			"/apis/v1/apis/{id}",
			c.RetrieveApi,
		},
	}
}

// ListApis - Alle API's ophalen
func (c *APIsAPIController) ListApis(w http.ResponseWriter, r *http.Request) {
	result, err := c.service.ListApis(r.Context())
	// If an error occurred, encode the error with the status code
	if err != nil {
		c.errorHandler(w, r, err, &result)
		return
	}
	// If no error, encode the body and the result code
	_ = EncodeJSONResponse(result.Body, &result.Code, w)
}

// RetrieveApi - API ophalen
func (c *APIsAPIController) RetrieveApi(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	idParam := params["id"]
	if idParam == "" {
		c.errorHandler(w, r, &RequiredError{"id"}, nil)
		return
	}
	result, err := c.service.RetrieveApi(r.Context(), idParam)
	// If an error occurred, encode the error with the status code
	if err != nil {
		c.errorHandler(w, r, err, &result)
		return
	}
	// If no error, encode the body and the result code
	_ = EncodeJSONResponse(result.Body, &result.Code, w)
}

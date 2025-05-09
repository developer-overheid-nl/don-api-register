package handler

import (
	"encoding/json"
	"fmt"
	"github.com/developer-overheid-nl/don-api-register/pkg/api_client/models"
	"github.com/developer-overheid-nl/don-api-register/pkg/api_client/services"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/developer-overheid-nl/don-api-register/pkg/api_client"
	"github.com/gorilla/mux"
)

// APIsAPIController bindt HTTP-verzoeken aan de APIsAPIService
type APIsAPIController struct {
	service      *services.APIsAPIService
	errorHandler api_client.ErrorHandler
}

// Constructor voor de controller
func NewAPIsAPIController(s *services.APIsAPIService) *APIsAPIController {
	controller := &APIsAPIController{
		service:      s,
		errorHandler: api_client.DefaultErrorHandler,
	}
	return controller
}

// ListApis - Alle API's ophalen met paginering
func (c *APIsAPIController) ListApis(w http.ResponseWriter, r *http.Request) {
	page, err := strconv.Atoi(r.URL.Query().Get("page"))
	if err != nil || page < 1 {
		page = 1
	}

	perPage, err := strconv.Atoi(r.URL.Query().Get("perPage"))
	if err != nil || perPage < 1 {
		perPage = 10
	}

	response, err := c.service.ListApis(r.Context(), page, perPage)
	if err != nil {
		c.errorHandler(w, r, err, &api_client.ImplResponse{Code: http.StatusInternalServerError})
		return
	}

	status := http.StatusOK
	err = api_client.EncodeJSONResponse(response, &status, w)
	if err != nil {
		return
	}
}

// RetrieveApi - Specifieke API ophalen op basis van ID
func (c *APIsAPIController) RetrieveApi(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	idParam := params["id"]
	println(idParam)
	if idParam == "" {
		c.errorHandler(w, r, &api_client.RequiredError{Field: "id"}, nil)
		return
	}

	api, err := c.service.RetrieveApi(r.Context(), idParam)
	if err != nil {
		c.errorHandler(w, r, err, &api_client.ImplResponse{Code: http.StatusInternalServerError})
		return
	}

	if api == nil {
		http.Error(w, "API not found", http.StatusNotFound)
		return
	}

	status := http.StatusOK
	err = api_client.EncodeJSONResponse(api, &status, w)
	if err != nil {
		return
	}
}

func (c *APIsAPIController) CreateApiFromOas(w http.ResponseWriter, r *http.Request) {
	var body models.Api
	if err := api_client.DecodeJSONBody(w, r, &body); err != nil {
		log.Printf("DecodeJSONBody error: %v", err)
		c.errorHandler(w, r, err, &api_client.ImplResponse{Code: http.StatusBadRequest})
		return
	}

	api, missing, err := c.service.CreateApiFromOas(body)
	if err != nil {
		log.Printf("CreateApiFromOas error: %v", err)

		if len(missing) > 0 {
			w.Header().Set("Content-Type", "application/problem+json")
			w.WriteHeader(http.StatusBadRequest)

			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"missingProperties": missing,
				"message":           fmt.Sprintf("De volgende gegevens ontbreken: %s", strings.Join(missing, ", ")),
			})
			return
		}

		c.errorHandler(w, r, err, &api_client.ImplResponse{Code: http.StatusUnprocessableEntity})
		return
	}

	status := http.StatusCreated
	_ = api_client.EncodeJSONResponse(api, &status, w)
}

func (c *APIsAPIController) ServeOASFile(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "./api/openapi.json")
}

func (c *APIsAPIController) UpdateApi(w http.ResponseWriter, r *http.Request) {
	var apiObj models.Api

	if err := json.NewDecoder(r.Body).Decode(&apiObj); err != nil {
		c.errorHandler(w, r, err, &api_client.ImplResponse{
			Code: http.StatusBadRequest,
			Body: map[string]string{"error": "Invalid JSON payload"},
		})
		return
	}

	err := c.service.UpdateApi(r.Context(), apiObj)
	if err != nil {
		println(err)
		c.errorHandler(w, r, err, &api_client.ImplResponse{
			Code: http.StatusInternalServerError,
			Body: map[string]string{"error": "Failed to update or insert API"},
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(apiObj)
}

func (c *APIsAPIController) Routes() api_client.Routes {
	return api_client.Routes{
		"ListApis": api_client.Route{
			Method:      http.MethodGet,
			Pattern:     "/v1/apis",
			HandlerFunc: c.ListApis,
		},
		"RetrieveApi": api_client.Route{
			Method:      http.MethodGet,
			Pattern:     "/v1/api/{id}",
			HandlerFunc: c.RetrieveApi,
		},
		"UpdateApi": api_client.Route{
			Method:      http.MethodPut,
			Pattern:     "/v1/api/{id}",
			HandlerFunc: c.UpdateApi,
		},
		"CreateApiFromOas": api_client.Route{
			Method:      http.MethodPost,
			Pattern:     "/v1/apis",
			HandlerFunc: c.CreateApiFromOas,
		},
		"ServeOASFile": api_client.Route{
			Method:      http.MethodGet,
			Pattern:     "/openapi.json",
			HandlerFunc: c.ServeOASFile,
		},
	}
}

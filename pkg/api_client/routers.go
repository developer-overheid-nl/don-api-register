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
	"github.com/gorilla/mux"
	"io"
	"net/http"
	"os"
)

// A Route defines the parameters for an api endpoint
type Route struct {
	Method      string
	Pattern     string
	HandlerFunc http.HandlerFunc
}

// Routes is a map of defined api endpoints
type Routes map[string]Route

// Router defines the required methods for retrieving api routes
type Router interface {
	Routes() Routes
}

// NewRouter creates a new router for any number of api routers
func NewRouter(apiVersion string, routers ...Router) *mux.Router {
	router := mux.NewRouter().StrictSlash(true)
	for _, api := range routers {
		for name, route := range api.Routes() {
			var handler http.Handler = route.HandlerFunc
			handler = Logger(handler, name)
			handler = addAPIVersionHeader(apiVersion, handler)

			router.
				Methods(route.Method).
				Path(route.Pattern).
				Name(name).
				Handler(handler)
		}
	}
	return router
}

type statusTrackingResponseWriter struct {
	http.ResponseWriter
	status     int
	apiVersion string
	headerSent bool
}

func (w *statusTrackingResponseWriter) WriteHeader(code int) {
	if !w.headerSent && code >= 200 && code < 300 {
		w.Header().Set("API-Version", w.apiVersion)
		w.headerSent = true
	}
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

func addAPIVersionHeader(version string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rec := &statusTrackingResponseWriter{
			ResponseWriter: w,
			status:         http.StatusOK,
			apiVersion:     version,
		}
		next.ServeHTTP(rec, r)
	})
}

// EncodeJSONResponse uses the json encoder to write an interface to the http response with an optional status code
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

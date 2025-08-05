package api_client

import (
	"github.com/developer-overheid-nl/don-api-register/pkg/api_client/handler"
	"github.com/developer-overheid-nl/don-api-register/pkg/api_client/middleware"
	"github.com/gin-gonic/gin"
	"github.com/loopfz/gadgeto/tonic"
	"github.com/wI2L/fizz"
	"github.com/wI2L/fizz/openapi"
)

var (
	apiVersionHeader = fizz.Header(
		"API-Version",
		"De API-versie van de response",
		"", // lege string betekent: primitive string in de spec
	)

	notFoundResponse = fizz.Response(
		"404",
		"Not Found",
		nil, // geen inline schema
		nil, // geen content-media-type
		nil, // geen extra headers
	)
)

func NewRouter(apiVersion string, controller *handler.APIsAPIController) *fizz.Fizz {
	// 0) Gin + Fizz init
	//gin.SetMode(gin.ReleaseMode)
	g := gin.Default()
	g.Use(APIVersionMiddleware(apiVersion))
	f := fizz.NewFromEngine(g)

	// 1) Voeg je Server-url toe (inclusief version path)
	f.Generator().SetServers([]*openapi.Server{
		{
			URL:         "https://api.developer.overheid.nl/v1",
			Description: "Production",
		},
	})

	// 2) Definieer je API-Version header in de global components
	gen := f.Generator()

	gen.API().Components.Responses["404"] = &openapi.ResponseOrRef{
		Reference: &openapi.Reference{
			Ref: "https://static.developer.overheid.nl/adr/components.yaml#/responses/404",
		},
	}

	gen.API().Components.Headers["API-Version"] = &openapi.HeaderOrRef{
		Header: &openapi.Header{
			Description: "De API-versie van de response",
			Schema: &openapi.SchemaOrRef{
				Schema: &openapi.Schema{
					Type: "string",
				},
			},
		},
	}

	// 4) Basis-info van je API
	info := &openapi.Info{
		Title:       "API register API v1",
		Description: "API van het API register (apis.developer.overheid.nl)",
		Version:     apiVersion,
		Contact: &openapi.Contact{
			Name:  "Team developer.overheid.nl",
			Email: "developer@overheid.nl",
			URL:   "https://apis.developer.overheid.nl",
		},
	}

	root := f.Group("/v1", "API v1", "API Register V1 routes")

	// 5a) Alleen-lezen endpoints
	read := root.Group("", "Lezen", "Alleen lezen endpoints", middleware.RequireAccess("apis:read"))
	read.GET("/apis",
		[]fizz.OperationOption{
			fizz.Summary("Alle API's ophalen"),
			apiVersionHeader,
			notFoundResponse,
		},
		tonic.Handler(controller.ListApis, 200),
	)

	read.GET("/apis/:id",
		[]fizz.OperationOption{
			fizz.Summary("Specifieke API ophalen"),
			apiVersionHeader,
			notFoundResponse,
		},
		tonic.Handler(controller.RetrieveApi, 200),
	)

	read.GET("/organisations",
		[]fizz.OperationOption{
			fizz.Summary("Alle organisations's ophalen"),
			apiVersionHeader,
			notFoundResponse,
		},
		tonic.Handler(controller.ListOrganisations, 200),
	)

	// 5b) Schrijf-endpoints
	write := root.Group("", "Schrijven", "Bewerken van API's", middleware.RequireAccess("apis:write"))
	write.POST("/apis",
		[]fizz.OperationOption{
			fizz.Summary("Registreer een nieuwe API met een OpenAPI URL"),
			apiVersionHeader,
			notFoundResponse,
		},
		tonic.Handler(controller.CreateApiFromOas, 201),
	)

	write.PUT("/apis/:id",
		[]fizz.OperationOption{
			fizz.Summary("Forceer de linter aan te roepen van een API"),
			apiVersionHeader,
			notFoundResponse,
		},
		tonic.Handler(controller.UpdateApi, 201),
	)

	// 6) OpenAPI documentatie
	f.GET("/v1/openapi.json", []fizz.OperationOption{}, f.OpenAPI(info, "json"))

	return f
}

type apiVersionWriter struct {
	gin.ResponseWriter
	version string
}

func (w *apiVersionWriter) WriteHeader(code int) {
	if code >= 200 && code < 300 {
		w.Header().Set("API-Version", w.version)
	}
	w.ResponseWriter.WriteHeader(code)
}

func APIVersionMiddleware(version string) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer = &apiVersionWriter{c.Writer, version}
		c.Next()
	}
}

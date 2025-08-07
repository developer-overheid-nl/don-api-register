package api_client

import (
	"github.com/developer-overheid-nl/don-api-register/pkg/api_client/handler"
	"github.com/gin-gonic/gin"
	"github.com/loopfz/gadgeto/tonic"
	"github.com/wI2L/fizz"
	"github.com/wI2L/fizz/openapi"
)

var (
	apiVersionHeader = fizz.Header(
		"API-Version",
		"De API-versie van de response",
		"",
	)

	notFoundResponse = fizz.Response(
		"404",
		"Not Found",
		nil,
		nil,
		nil,
	)
)

func NewRouter(apiVersion string, controller *handler.APIsAPIController) *fizz.Fizz {
	//gin.SetMode(gin.ReleaseMode)
	g := gin.Default()
	g.Use(APIVersionMiddleware(apiVersion))
	f := fizz.NewFromEngine(g)

	f.Generator().SetServers([]*openapi.Server{
		{
			URL:         "https://api.developer.overheid.nl/api-register/v1",
			Description: "Production",
		},
		{
			URL:         "https://api-register.don.apps.digilab.network/api-register/v1",
			Description: "Test",
		},
	})

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
					Type:    "string",
					Example: "1.0.0",
				},
			},
		},
	}

	info := &openapi.Info{
		Title:       "API register API v1",
		Description: "API van het API register (apis.developer.overheid.nl)",
		Version:     apiVersion,
		Contact: &openapi.Contact{
			Name:  "Team developer.overheid.nl",
			Email: "developer.overheid@geonovum.nl",
			URL:   "https://github.com/developer-overheid-nl/don-api-register/issues",
		},
	}

	root := f.Group("/v1", "API v1", "API Register V1 routes")

	read := root.Group("", "Publieke endpoints", "Alleen lezen endpoints")
	read.GET("/apis",
		[]fizz.OperationOption{
			fizz.ID("listApis"),
			fizz.Summary("Alle API's ophalen"),
			fizz.Description("Geeft een lijst met alle geregistreerde API's terug."),
			fizz.Security(&openapi.SecurityRequirement{
				"apiKey":            {},
				"clientCredentials": {"apis:read"},
			}),
			apiVersionHeader,
			notFoundResponse,
		},
		tonic.Handler(controller.ListApis, 200),
	)

	read.GET("/apis/:id",
		[]fizz.OperationOption{
			fizz.ID("retrieveApi"),
			fizz.Summary("Specifieke API ophalen"),
			fizz.Description("Geeft de details van een specifieke API terug."),
			fizz.Security(&openapi.SecurityRequirement{
				"apiKey":            {},
				"clientCredentials": {"apis:read"},
			}),
			apiVersionHeader,
			notFoundResponse,
		},
		tonic.Handler(controller.RetrieveApi, 200),
	)

	readOrg := root.Group("", "Private endpoints", "Alleen lezen endpoints")
	readOrg.GET("/organisations",
		[]fizz.OperationOption{
			fizz.ID("listOrganisations"),
			fizz.Summary("Alle organisaties ophalen"),
			fizz.Description("Geeft een lijst van alle organisaties terug."),
			fizz.Security(&openapi.SecurityRequirement{
				"apiKey":            {},
				"clientCredentials": {"organisations:read"},
			}),
			apiVersionHeader,
			notFoundResponse,
		},
		tonic.Handler(controller.ListOrganisations, 200),
	)
	writeOrg := root.Group("", "Private endpoints", "Alleen lezen endpoints")
	writeOrg.POST("/organisations",
		[]fizz.OperationOption{
			fizz.ID("createOrganisation"),
			fizz.Summary("Voeg een nieuwe organisatie toe"),
			fizz.Description("Voeg een organisatie toe op basis van URI en label."),
			fizz.Security(&openapi.SecurityRequirement{
				"apiKey":            {},
				"clientCredentials": {"organisations:write"},
			}),
			apiVersionHeader,
			notFoundResponse,
		},
		tonic.Handler(controller.CreateOrganisation, 201),
	)

	write := root.Group("", "Private endpoints", "Bewerken van API's")
	write.POST("/apis",
		[]fizz.OperationOption{
			fizz.ID("createApi"),
			fizz.Summary("Registreer een nieuwe API"),
			fizz.Description("Registreer een nieuwe API met een OpenAPI URL."),
			fizz.Security(&openapi.SecurityRequirement{
				"apiKey":            {},
				"clientCredentials": {"apis:write"},
			}),
			apiVersionHeader,
			notFoundResponse,
		},
		tonic.Handler(controller.CreateApiFromOas, 201),
	)

	write.PUT("/apis/:id",
		[]fizz.OperationOption{
			fizz.ID("updateApi"),
			fizz.Summary("Specifieke API updaten"),
			fizz.Description("Update een bestaande API."),
			fizz.Security(&openapi.SecurityRequirement{
				"apiKey":            {},
				"clientCredentials": {"apis:write"},
			}),
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

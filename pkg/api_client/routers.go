package api_client

import (
	"github.com/developer-overheid-nl/don-api-register/pkg/api_client/handler"
	"github.com/gin-contrib/cors"
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

	// Configure CORS to allow access from everywhere
	config := cors.DefaultConfig()
	config.AllowAllOrigins = true
	config.AllowMethods = []string{"GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS"}
	config.AllowHeaders = []string{"Origin", "Content-Length", "Content-Type", "Authorization", "API-Version"}
	config.ExposeHeaders = []string{"API-Version"}
	g.Use(cors.New(config))

	g.Use(APIVersionMiddleware(apiVersion))
	f := fizz.NewFromEngine(g)

	root := f.Group("/v1", "API v1", "API Register V1 routes")

	read := root.Group("", "Publieke endpoints", "Alleen lezen endpoints")
	read.GET("/apis/_search",
		[]fizz.OperationOption{
			fizz.ID("searchApis"),
			fizz.Summary("Zoek API's"),
			fizz.Description("Zoekt geregistreerde API's op basis van titel."),
			fizz.Security(&openapi.SecurityRequirement{
				"apiKey":            {},
				"clientCredentials": {"apis:read"},
			}),
			apiVersionHeader,
		},
		tonic.Handler(controller.SearchApis, 200),
	)
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

	read.GET("/apis/:id/postman",
		[]fizz.OperationOption{
			fizz.ID("getPostman"),
			fizz.Summary("Download Postman collectie"),
			fizz.Description("Geeft de gegenereerde Postman JSON terug."),
			fizz.Security(&openapi.SecurityRequirement{
				"apiKey":            {},
				"clientCredentials": {"apis:read"},
			}),
			apiVersionHeader,
			notFoundResponse,
		},
		tonic.Handler(controller.GetPostman, 200),
	)

	read.GET("/apis/:id/oas/:version",
		[]fizz.OperationOption{
			fizz.ID("getOasVersion"),
			fizz.Summary("Download OAS document"),
			fizz.Description("Geeft de OAS 3.0 of 3.1 specificatie in JSON of YAML terug."),
			fizz.Security(&openapi.SecurityRequirement{
				"apiKey":            {},
				"clientCredentials": {"apis:read"},
			}),
			apiVersionHeader,
			notFoundResponse,
		},
		tonic.Handler(controller.GetOas, 200),
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
	g.StaticFile("/v1/openapi.json", "./api/openapi.json")

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

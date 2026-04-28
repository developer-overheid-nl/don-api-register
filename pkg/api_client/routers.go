package api_client

import (
	"github.com/developer-overheid-nl/don-api-register/pkg/api_client/handler"
	"github.com/developer-overheid-nl/don-api-register/pkg/api_client/helpers/problem"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/loopfz/gadgeto/tonic"
	"github.com/wI2L/fizz"
	"github.com/wI2L/fizz/openapi"
)

var (
	apiVersionHeaderOption = fizz.Header(
		"API-Version",
		"De API-versie van de response",
		"",
	)

	apiVersionResponseHeader = &openapi.ResponseHeader{
		Name:        "API-Version",
		Description: "De API-versie van de response",
		Model:       "",
	}

	badRequestResponse = fizz.Response(
		"400",
		"Request validation failed",
		problem.APIError{},
		[]*openapi.ResponseHeader{apiVersionResponseHeader},
		nil,
	)

	notFoundResponse = fizz.Response(
		"404",
		"Resource not found",
		problem.APIError{},
		[]*openapi.ResponseHeader{apiVersionResponseHeader},
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

	apiGroup := f.Group("/v1", "APIs", "Endpoints for listing and managing APIs.")
	publicApis := apiGroup.Group("", "Public endpoints", "Public endpoints, accessible with an API key or client credentials token.")
	privateApis := apiGroup.Group("", "Private endpoints", "Private endpoints of the API register, accessible with a client credentials token.")
	publicApis.GET("/apis/_search",
		[]fizz.OperationOption{
			fizz.ID("searchApis"),
			fizz.Summary("Search APIs"),
			fizz.Description("Returns a list of APIs matching the search query."),
			fizz.WithOptionalSecurity(),
			fizz.Security(&openapi.SecurityRequirement{
				"clientCredentials": []string{},
			}),
			apiVersionHeaderOption,
			badRequestResponse,
		},
		tonic.Handler(controller.SearchApis, 200),
	)
	listApisJson := tonic.Handler(controller.ListApis, 200)
	listApisJsonLd := tonic.Handler(controller.ListApisJsonLd, 200)
	publicApis.GET("/apis",
		[]fizz.OperationOption{
			fizz.ID("listApis"),
			fizz.Summary("List APIs"),
			fizz.Description("Returns a list of APIs included in the register."),
			fizz.WithOptionalSecurity(),
			fizz.Security(&openapi.SecurityRequirement{
				"apiKey": []string{},
			}),
			fizz.Security(&openapi.SecurityRequirement{
				"clientCredentials": {"apis:read"},
			}),
			apiVersionHeaderOption,
			badRequestResponse,
		},
		func(c *gin.Context) {
			if handler.AcceptsJsonLd(c.GetHeader("Accept")) {
				listApisJsonLd(c)
				return
			}
			listApisJson(c)
		},
	)

	publicApis.GET("/apis/:id",
		[]fizz.OperationOption{
			fizz.ID("retreiveApi"),
			fizz.Summary("Get API by id"),
			fizz.Description("Returns a single API by id."),
			fizz.WithOptionalSecurity(),
			fizz.Security(&openapi.SecurityRequirement{
				"apiKey": []string{},
			}),
			fizz.Security(&openapi.SecurityRequirement{
				"clientCredentials": {"apis:read"},
			}),
			apiVersionHeaderOption,
			notFoundResponse,
		},
		tonic.Handler(controller.RetrieveApi, 200),
	)

	publicApis.GET("/apis/:id/postman",
		[]fizz.OperationOption{
			fizz.ID("getPostman"),
			fizz.Summary("Download Postman collection"),
			fizz.Description("Returns the generated Postman JSON."),
			fizz.WithOptionalSecurity(),
			fizz.Security(&openapi.SecurityRequirement{
				"apiKey": []string{},
			}),
			fizz.Security(&openapi.SecurityRequirement{
				"clientCredentials": {"apis:read"},
			}),
			apiVersionHeaderOption,
			notFoundResponse,
		},
		tonic.Handler(controller.GetPostman, 200),
	)

	publicApis.GET("/apis/:id/oas/:version",
		[]fizz.OperationOption{
			fizz.ID("getOasVersion"),
			fizz.Summary("Download OAS document"),
			fizz.Description("Returns the OAS 3.0 or 3.1 specification in JSON or YAML."),
			fizz.WithOptionalSecurity(),
			fizz.Security(&openapi.SecurityRequirement{
				"apiKey": []string{},
			}),
			fizz.Security(&openapi.SecurityRequirement{
				"clientCredentials": {"apis:read"},
			}),
			apiVersionHeaderOption,
			badRequestResponse,
			notFoundResponse,
		},
		tonic.Handler(controller.GetOas, 200),
	)

	orgGroup := f.Group("/v1", "Organisations", "Endpoints for listing and managing organisations.")
	publicOrganisations := orgGroup.Group("", "Public endpoints", "Public endpoints, accessible with an API key or client credentials token.")
	privateOrganisations := orgGroup.Group("", "Private endpoints", "Private endpoints of the API register, accessible with a client credentials token.")
	publicOrganisations.GET("/organisations",
		[]fizz.OperationOption{
			fizz.ID("listOrganisations"),
			fizz.Summary("List organisations"),
			fizz.Description("Returns a list of organisations included in the register."),
			fizz.WithOptionalSecurity(),
			fizz.Security(&openapi.SecurityRequirement{
				"apiKey": []string{},
			}),
			fizz.Security(&openapi.SecurityRequirement{
				"clientCredentials": {"organisations:read"},
			}),
			apiVersionHeaderOption,
		},
		tonic.Handler(controller.ListOrganisations, 200),
	)
	privateOrganisations.POST("/organisations",
		[]fizz.OperationOption{
			fizz.ID("createOrganisation"),
			fizz.Summary("Create organisation"),
			fizz.Description("Create a new organisation."),
			fizz.WithOptionalSecurity(),
			fizz.Security(&openapi.SecurityRequirement{
				"apiKey": []string{},
			}),
			fizz.Security(&openapi.SecurityRequirement{
				"clientCredentials": {"organisations:write"},
			}),
			apiVersionHeaderOption,
			badRequestResponse,
		},
		tonic.Handler(controller.CreateOrganisation, 201),
	)

	privateApis.GET("/lint-results",
		[]fizz.OperationOption{
			fizz.ID("listLintResults"),
			fizz.Summary("List lint results"),
			fizz.Description("Returns all stored lint results for registered APIs."),
			fizz.WithOptionalSecurity(),
			fizz.Security(&openapi.SecurityRequirement{
				"clientCredentials": {"apis:read"},
			}),
			apiVersionHeaderOption,
			badRequestResponse,
		},
		tonic.Handler(controller.ListLintResults, 200),
	)

	privateApis.POST("/apis",
		[]fizz.OperationOption{
			fizz.ID("createApi"),
			fizz.Summary("Register API"),
			fizz.Description("Registers a new API in the register from its OpenAPI document."),
			fizz.WithOptionalSecurity(),
			fizz.Security(&openapi.SecurityRequirement{
				"clientCredentials": {"apis:write"},
			}),
			apiVersionHeaderOption,
			badRequestResponse,
		},
		tonic.Handler(controller.CreateApiFromOas, 201),
	)

	privateApis.PUT("/apis/:id",
		[]fizz.OperationOption{
			fizz.ID("updateApi"),
			fizz.Summary("Update API"),
			fizz.Description("Updates an existing API by id."),
			fizz.WithOptionalSecurity(),
			fizz.Security(&openapi.SecurityRequirement{
				"clientCredentials": {"apis:write"},
			}),
			apiVersionHeaderOption,
			badRequestResponse,
			notFoundResponse,
		},
		tonic.Handler(controller.UpdateApi, 200),
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
	w.Header().Set("API-Version", w.version)
	w.ResponseWriter.WriteHeader(code)
}

func APIVersionMiddleware(version string) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer = &apiVersionWriter{c.Writer, version}
		c.Next()
	}
}

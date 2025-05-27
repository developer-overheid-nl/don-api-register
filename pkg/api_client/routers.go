package api_client

import (
	"github.com/developer-overheid-nl/don-api-register/pkg/api_client/handler"
	"github.com/gin-gonic/gin"
	"github.com/loopfz/gadgeto/tonic"
	"github.com/wI2L/fizz"
	"github.com/wI2L/fizz/openapi"
)

func NewRouter(apiVersion string, controller *handler.APIsAPIController) *fizz.Fizz {
	g := gin.Default()
	g.Use(APIVersionMiddleware(apiVersion))
	f := fizz.NewFromEngine(g)

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

	// 1) Register all endpoints with tonic.Handler
	rg := f.Group("/apis/v1", "API's", "Beheer van API-register")

	rg.GET("/apis",
		[]fizz.OperationOption{fizz.Summary("Alle API's ophalen")},
		tonic.Handler(controller.ListApis, 200),
	)

	rg.GET("/api/:id",
		[]fizz.OperationOption{fizz.Summary("Specifieke API ophalen")},
		tonic.Handler(controller.RetrieveApi, 200),
	)

	rg.POST("/apis",
		[]fizz.OperationOption{fizz.Summary("Registreer een nieuwe API met een OpenAPI URL")},
		tonic.Handler(controller.CreateApiFromOas, 201),
	)

	rg.PUT("/api/:id",
		[]fizz.OperationOption{fizz.Summary("Update een bestaande API")},
		tonic.Handler(controller.UpdateApi, 200),
	)

	// 2) Expose OpenAPI spec *after* all routes are registered
	f.GET("/openapi.json", []fizz.OperationOption{}, f.OpenAPI(info, "json"))

	return f
}

func APIVersionMiddleware(version string) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()
		if c.Writer.Status() >= 200 && c.Writer.Status() < 300 {
			c.Writer.Header().Set("API-Version", version)
		}
	}
}

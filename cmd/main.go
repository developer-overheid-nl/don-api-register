package main

import (
	"context"
	"github.com/developer-overheid-nl/don-api-register/pkg/api_client/handler"
	"github.com/developer-overheid-nl/don-api-register/pkg/api_client/helpers"
	"github.com/developer-overheid-nl/don-api-register/pkg/jobs"
	"github.com/gin-gonic/gin"
	"github.com/loopfz/gadgeto/tonic"
	"log"
	"net/http"
	"os"

	"github.com/joho/godotenv"
	_ "github.com/lib/pq"

	api "github.com/developer-overheid-nl/don-api-register/pkg/api_client"
	"github.com/developer-overheid-nl/don-api-register/pkg/api_client/database"
	"github.com/developer-overheid-nl/don-api-register/pkg/api_client/repositories"
	"github.com/developer-overheid-nl/don-api-register/pkg/api_client/services"
)

func init() {
	tonic.SetErrorHook(func(c *gin.Context, err error) (int, interface{}) {
		if _, ok := err.(tonic.BindError); ok {
			apiErr := helpers.NewBadRequest(
				"",
				"Invalid input voor update",
				helpers.InvalidParam{
					Name:   "oasUrl",
					Reason: "Moet een geldige URL zijn (bijv. https://…)",
				},
			)
			c.Header("Content-Type", "application/problem+json")
			return apiErr.Status, apiErr
		}

		// 2) Your own APIError → pass through
		if apiErr, ok := err.(helpers.APIError); ok {
			c.Header("Content-Type", "application/problem+json")
			return apiErr.Status, apiErr
		}

		internal := helpers.NewInternalServerError(err.Error())
		c.Header("Content-Type", "application/problem+json")
		return internal.Status, internal
	})
}

func main() {
	_ = godotenv.Load()

	version, err := helpers.LoadOASVersion("./api/openapi.json")
	if err != nil {
		log.Fatalf("failed to load OAS version: %v", err)
	}

	dbcon := "postgres://" +
		os.Getenv("DB_USERNAME") + ":" +
		os.Getenv("DB_PASSWORD") + "@" +
		os.Getenv("DB_HOSTNAME") + "/" +
		os.Getenv("DB_DBNAME") + "?search_path=" +
		os.Getenv("DB_SCHEMA")
	db, err := database.Connect(dbcon)
	if err != nil {
		log.Printf("[WARN] Geen databaseverbinding: %v", err)
		log.Println("[INFO] API wordt gestart zonder databasefunctionaliteit")
	}
	apiRepo := repositories.NewApiRepository(db)
	APIsAPIService := services.NewAPIsAPIService(apiRepo)
	APIsAPIController := handler.NewAPIsAPIController(APIsAPIService)
	jobs.ScheduleDailyLint(context.Background(), APIsAPIService)

	// Start server
	router := api.NewRouter(version, APIsAPIController)

	log.Println("Server is running on port 1337")
	log.Fatal(http.ListenAndServe(":1338", router))
}

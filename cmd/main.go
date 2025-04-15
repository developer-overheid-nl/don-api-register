package main

import (
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
	"log"
	"net/http"
	"os"

	api "github.com/developer-overheid-nl/don-api-register/pkg/api_client"
	"github.com/developer-overheid-nl/don-api-register/pkg/api_client/database"
	"github.com/developer-overheid-nl/don-api-register/pkg/api_client/handler"
	"github.com/developer-overheid-nl/don-api-register/pkg/api_client/repositories"
	"github.com/developer-overheid-nl/don-api-register/pkg/api_client/services"
)

func main() {
	_ = godotenv.Load()

	version, err := api.LoadOASVersion("./api/openapi.json")
	if err != nil {
		log.Fatalf("failed to load OAS version: %v", err)
	}

	dbcon := os.Getenv("DB_AUTH")
	if dbcon == "" {
		dbcon = "postgres://don:don@localhost:5432/don_v1?sslmode=disable"
	}

	db, err := database.Connect(dbcon)
	if err != nil {
		log.Printf("[WARN] Geen databaseverbinding: %v", err)
		log.Println("[INFO] API wordt gestart zonder databasefunctionaliteit")
	}
	apiRepo := repositories.NewApiRepository(db)
	APIsAPIService := services.NewAPIsAPIService(apiRepo)
	APIsAPIController := handler.NewAPIsAPIController(APIsAPIService)

	// Start server
	router := api.NewRouter(version, APIsAPIController)

	log.Println("Server is running on port 1337")
	for name, route := range APIsAPIController.Routes() {
		log.Printf("%s: http://localhost:1337%s [%s]", name, route.Pattern, route.Method)
	}
	log.Fatal(http.ListenAndServe(":1337", router))
}

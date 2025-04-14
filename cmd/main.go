package main

import (
	"database/sql"
	_ "github.com/lib/pq"
	"log"
	"net/http"

	api "github.com/developer-overheid-nl/don-api-register/pkg/api_client"
	"github.com/developer-overheid-nl/don-api-register/pkg/api_client/handler"
	"github.com/developer-overheid-nl/don-api-register/pkg/api_client/repositories"
	"github.com/developer-overheid-nl/don-api-register/pkg/api_client/services"
)

func main() {
	version, err := api.LoadOASVersion("./api/openapi.json")
	if err != nil {
		log.Fatalf("failed to load OAS version: %v", err)
	}
	// Verbind met de database
	connStr := "postgres://don:don@localhost:5432/don?sslmode=disable"
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	// Initialiseer repository, service en controller
	apiRepo := repositories.NewApiRepository(db)
	APIsAPIService := services.NewAPIsAPIService(apiRepo)
	APIsAPIController := handler.NewAPIsAPIController(APIsAPIService)

	// Start server
	router := api.NewRouter(version, APIsAPIController)

	log.Println("Server is running on port 8080")
	log.Println("http://localhost:8080/apis/v1/apis")
	log.Println("http://localhost:8080/apis/v1/apis/1")
	log.Fatal(http.ListenAndServe(":8080", router))
}

package main

import (
	"log"
	"net/http"

	api "github.com/developer-overheid-nl/don-api-register/pkg/api_client"
)

func main() {
	APIsAPIService := api.NewAPIsAPIService()
	APIsAPIController := api.NewAPIsAPIController(APIsAPIService)

	router := api.NewRouter(APIsAPIController)

	log.Fatal(http.ListenAndServe(":8080", router))
}

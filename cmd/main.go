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
	log.Println("Server is running on port 8080")
	log.Println("http://localhost:8080/apis/v1/apis")
	log.Println("http://localhost:8080/apis/v1/apis/1")
	log.Fatal(http.ListenAndServe(":8080", router))

}

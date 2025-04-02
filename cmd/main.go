package main

import (
	"fmt"

	don_api_register "github.com/developer-overheid-nl/don-api-register/pkg/api_client"
)

func main() {
	cfg := don_api_register.NewConfiguration()
	client := don_api_register.NewAPIClient(cfg)

	fmt.Println("Client ready:", client)
}

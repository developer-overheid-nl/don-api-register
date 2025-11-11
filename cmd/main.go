package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"net/url"
	"os"
	"reflect"
	"strings"

	"github.com/developer-overheid-nl/don-api-register/pkg/api_client/handler"
	problem "github.com/developer-overheid-nl/don-api-register/pkg/api_client/helpers/problem"
	util "github.com/developer-overheid-nl/don-api-register/pkg/api_client/helpers/util"
	"github.com/developer-overheid-nl/don-api-register/pkg/api_client/models"
	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"github.com/loopfz/gadgeto/tonic"

	"github.com/joho/godotenv"
	_ "github.com/lib/pq"

	api "github.com/developer-overheid-nl/don-api-register/pkg/api_client"
	"github.com/developer-overheid-nl/don-api-register/pkg/api_client/database"
	"github.com/developer-overheid-nl/don-api-register/pkg/api_client/repositories"
	"github.com/developer-overheid-nl/don-api-register/pkg/api_client/services"
)

func invalidParamsFromBinding(err error, sample any) []problem.InvalidParam {
	// Probeer direct op validator.ValidationErrors te matchen.
	var verrs validator.ValidationErrors
	if !errors.As(err, &verrs) {
		// Geen validator-errors? Geef generiek terug.
		return []problem.InvalidParam{{Name: "body", Reason: err.Error()}}
	}

	t := reflect.TypeOf(sample)
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}

	out := make([]problem.InvalidParam, 0, len(verrs))
	for _, fe := range verrs {
		name := fe.Field()
		// StructField -> json tag
		if f, ok := t.FieldByName(fe.StructField()); ok {
			if tag := f.Tag.Get("json"); tag != "" && tag != "-" {
				name = strings.Split(tag, ",")[0]
			}
		}
		out = append(out, problem.InvalidParam{
			Name:   name,
			Reason: humanReason(fe),
		})
	}
	return out
}

func humanReason(fe validator.FieldError) string {
	switch fe.Tag() {
	case "required":
		return "is verplicht"
	case "url":
		return "Moet een geldige URL zijn (bijv. https://…)"
	default:
		return fe.Error()
	}
}

func init() {
	tonic.SetErrorHook(func(c *gin.Context, err error) (int, interface{}) {
		// 1) Bind/validate errors → 400 met correcte invalidParams
		var be tonic.BindError
		if errors.As(err, &be) || isValidationErr(err) {
			invalids := invalidParamsFromBinding(err, models.UpdateApiInput{})
			apiErr := problem.NewBadRequest("body", "Invalid input voor update", invalids...)
			c.Header("Content-Type", "application/problem+json")
			return apiErr.Status, apiErr
		}

		// 2) Jouw eigen APIError → pass-through
		if apiErr, ok := err.(problem.APIError); ok {
			c.Header("Content-Type", "application/problem+json")
			return apiErr.Status, apiErr
		}

		// 3) Alles anders → 500
		internal := problem.NewInternalServerError(err.Error())
		c.Header("Content-Type", "application/problem+json")
		return internal.Status, internal
	})
}

func isValidationErr(err error) bool {
	var verrs validator.ValidationErrors
	return errors.As(err, &verrs)
}

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file ", err)
	}

	version, err := util.LoadOASVersion("./api/openapi.json")
	if err != nil {
		log.Fatalf("failed to load OAS version: %v", err)
	}
	host := os.Getenv("DB_HOSTNAME")
	user := os.Getenv("DB_USERNAME")
	pass := os.Getenv("DB_PASSWORD")
	dbname := os.Getenv("DB_DBNAME")
	schema := os.Getenv("DB_SCHEMA")

	u := &url.URL{
		Scheme: "postgres",
		Host:   host + ":5432",
		Path:   dbname,
	}
	u.User = url.UserPassword(user, pass)

	q := u.Query()
	// q.Set("sslmode", "require")
	q.Set("search_path", schema)
	u.RawQuery = q.Encode()

	dbcon := u.String()
	db, err := database.Connect(dbcon)
	if err != nil {
		log.Fatalf("Geen databaseverbinding: %v", err)
	}
	apiRepo := repositories.NewApiRepository(db)
	APIsAPIService := services.NewAPIsAPIService(apiRepo)
	APIsAPIController := handler.NewAPIsAPIController(APIsAPIService)
	if err := APIsAPIService.PublishAllApisToTypesense(context.Background()); err != nil {
		log.Fatalf("[typesense-sync] bulk publish failed: %v", err)
	}

	refreshSvc := services.NewOASRefreshService(APIsAPIService, context.Background())
	defer func() {
		if refreshSvc != nil {
			refreshSvc.Stop()
		}
	}()

	// Start server
	router := api.NewRouter(version, APIsAPIController)

	log.Println("Server is running on port 1337")
	log.Fatal(http.ListenAndServe(":1337", router))
}

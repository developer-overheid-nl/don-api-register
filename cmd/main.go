package main

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"reflect"
	"strings"

	appLogging "github.com/developer-overheid-nl/don-api-register/internal/logging"
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
	"github.com/developer-overheid-nl/don-api-register/pkg/api_client/jobs"
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
	case "required_without", "required_without_all":
		return "is verplicht wanneer geen OAS of lifecycle-datum is opgegeven"
	case "datetime":
		return "Moet een geldige datum zijn (YYYY-MM-DD)"
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
	envErr := godotenv.Load()
	logger, err := appLogging.NewJSONLogger(os.Stdout, os.Getenv("LOG_LEVEL"))
	if err != nil {
		fallbackLogger, _ := appLogging.NewJSONLogger(os.Stdout, "info")
		fallbackLogger.Error(
			"invalid logging configuration",
			"component", "application",
			"operation", "configure_logging",
			"error", err,
		)
		os.Exit(1)
	}
	slog.SetDefault(logger)
	gin.DisableConsoleColor()
	gin.DefaultWriter = io.Discard
	gin.DefaultErrorWriter = appLogging.NewSlogWriter(logger, slog.LevelError, "http_server", "recovery")

	if envErr != nil {
		slog.Error(
			"failed to load environment file",
			"component", "application",
			"operation", "load_environment",
			"error", envErr,
		)
		os.Exit(1)
		return
	}

	version, err := util.LoadOASVersion("./api/openapi.json")
	if err != nil {
		slog.Error(
			"failed to load OAS version",
			"component", "application",
			"operation", "load_oas_version",
			"error", err,
		)
		os.Exit(1)
		return
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
		slog.Error(
			"database connection failed",
			"component", "database",
			"operation", "connect",
			"error", err,
		)
		os.Exit(1)
		return
	}
	database.ConfigureLogging(db, slog.Default())
	apiRepo := repositories.NewApiRepository(db)
	APIsAPIService := services.NewAPIsAPIService(apiRepo)
	APIsAPIController := handler.NewAPIsAPIController(APIsAPIService)
	if err := APIsAPIService.PublishAllApisToTypesense(context.Background()); err != nil {
		slog.Error(
			"initial Typesense synchronization failed",
			"component", "typesense",
			"operation", "bulk_index",
			"error", err,
		)
		os.Exit(1)
		return
	}

	refreshJob := jobs.NewOASRefreshJob(APIsAPIService, context.Background())
	harvesterService := services.NewHarvesterService(APIsAPIService)
	jobs.SchedulePDOKHarvest(context.Background(), harvesterService)
	defer func() {
		if refreshJob != nil {
			refreshJob.Stop()
		}
	}()

	// Start server
	router := api.NewRouter(version, APIsAPIController)

	slog.Info(
		"server started",
		"component", "http_server",
		"operation", "listen",
		"address", ":1337",
	)
	if err := http.ListenAndServe(":1337", router); err != nil {
		slog.Error(
			"HTTP server stopped",
			"component", "http_server",
			"operation", "listen",
			"error", err,
		)
		os.Exit(1)
	}
}

package services

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	problem "github.com/developer-overheid-nl/don-api-register/pkg/api_client/helpers/problem"
	"github.com/developer-overheid-nl/don-api-register/pkg/api_client/models"
	"github.com/developer-overheid-nl/don-api-register/pkg/api_client/repositories"
	commonlogging "github.com/developer-overheid-nl/don-register-common/logging"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/time/rate"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestHarvestErrorUsesProblemDetailAndPreservesCause(t *testing.T) {
	cause := problem.NewBadRequest("https://example.org/openapi.json", "operationId ontbreekt")
	harvestErr := newHarvestError(
		models.HarvestResult{CandidateCount: 2, FailedCount: 1},
		"https://example.org/openapi.json",
		cause,
	)

	assert.Equal(t, "operationId ontbreekt", harvestErr.Detail)
	assert.Equal(
		t,
		"1 failures; first: https://example.org/openapi.json: operationId ontbreekt",
		harvestErr.Error(),
	)
	var problemCause problem.APIError
	assert.True(t, errors.As(harvestErr, &problemCause))
	assert.Equal(t, cause.Title, problemCause.Title)
}

func TestHarvestErrorFallsBackToCauseText(t *testing.T) {
	cause := errors.New("upstream timeout")
	harvestErr := newHarvestError(models.HarvestResult{FailedCount: 1}, "https://example.org/openapi.json", cause)

	assert.Equal(t, "upstream timeout", harvestErr.Detail)
	assert.ErrorIs(t, harvestErr, cause)
}

func newHarvesterTestService(t *testing.T) (*HarvesterService, repositories.ApiRepository) {
	t.Helper()

	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&models.Api{},
		&models.Organisation{},
		&models.Server{},
		&models.ApiArtifact{},
		&models.LintResult{},
		&models.LintMessage{},
		&models.LintMessageInfo{},
		&models.ApiFeedEvent{},
	))
	repo := repositories.NewApiRepository(db)
	service := NewAPIsAPIService(repo)
	service.limiter = rate.NewLimiter(rate.Inf, 1)
	harvester := NewHarvesterService(service)
	harvester.httpClient = &http.Client{Timeout: 2 * time.Second}
	return harvester, repo
}

func TestDeriveOASURLWith(t *testing.T) {
	tests := []struct {
		name    string
		href    string
		suffix  string
		oasPath string
		want    string
	}{
		{
			name:    "trims configured ui suffix",
			href:    "https://api.example.org/service/ui/",
			suffix:  "ui/",
			oasPath: "openapi.json",
			want:    "https://api.example.org/service/openapi.json",
		},
		{
			name:    "handles suffix without trailing slash",
			href:    "https://api.example.org/service/ui",
			suffix:  "ui",
			oasPath: "openapi.yaml",
			want:    "https://api.example.org/service/openapi.yaml",
		},
		{
			name:    "appends oas path to directory href",
			href:    "https://api.example.org/service/",
			suffix:  "",
			oasPath: "",
			want:    "https://api.example.org/service/openapi.json",
		},
		{
			name:    "appends slash for file-like href",
			href:    "https://api.example.org/service",
			suffix:  "docs",
			oasPath: "spec.json",
			want:    "https://api.example.org/service/spec.json",
		},
	}

	for _, tc := range tests {
		current := tc
		t.Run(current.name, func(t *testing.T) {
			assert.Equal(t, current.want, deriveOASURLWith(current.href, current.suffix, current.oasPath))
		})
	}
}

func TestExtractIndexHrefs(t *testing.T) {
	index := []byte(`{
	  "apis": [
	    {"links": [{"href": "https://api.example.org/a/ui/"}, {"href": " "}]},
	    {"links": {"href": "https://api.example.org/b/ui/"}},
	    {"links": []}
	  ]
	}`)

	got, err := extractIndexHrefs(index)

	require.NoError(t, err)
	assert.Equal(t, []string{"https://api.example.org/a/ui/", "https://api.example.org/b/ui/"}, got)
}

func TestExtractIndexHrefsRejectsInvalidJSON(t *testing.T) {
	_, err := extractIndexHrefs([]byte(`{`))

	require.Error(t, err)
	assert.Contains(t, err.Error(), "parse index.json")
}

func TestHarvesterRunOnceCreatesApisFromIndex(t *testing.T) {
	t.Setenv("TOOLS_API_ENDPOINT", "")
	t.Setenv("ENABLE_TYPESENSE", "false")
	harvester, repo := newHarvesterTestService(t)
	ctx := context.Background()

	org := models.Organisation{Uri: "https://example.org/org", Label: "Example Org"}
	require.NoError(t, repo.SaveOrganisatie(&org))

	oasServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/service/openapi.json", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
		  "openapi": "3.1.0",
		  "info": {
		    "title": "Harvested API",
		    "version": "1.0.0",
		    "contact": {
		      "name": "Spec Team",
		      "email": "spec@example.org",
		      "url": "https://example.org/contact"
		    }
		  },
		  "paths": {}
		}`))
	}))
	t.Cleanup(oasServer.Close)

	indexServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"apis":[{"links":{"href":"` + oasServer.URL + `/service/ui/"}}]}`))
	}))
	t.Cleanup(indexServer.Close)

	result, err := harvester.RunOnce(ctx, models.HarvestSource{
		Name:            "example",
		IndexURL:        indexServer.URL,
		OrganisationUri: org.Uri,
		Contact:         models.Contact{Name: "Fallback", Email: "fallback@example.org", URL: "https://example.org/fallback"},
	})

	require.NoError(t, err)
	assert.Equal(t, models.HarvestResult{CandidateCount: 1, CreatedCount: 1}, result)
	apis, err := repo.AllApis(ctx)
	require.NoError(t, err)
	require.Len(t, apis, 1)
	assert.Equal(t, "Harvested API", apis[0].Title)
	assert.Equal(t, oasServer.URL+"/service/openapi.json", apis[0].OasUri)
	assert.Equal(t, org.Uri, *apis[0].OrganisationID)
	assert.Equal(t, "3.1.0", apis[0].OAS.Version)
	assert.Equal(t, models.OASStatusValid, apis[0].OAS.Status)
}

func TestHarvesterRunOnceSkipsExistingOASAndLogsSummary(t *testing.T) {
	t.Setenv("TOOLS_API_ENDPOINT", "")
	t.Setenv("ENABLE_TYPESENSE", "false")
	harvester, repo := newHarvesterTestService(t)
	ctx := context.Background()

	var oasCalls atomic.Int32
	oasServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		oasCalls.Add(1)
		http.Error(w, "existing OAS must not be fetched", http.StatusInternalServerError)
	}))
	t.Cleanup(oasServer.Close)
	oasURL := oasServer.URL + "/service/openapi.json"
	require.NoError(t, repo.Save(&models.Api{Id: "existing", OasUri: oasURL, Title: "Existing"}))

	indexServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"apis":[{"links":{"href":"` + oasServer.URL + `/service/ui/"}}]}`))
	}))
	t.Cleanup(indexServer.Close)

	var logBuffer bytes.Buffer
	logger, err := commonlogging.NewJSONLogger(&logBuffer, "api-register", "debug")
	require.NoError(t, err)
	previousLogger := slog.Default()
	slog.SetDefault(logger)
	t.Cleanup(func() { slog.SetDefault(previousLogger) })

	result, err := harvester.RunOnce(ctx, models.HarvestSource{
		Name:     "example",
		IndexURL: indexServer.URL,
	})

	require.NoError(t, err)
	assert.Equal(t, models.HarvestResult{CandidateCount: 1, SkippedCount: 1}, result)
	assert.Zero(t, oasCalls.Load())

	var completed map[string]any
	for _, line := range strings.Split(strings.TrimSpace(logBuffer.String()), "\n") {
		var record map[string]any
		require.NoError(t, json.Unmarshal([]byte(line), &record))
		assert.NotEqual(t, "ERROR", record["level"])
		assert.NotEqual(t, "WARN", record["level"])
		if record["msg"] == "harvest completed" {
			completed = record
		}
	}
	require.NotNil(t, completed)
	assert.Equal(t, "api-register", completed["app"])
	assert.Equal(t, "harvest", completed["component"])
	assert.Equal(t, "run", completed["operation"])
	assert.Equal(t, float64(1), completed["candidate_count"])
	assert.Equal(t, float64(0), completed["created_count"])
	assert.Equal(t, float64(1), completed["skipped_count"])
	assert.Equal(t, float64(0), completed["failed_count"])
}

func TestHarvesterRunOnceValidatesConfiguration(t *testing.T) {
	service := &HarvesterService{}

	_, err := service.RunOnce(context.Background(), models.HarvestSource{IndexURL: "https://example.org/index.json"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "api service is not configured")

	service.apiService = NewAPIsAPIService(nil)
	_, err = service.RunOnce(context.Background(), models.HarvestSource{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "source indexUrl is empty")
}

package problem

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProblemConstructors(t *testing.T) {
	t.Run("new bad request maps invalid params", func(t *testing.T) {
		got := NewBadRequest("body", "invalid body", InvalidParam{Name: "oasUrl", Reason: "is verplicht"})

		assert.Equal(t, http.StatusBadRequest, got.Status)
		assert.Equal(t, "Request validation failed", got.Title)
		require.Len(t, got.Errors, 1)
		assert.Equal(t, "oasUrl", got.Errors[0].Code)
		assert.Equal(t, "is verplicht", got.Errors[0].Detail)
	})

	t.Run("new not found uses path location", func(t *testing.T) {
		got := NewNotFound("api-1", "Api not found")

		assert.Equal(t, http.StatusNotFound, got.Status)
		assert.Equal(t, "Resource Not Found", got.Title)
		require.Len(t, got.Errors, 1)
		assert.Equal(t, "not_found", got.Errors[0].Code)
		assert.Equal(t, "Api not found", got.Errors[0].Detail)
	})

	t.Run("new internal server error", func(t *testing.T) {
		got := NewInternalServerError("database offline")

		assert.Equal(t, http.StatusInternalServerError, got.Status)
		assert.Equal(t, "Internal Server Error", got.Title)
		require.Len(t, got.Errors, 1)
		assert.Equal(t, "internal_error", got.Errors[0].Code)
		assert.Equal(t, "database offline", got.Errors[0].Detail)
	})

	t.Run("new forbidden", func(t *testing.T) {
		got := NewForbidden("https://example.org/openapi.json", "not owner")

		assert.Equal(t, http.StatusForbidden, got.Status)
		assert.Equal(t, "Forbidden", got.Title)
		require.Len(t, got.Errors, 1)
		assert.Equal(t, "forbidden", got.Errors[0].Code)
		assert.Equal(t, "not owner", got.Errors[0].Detail)
	})

	t.Run("new generic problem", func(t *testing.T) {
		got := New(http.StatusConflict, "Conflict")

		assert.Equal(t, http.StatusConflict, got.Status)
		assert.Equal(t, "Conflict", got.Title)
	})
}

func TestProblemAlwaysContainsStatusTitleAndDetail(t *testing.T) {
	tests := []struct {
		name       string
		problem    APIError
		wantStatus int
		wantDetail string
	}{
		{"bad request with detail", NewBadRequest("body", "Invalid input"), 400, "Invalid input"},
		{"bad request from params", NewBadRequest("body", "", InvalidParam{Name: "uri", Reason: "is verplicht"}), 400, "is verplicht"},
		{"bad request joins param reasons", NewBadRequest("body", "", InvalidParam{Name: "uri", Reason: "is verplicht"}, InvalidParam{Name: "label", Reason: "is te lang"}), 400, "is verplicht; is te lang"},
		{"not found", NewNotFound("/apis/1", "API not found"), 404, "API not found"},
		{"internal", NewInternalServerError("boom"), 500, "boom"},
		{"forbidden", NewForbidden("", "no access"), 403, "no access"},
		{"new with error detail", New(409, "Organisation already exists", ErrorDetail{Detail: "organisation already exists"}), 409, "organisation already exists"},
		{"new falls back to title", New(400, "Request validation failed"), 400, "Request validation failed"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			raw, err := json.Marshal(tt.problem)
			require.NoError(t, err)
			var body map[string]any
			require.NoError(t, json.Unmarshal(raw, &body))
			assert.Equal(t, float64(tt.wantStatus), body["status"])
			assert.NotEmpty(t, body["title"])
			assert.Equal(t, tt.wantDetail, body["detail"])
		})
	}
}

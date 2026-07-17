package problem

import (
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

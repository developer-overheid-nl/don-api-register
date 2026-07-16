package util

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadOASVersion(t *testing.T) {
	t.Run("returns version", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "openapi.json")
		require.NoError(t, os.WriteFile(path, []byte(`{"info":{"version":"2.3.4"}}`), 0o600))

		got, err := LoadOASVersion(path)

		require.NoError(t, err)
		assert.Equal(t, "2.3.4", got)
	})

	t.Run("missing file", func(t *testing.T) {
		_, err := LoadOASVersion(filepath.Join(t.TempDir(), "missing.json"))

		require.Error(t, err)
		assert.Contains(t, err.Error(), "could not open OAS file")
	})

	t.Run("invalid json", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "openapi.json")
		require.NoError(t, os.WriteFile(path, []byte(`{`), 0o600))

		_, err := LoadOASVersion(path)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "could not parse OAS")
	})

	t.Run("missing version", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "openapi.json")
		require.NoError(t, os.WriteFile(path, []byte(`{"info":{}}`), 0o600))

		_, err := LoadOASVersion(path)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "version missing from OAS")
	})
}

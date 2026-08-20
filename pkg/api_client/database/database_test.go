package database

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type loggingTestRecord struct {
	ID uint `gorm:"primaryKey"`
}

func TestConfigureLoggingSuppressesExpectedMissesAndStructuresDatabaseErrors(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"))
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&loggingTestRecord{}))

	var output bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&output, nil))
	ConfigureLogging(db, logger)

	err = db.First(&loggingTestRecord{}, 404).Error
	require.ErrorIs(t, err, gorm.ErrRecordNotFound)
	assert.Empty(t, output.String(), "an expected lookup miss must not become a Loki error")

	err = db.Exec("SELECT * FROM logging_table_that_does_not_exist").Error
	require.Error(t, err)

	var event map[string]any
	require.NoError(t, json.Unmarshal(output.Bytes(), &event))
	assert.Equal(t, "ERROR", event["level"])
	assert.Equal(t, "database", event["component"])
	assert.Equal(t, "query", event["operation"])
	assert.Equal(t, "SQL executed", event["msg"])
}

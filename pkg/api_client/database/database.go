package database

import (
	"log/slog"
	"time"

	"github.com/developer-overheid-nl/don-api-register/pkg/api_client/models"
	commondatabase "github.com/developer-overheid-nl/don-register-common/database"
	_ "github.com/lib/pq"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

func Connect(connStr string) (*gorm.DB, error) {
	return commondatabase.ConnectPostgres(connStr,
		&models.Api{},
		&models.LintResult{},
		&models.LintMessage{},
		&models.LintMessageInfo{},
		&models.ApiArtifact{},
		&models.ApiFeedEvent{},
	)
}

// ConfigureDefaultLogging structures GORM events emitted while opening and
// migrating the database.
func ConfigureDefaultLogging(logger *slog.Logger) {
	gormlogger.Default = newStructuredLogger(logger)
}

// ConfigureLogging sends runtime GORM events through the application's
// structured logger. Expected lookup misses are intentionally not errors.
func ConfigureLogging(db *gorm.DB, logger *slog.Logger) {
	db.Logger = newStructuredLogger(logger)
}

func newStructuredLogger(logger *slog.Logger) gormlogger.Interface {
	return gormlogger.NewSlogLogger(
		logger.With("component", "database", "operation", "query"),
		gormlogger.Config{
			SlowThreshold:             200 * time.Millisecond,
			LogLevel:                  gormlogger.Warn,
			IgnoreRecordNotFoundError: true,
			ParameterizedQueries:      true,
		},
	)
}

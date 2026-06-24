package database

import (
	"github.com/developer-overheid-nl/don-api-register/pkg/api_client/models"
	commondatabase "github.com/developer-overheid-nl/don-register-common/database"
	_ "github.com/lib/pq"
	"gorm.io/gorm"
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

package database

import (
	"fmt"

	"github.com/developer-overheid-nl/don-api-register/pkg/api_client/models"
	_ "github.com/lib/pq"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

func Connect(connStr string) (*gorm.DB, error) {
	db, err := gorm.Open(postgres.Open(connStr), &gorm.Config{
		NamingStrategy: schema.NamingStrategy{
			TablePrefix: "v1_", // add your prefix here
		}})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	if err := db.AutoMigrate(&models.Api{}, &models.LintResult{}, &models.LintMessage{}, &models.LintMessageInfo{}); err != nil {
		return nil, fmt.Errorf("migration failed: %w", err)
	}

	return db, nil
}

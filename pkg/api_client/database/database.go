package database

import (
	"github.com/developer-overheid-nl/don-api-register/pkg/api_client/models"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"log"

	_ "github.com/lib/pq"
)

func Connect(connStr string) (*gorm.DB, error) {
	db, err := gorm.Open(postgres.Open(connStr), &gorm.Config{})
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	if err := db.AutoMigrate(&models.Api{}); err != nil {
		log.Fatalf("Migration failed: %v", err)
	}

	return db, nil
}

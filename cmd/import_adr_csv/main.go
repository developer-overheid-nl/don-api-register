package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/url"
	"os"
	"strings"

	"github.com/joho/godotenv"
	"gorm.io/gorm"

	"github.com/developer-overheid-nl/don-api-register/pkg/adrimport"
	"github.com/developer-overheid-nl/don-api-register/pkg/api_client/database"
)

func main() {
	csvPath := flag.String("csv", "ADR_VALIDATOR_HISTORY.csv", "path to ADR validator history CSV")
	dryRun := flag.Bool("dry-run", false, "parse and match without writing to the database")
	flag.Parse()

	if err := godotenv.Load(); err != nil {
		log.Printf(".env not loaded: %v", err)
	}

	db, err := connectDB()
	if err != nil {
		log.Fatalf("database connection failed: %v", err)
	}

	result, err := adrimport.ImportCSV(context.Background(), db, adrimport.Options{
		CSVPath: *csvPath,
		DryRun:  *dryRun,
	})
	if err != nil {
		log.Fatalf("import failed: %v", err)
	}
	if result.ParseErrors > 0 {
		os.Exit(1)
	}
}

func connectDB() (*gorm.DB, error) {
	host := os.Getenv("DB_HOSTNAME")
	user := os.Getenv("DB_USERNAME")
	pass := os.Getenv("DB_PASSWORD")
	dbname := os.Getenv("DB_DBNAME")
	schema := os.Getenv("DB_SCHEMA")
	if host == "" || user == "" || dbname == "" {
		return nil, fmt.Errorf("missing DB env vars; need DB_HOSTNAME, DB_USERNAME, DB_DBNAME")
	}

	u := &url.URL{
		Scheme: "postgres",
		Host:   host + ":5432",
		Path:   dbname,
	}
	u.User = url.UserPassword(user, pass)

	q := u.Query()
	if strings.TrimSpace(schema) != "" {
		q.Set("search_path", schema)
	}
	u.RawQuery = q.Encode()

	return database.Connect(u.String())
}
